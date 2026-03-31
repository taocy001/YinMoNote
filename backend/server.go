package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/acme/autocert"
)

// Server is the HTTP layer that wraps NoteLibrary with routing, authentication,
// and input validation. It deliberately contains no file I/O logic — that belongs
// in NoteLibrary. This separation makes each layer independently testable.
type Server struct {
	Library *NoteLibrary
	// CACert holds the PEM-encoded local CA certificate when TLS_SELF=1 is set.
	// It is served at GET /ca.crt so users can download and trust it once.
	// Nil in all other TLS modes.
	CACert []byte
	// UseTLS is set to true when the server runs with any TLS mode (ACME, manual, self-signed).
	// Used by secHeaders to conditionally add Strict-Transport-Security.
	UseTLS bool
}

// NewServer creates a new API server instance.
func NewServer(lib *NoteLibrary) *Server { return &Server{Library: lib} }

// SetupRouter registers all routes and middleware. Static assets are served
// from the embedded filesystem (go:embed dist in static.go); the SPA catch-all
// (NoRoute) serves index.html for any non-API path, enabling client-side routing.
//
// Note: /uploads/:filename is served via its own authenticated group (not the
// unauthenticated r.Static shorthand) so that Basic Auth protects asset files
// in addition to the JSON API. The /uploads/ URL is preserved to keep existing
// markdown image references (./uploads/filename) working without migration.
func (s *Server) SetupRouter() *gin.Engine {
	// Use gin.New() instead of gin.Default() to avoid the built-in Logger middleware,
	// which would print every request path (including note filenames) to container logs.
	// gin.Recovery() is added explicitly so panics are caught and converted to 500s.
	r := gin.New()
	r.Use(gin.Recovery())
	// Trust only loopback addresses as reverse-proxy sources.
	// Without this, Gin v1.7+ trusts all X-Forwarded-For headers by default, which
	// allows an attacker to spoof arbitrary IPs and bypass the per-IP auth rate limiter.
	// Loopback trust covers nginx/Caddy running on the same host; external proxy IPs
	// would use RemoteAddr directly, which is the TCP-layer address and cannot be spoofed.
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(s.secHeaders(), s.limitSize())

	// Serve embedded frontend assets (baked into the binary via go:embed).
	// assetsFS is sub-rooted at dist/assets so /assets/foo.js → dist/assets/foo.js.
	// If dist/assets is absent (e.g. local dev without a frontend build), the route is
	// simply skipped — unit tests still pass because they only exercise API endpoints.
	if assetsFS, err := fs.Sub(staticFiles, "dist/assets"); err == nil {
		r.StaticFS("/assets", http.FS(assetsFS))
	}
	for _, name := range []string{"favicon.ico", "vite.svg", "sw.js", "config.json"} {
		name := name // capture loop variable for the closure
		r.GET("/"+name, func(c *gin.Context) {
			c.FileFromFS("dist/"+name, http.FS(staticFiles))
		})
	}

	// GET /ca.crt — serves the local CA certificate for one-time device trust.
	// Only returns data when TLS_SELF=1 is active (s.CACert is non-nil).
	// Unauthenticated: the user needs to download this before trusting TLS.
	r.GET("/ca.crt", s.handleCACert)

	// GET /api/mcp/ca-fingerprint — unauthenticated; returns the SHA-256
	// fingerprint of the local CA cert so the Settings UI can display it for
	// out-of-band verification before the user installs the cert.
	// Returns 404 when TLS_SELF=1 is not active.
	r.GET("/api/mcp/ca-fingerprint", s.handleMCPCAFingerprint)

	// MCP transport endpoints — authenticated with the separate MCP token.
	// GET  /mcp/sse      opens a persistent SSE stream (one per AI session).
	// POST /mcp/messages sends a JSON-RPC request on an existing SSE session.
	mcp := r.Group("/mcp", s.mcpAuth())
	{
		mcp.GET("/sse", s.handleMCPSSE)
		mcp.POST("/messages", s.handleMCPMessage)
	}

	// GET /api/auth/status — unauthenticated; tells new devices whether a password
	// has been configured so they show "enter password" instead of "initialize library".
	r.GET("/api/auth/status", s.handleAuthStatus)

	// POST /api/auth/setup is intentionally outside the authenticated group:
	// it is the endpoint that establishes authentication in the first place.
	r.POST("/api/auth/setup", s.handleAuthSetup)

	// POST /api/test/reset-auth clears SRPVerifier and SRPSalt unconditionally.
	// Available ONLY when SYNC_COMMIT=1 (the E2E test environment variable).
	// This lets the E2E test suite restore the server to open/keyless state after
	// password-mode tests, without needing the current session token.
	if os.Getenv("SYNC_COMMIT") == "1" {
		r.POST("/api/test/reset-auth", s.handleTestResetAuth)
	}

	// POST /api/auth/srp/init — unauthenticated; starts SRP-6a handshake.
	// Client sends A; server responds with B and the SRP salt.
	r.POST("/api/auth/srp/init", s.handleSRPInit)

	// POST /api/auth/srp/verify — unauthenticated; completes SRP-6a handshake.
	// Client sends M1; server validates and returns Bearer token + M2.
	r.POST("/api/auth/srp/verify", s.handleSRPVerify)

	api := r.Group("/api", s.apiAuth())
	{
		api.GET("/config", s.handleGetConfig)
		api.PUT("/config", s.handleUpdateConfig)
		// MCP token management — requires the main session auth (not the MCP token).
		// POST creates/replaces the MCP token; DELETE revokes it.
		api.POST("/mcp/token", s.handleMCPSetToken)
		api.DELETE("/mcp/token", s.handleMCPRevokeToken)
		// WebDAV token management — persisted in config.json, survives restarts.
		// POST creates/replaces the WebDAV token; DELETE revokes it.
		api.POST("/webdav/token", s.handleWebDAVSetToken)
		api.DELETE("/webdav/token", s.handleWebDAVRevokeToken)
		api.GET("/notes", s.handleListNotes)
		api.GET("/notes/bulk", s.handleBulkGetNotes)
		api.GET("/notes/:filename", s.handleGetNote)
		api.PUT("/notes/:filename", s.handleSaveNote)
		api.DELETE("/notes/:filename", s.handleDeleteNote)
		api.GET("/structure", s.handleGetStructure)
		api.PUT("/structure", s.handleSaveStructure)
		api.POST("/upload", s.handleUpload)
		api.GET("/assets", s.handleListAssets)
		api.PUT("/uploads/:filename", s.handleOverwriteAsset)
		api.DELETE("/uploads/:filename", s.handleDeleteAsset)
		api.GET("/notes/:filename/history", s.handleGetHistory)
		api.GET("/notes/:filename/version/:hash", s.handleGetVersion)
		api.POST("/notes/:filename/rollback", s.handleRollback)
	}
	// Serve uploaded assets at /uploads/:filename with the same auth guard as the API group.
	uploads := r.Group("/uploads", s.apiAuth())
	uploads.GET("/:filename", s.handleGetUpload)

	r.NoRoute(s.handleSPA)
	return r
}

// buildHandler combines the Gin router and (optionally) the WebDAV handler
// into a single http.Handler.
//
// WebDAV is enabled by default. Set WEBDAV_DISABLED=1 to disable it entirely
// (e.g. when only the web app is needed and the /dav/ path should not exist).
//
// When enabled, WebDAV paths (/dav and /dav/*) are dispatched before Gin so
// that WebDAV methods (PROPFIND, MKCOL, COPY, MOVE, LOCK, UNLOCK) are never
// rejected by Gin's method-not-allowed check.
func (s *Server) buildHandler() http.Handler {
	ginH := s.SetupRouter()
	if os.Getenv("WEBDAV_DISABLED") == "1" {
		return ginH
	}
	davH := s.newDavHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dav" || strings.HasPrefix(r.URL.Path, "/dav/") {
			davH.ServeHTTP(w, r)
			return
		}
		ginH.ServeHTTP(w, r)
	})
}

// handleCACert serves the local CA certificate PEM for download.
// Only active when TLS_SELF=1 (s.CACert is non-nil); returns 404 otherwise.
// The Content-Type triggers "Install Profile" on iOS and automatic cert
// import on Android when the file is opened from the browser.
func (s *Server) handleCACert(c *gin.Context) {
	if s.CACert == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "application/x-x509-ca-cert", s.CACert)
}

// Run starts the HTTP server.
//
// TLS modes (checked in priority order):
//
//	ACME_DOMAIN=<domain>           Automatic TLS via Let's Encrypt.  The server
//	                               listens on :443; port 80 is also opened to serve
//	                               the ACME HTTP-01 challenge and redirect HTTP→HTTPS.
//	                               PORT is ignored in this mode.
//	TLS_CERT=<file> TLS_KEY=<file> Manual TLS with a user-supplied certificate/key.
//	                               Listens on PORT (default :8080).
//	TLS_SELF=1                     Self-signed TLS with a locally-generated CA.
//	                               Cert covers all current host IPs.  Download the CA
//	                               cert from GET /ca.crt (accept the browser warning
//	                               once) and install it on each device.
//	                               Listens on PORT (default :8080).
//	(none)                         Plain HTTP on PORT.
func (s *Server) Run(port string) {
	acmeDomain := os.Getenv("ACME_DOMAIN")
	certFile := os.Getenv("TLS_CERT")
	keyFile := os.Getenv("TLS_KEY")
	tlsSelf := os.Getenv("TLS_SELF") == "1"

	// Self-TLS setup must happen before buildHandler so s.CACert is set when
	// the /ca.crt route handler is first called.
	var selfTLSCfg *tls.Config
	if tlsSelf && acmeDomain == "" && certFile == "" {
		cacheDir := filepath.Join(filepath.Dir(s.Library.ConfigPath), "selfca")
		cfg, caPEM, err := selfTLSSetup(cacheDir)
		if err != nil {
			panic(fmt.Sprintf("TLS_SELF setup failed: %v", err))
		}
		selfTLSCfg = cfg
		s.CACert = caPEM
	}

	// Set UseTLS flag for HSTS header emission in secHeaders.
	s.UseTLS = acmeDomain != "" || certFile != "" || selfTLSCfg != nil

	h := s.buildHandler()

	switch {
	case acmeDomain != "":
		// Automatic TLS via Let's Encrypt ACME HTTP-01 challenge.
		// Certificates are cached in <config-dir>/autocert/ across restarts.
		cacheDir := filepath.Join(filepath.Dir(s.Library.ConfigPath), "autocert")
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(acmeDomain),
			Cache:      autocert.DirCache(cacheDir),
		}
		// Port :80 handles ACME challenges and redirects plain HTTP to HTTPS.
		go func() {
			_ = http.ListenAndServe(":80", m.HTTPHandler(nil))
		}()
		fmt.Printf("YinMo running on :443 (ACME TLS: %s)\n", acmeDomain)
		srv := &http.Server{Addr: ":443", TLSConfig: m.TLSConfig(), Handler: h}
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			panic(err)
		}

	case certFile != "" && keyFile != "":
		// Manual TLS: user supplies the certificate and private-key PEM files.
		fmt.Printf("YinMo running on %s (TLS)\n", port)
		if err := http.ListenAndServeTLS(port, certFile, keyFile, h); err != nil {
			panic(err)
		}

	case selfTLSCfg != nil:
		// Self-signed TLS: locally-generated CA, cert covers all host IPs.
		cacheDir := filepath.Join(filepath.Dir(s.Library.ConfigPath), "selfca")
		caCertPath := filepath.Join(cacheDir, "ca.crt")
		fmt.Printf("YinMo running on %s (self-signed TLS)\n", port)
		fmt.Printf("  → This machine: install the CA cert file directly:\n")
		fmt.Printf("    %s\n", caCertPath)
		fmt.Printf("    macOS: open %s\n", caCertPath)
		fmt.Printf("  → Remote devices: visit the URL below, click through the security\n")
		fmt.Printf("    warning once (Advanced → Proceed), then install the downloaded file:\n")
		fmt.Printf("    https://<this-server-ip>%s/ca.crt\n", port)
		srv := &http.Server{Addr: port, TLSConfig: selfTLSCfg, Handler: h}
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			panic(err)
		}

	default:
		// Plain HTTP — warn if binding to all network interfaces.
		if !strings.HasPrefix(port, "127.0.0.1:") && !strings.HasPrefix(port, "[::1]:") && !strings.HasPrefix(port, "localhost:") {
			fmt.Fprintf(os.Stderr, "WARNING: YinMo is serving plain HTTP on %s. "+
				"Traffic is unencrypted and credentials are exposed to the network. "+
				"Set TLS_SELF=1, TLS_CERT+TLS_KEY, or ACME_DOMAIN for production use.\n", port)
		}
		fmt.Printf("YinMo running on %s\n", port)
		if err := http.ListenAndServe(port, h); err != nil {
			panic(err)
		}
	}
}

func (s *Server) handleGetConfig(c *gin.Context) {
	s.Library.mu.Lock()
	cfg := s.Library.Config
	mcpTokenSet := cfg.MCPTokenHash != ""
	webdavTokenSet := cfg.WebDAVTokenHash != ""
	s.Library.mu.Unlock()
	// Never expose authentication secrets to clients.
	cfg.SRPSalt = ""
	cfg.SRPVerifier = ""
	cfg.MCPTokenHash = ""
	cfg.WebDAVTokenHash = ""
	// Augment with booleans so the UI can tell whether tokens are configured
	// without receiving the hashes themselves.
	type configResp struct {
		AppConfig
		MCPTokenSet    bool `json:"mcpTokenSet"`
		WebDAVTokenSet bool `json:"webdavTokenSet"`
	}
	c.JSON(200, configResp{AppConfig: cfg, MCPTokenSet: mcpTokenSet, WebDAVTokenSet: webdavTokenSet})
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	// Seed the new config from the existing one so that fields absent from the JSON
	// payload retain their current values. Without this, json.Decoder zero-fills
	// unspecified numeric fields, which clampConfig then resets to defaults — silently
	// overwriting user-configured quotas on any partial update (e.g. language change).
	s.Library.mu.Lock()
	cfg := s.Library.Config
	s.Library.mu.Unlock()

	if err := c.BindJSON(&cfg); err != nil {
		return
	}
	// Clamp quotas so a client cannot bypass the disk-usage limits established at startup.
	clampConfig(&cfg)
	// Sanitize MCPPolicy: reject unrecognized access level values so a hand-edited
	// or API-supplied policy cannot introduce undefined behaviour.
	sanitizeMCPPolicy(&cfg.MCPPolicy)

	// Single lock covers the read→merge→write sequence to prevent races between
	// concurrent config updates. Write to disk first; only update in-memory on
	// success so a disk failure never leaves memory and disk inconsistent.
	s.Library.mu.Lock()
	defer s.Library.mu.Unlock()
	// Preserve fields that clients must not override.
	cfg.Port = s.Library.Config.Port
	cfg.SRPSalt = s.Library.Config.SRPSalt
	cfg.SRPVerifier = s.Library.Config.SRPVerifier
	cfg.MCPTokenHash = s.Library.Config.MCPTokenHash
	cfg.WebDAVTokenHash = s.Library.Config.WebDAVTokenHash
	// Preserve salt if client didn't send one (partial update must not clear it)
	if cfg.Pbkdf2Salt == "" { cfg.Pbkdf2Salt = s.Library.Config.Pbkdf2Salt }
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		c.JSON(500, gin.H{"error": "marshal_failed"})
		return
	}
	if err := atomicWriteFile(s.Library.ConfigPath, data, 0600); err != nil {
		c.JSON(500, gin.H{"error": "write_failed"})
		return
	}
	s.Library.Config = cfg
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleListNotes(c *gin.Context) {
	n, err := s.Library.ListNotes()
	if err != nil {
		c.JSON(500, gin.H{"error": "list_failed"})
		return
	}
	c.JSON(200, gin.H{"notes": n})
}

func (s *Server) handleGetNote(c *gin.Context) {
	f := c.Param("filename")
	if !isExposableNote(f) {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	d, err := os.ReadFile(s.Library.FullPath(f))
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "not_found"})
		} else {
			c.JSON(500, gin.H{"error": "read_failed"})
		}
		return
	}
	c.JSON(200, gin.H{"content": string(d)})
}

// handleBulkGetNotes returns all note contents in a single response.
// Eliminates N individual HTTP round-trips during full-text index builds.
// Response format: { "notes": { "filename.md": { "content": "...", "modTime": 123 }, ... } }
func (s *Server) handleBulkGetNotes(c *gin.Context) {
	notes, err := s.Library.ListNotes()
	if err != nil {
		c.JSON(500, gin.H{"error": "list_failed"})
		return
	}
	// Cap total response size at 50 MB to prevent OOM on very large libraries.
	const maxBulkBytes = 50 * 1024 * 1024
	var totalBytes int64
	truncated := false
	result := make(map[string]gin.H, len(notes))
	for _, n := range notes {
		data, err := os.ReadFile(s.Library.FullPath(n.Name))
		if err != nil {
			continue // skip unreadable files
		}
		totalBytes += int64(len(data))
		if totalBytes > maxBulkBytes {
			truncated = true
			break // stop reading further notes
		}
		result[n.Name] = gin.H{"content": string(data), "modTime": n.ModTime}
	}
	c.JSON(200, gin.H{"notes": result, "truncated": truncated})
}

func (s *Server) handleSaveNote(c *gin.Context) {
	f := c.Param("filename")
	// Canonical names can create new notes; non-canonical names (e.g. written by
	// WebDAV clients) can only update existing files to prevent TOCTOU races.
	canonical := s.Library.IsValidName(f) && f != "_structure.json"
	nonCanonical := !canonical && isExposableNote(f)
	if !canonical && !nonCanonical {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	var req NoteRequest
	if err := c.BindJSON(&req); err != nil {
		return
	}
	if err := s.Library.CheckNoteQuota(f, int64(len(req.Content))); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Enforce encryption: reject plaintext when serverEncrypt is enabled.
	// Snapshot ServerEncrypt under mu to avoid a data race with handleUpdateConfig.
	s.Library.mu.Lock()
	serverEncrypt := s.Library.Config.ServerEncrypt
	s.Library.mu.Unlock()
	if serverEncrypt && !strings.HasPrefix(req.Content, "ENC1:") {
		// Allow JSON-encoded ENC1 strings (axios sends `"ENC1:..."`)
		trimmed := strings.TrimSpace(req.Content)
		if !(len(trimmed) > 1 && trimmed[0] == '"' && strings.HasPrefix(trimmed[1:], "ENC1:")) {
			c.JSON(400, gin.H{"error": "encryption_required"})
			return
		}
	}
	if nonCanonical {
		// Non-canonical: only update existing file, do not create.
		err := s.Library.UpdateNote(f, req.Content)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "not_found"})
			} else {
				c.JSON(500, gin.H{"error": "save_failed"})
			}
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
		return
	}
	if err := s.Library.SaveNote(f, req.Content); err != nil {
		c.JSON(500, gin.H{"error": "save_failed"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleDeleteNote(c *gin.Context) {
	f := c.Param("filename")
	if !isExposableNote(f) {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if err := s.Library.DeleteNote(f); err != nil {
		c.JSON(500, gin.H{"error": "delete_failed"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleGetStructure(c *gin.Context) {
	c.String(200, s.Library.GetStructure())
}

func (s *Server) handleSaveStructure(c *gin.Context) {
	d, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read_failed"})
		return
	}
	// Axios JSON-encodes string payloads, so an encrypted blob arrives as a quoted
	// JSON string: `"ENC1:..."`. Unwrap it so the file stores the raw ENC1 value,
	// which the GET handler can return as plain text for direct startsWith detection.
	var encStr string
	if json.Unmarshal(d, &encStr) == nil && strings.HasPrefix(encStr, "ENC1:") {
		if err := s.Library.SaveStructure(encStr); err != nil {
			c.JSON(500, gin.H{"error": "save_failed"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
		return
	}
	// Enforce encryption: reject plaintext structure when serverEncrypt is enabled.
	// Snapshot ServerEncrypt under mu to avoid a data race with handleUpdateConfig.
	s.Library.mu.Lock()
	serverEncryptForStructure := s.Library.Config.ServerEncrypt
	s.Library.mu.Unlock()
	if serverEncryptForStructure {
		c.JSON(400, gin.H{"error": "encryption_required"})
		return
	}
	// Quota and integrity checks apply only to plaintext JSON payloads.
	// Reject payloads that neither start with ENC1 nor parse as a valid Structure
	// with a non-nil order array; this prevents arbitrary JSON from being persisted.
	var st Structure
	if err := json.Unmarshal(d, &st); err != nil || st.Order == nil {
		c.JSON(400, gin.H{"error": "invalid_structure"})
		return
	}
	if err := s.Library.CheckStructureQuota(st); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Reject an empty-order payload when notes exist on disk that are NOT in
	// the trash or nested under a parent. An empty order from the client almost
	// certainly indicates a UI bug — unless every note is either trashed or a
	// child of another note.
	if len(st.Order) == 0 {
		notes, listErr := s.Library.ListNotes()
		if listErr != nil {
			c.JSON(500, gin.H{"error": "integrity_check_failed"})
			return
		}
		// Build sets of trashed and parented IDs so we can count truly "orphaned" notes.
		trashIds := make(map[string]bool, len(st.Trash))
		for _, t := range st.Trash {
			trashIds[t.ID] = true
		}
		parentedIds := make(map[string]bool, len(st.Parents))
		for k := range st.Parents {
			parentedIds[k] = true
		}
		unaccounted := 0
		for _, n := range notes {
			if !trashIds[n.Name] && !parentedIds[n.Name] {
				unaccounted++
			}
		}
		if unaccounted > 0 {
			c.JSON(400, gin.H{"error": "integrity_check_failed_empty_order"})
			return
		}
	}
	if err := s.Library.SaveStructure(string(d)); err != nil {
		c.JSON(500, gin.H{"error": "save_failed"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleUpload(c *gin.Context) {
	f, err := c.FormFile("image")
	if err != nil || f == nil {
		c.JSON(400, gin.H{"error": "no_file"})
		return
	}
	// Only the file extension from the client-supplied filename is used; the rest
	// is discarded. The server generates a fresh canonical filename to prevent
	// directory traversal or filename-based injection from the original name.
	ext := strings.ToLower(filepath.Ext(f.Filename))
	if !validUploadExts[ext] {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if err := s.Library.CheckAssetQuota(f.Size); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	file, err := f.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "open_failed"})
		return
	}
	defer file.Close()
	// Verify MIME type by reading magic bytes. Reject content detected as dangerous
	// types (HTML, JavaScript, XML) that could be rendered by browsers if the
	// X-Content-Type-Options header is stripped by a reverse proxy.
	// Allow: image/*, application/octet-stream, text/plain (encrypted ENC1 blobs).
	header := make([]byte, 512)
	n, _ := file.Read(header)
	if n > 0 {
		detected := http.DetectContentType(header[:n])
		if strings.HasPrefix(detected, "text/html") || strings.HasPrefix(detected, "text/xml") ||
			strings.HasPrefix(detected, "application/javascript") {
			c.JSON(400, gin.H{"error": "invalid_content_type"})
			return
		}
	}
	// Reset reader to beginning for SaveAsset
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}
	name := time.Now().Format("20060102") + randomString(16) + ext
	if err := s.Library.SaveAsset(name, file); err != nil {
		c.JSON(500, gin.H{"error": "save_failed"})
		return
	}
	c.JSON(200, gin.H{"preview_url": "/uploads/" + name, "markdown_url": "./uploads/" + name})
}

// handleListAssets returns filenames of all uploaded assets.
// Only files that pass both the extension allowlist and the canonical name regex
// are included, so any stray files (e.g. .tmp leftovers) are silently excluded.
func (s *Server) handleListAssets(c *gin.Context) {
	dir := s.Library.AssetsPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(200, gin.H{"assets": []string{}})
		return
	}
	assets := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if validUploadExts[ext] && validFileRegex.MatchString(e.Name()) {
				assets = append(assets, e.Name())
			}
		}
	}
	c.JSON(200, gin.H{"assets": assets})
}

// handleOverwriteAsset replaces an existing asset in-place.
// This endpoint exists to support batch re-encryption: when the user changes their
// master key, image assets need to be re-encrypted and uploaded back to the server.
// Restricting to existing files prevents this endpoint from being used as a general
// upload bypass (the POST /upload quota check would be skipped otherwise).
//
// base64OverheadFactor accounts for the ~1.37× size expansion of base64-encoded
// re-encrypted assets; 3× gives generous headroom while still bounding uploads.
const base64OverheadFactor = 3

func (s *Server) handleOverwriteAsset(c *gin.Context) {
	name := c.Param("filename")
	if !validFileRegex.MatchString(name) {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !validUploadExts[ext] {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	// Only allow overwriting files that already exist — not creating new ones.
	assetPath := s.Library.FullAssetPath(name)
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "not_found"})
		return
	}
	f, err := c.FormFile("image")
	if err != nil || f == nil {
		c.JSON(400, gin.H{"error": "no_file"})
		return
	}
	s.Library.mu.Lock()
	maxSize := s.Library.Config.MaxAssetSize * base64OverheadFactor
	s.Library.mu.Unlock()
	if f.Size > maxSize {
		c.JSON(400, gin.H{"error": "limit_asset_size"})
		return
	}
	file, err := f.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "open_failed"})
		return
	}
	defer file.Close()
	// Verify MIME type by reading magic bytes — same check as handleUpload.
	header := make([]byte, 512)
	n, _ := file.Read(header)
	if n > 0 {
		detected := http.DetectContentType(header[:n])
		if strings.HasPrefix(detected, "text/html") || strings.HasPrefix(detected, "text/xml") ||
			strings.HasPrefix(detected, "application/javascript") {
			c.JSON(400, gin.H{"error": "invalid_content_type"})
			return
		}
	}
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}
	if err := s.Library.SaveAsset(name, file); err != nil {
		c.JSON(500, gin.H{"error": "save_failed"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

// handleGetUpload serves an asset file from the uploads directory.
// Placed inside the authenticated API group so that Basic Auth (when configured)
// protects asset files in addition to the JSON API endpoints.
func (s *Server) handleGetUpload(c *gin.Context) {
	name := c.Param("filename")
	if !validFileRegex.MatchString(name) {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	p := s.Library.FullAssetPath(name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "not_found"})
		return
	}
	c.File(p)
}

func (s *Server) handleDeleteAsset(c *gin.Context) {
	name := c.Param("filename")
	if !validFileRegex.MatchString(name) {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !validUploadExts[ext] {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if err := s.Library.DeleteAsset(name); err != nil {
		c.JSON(500, gin.H{"error": "delete_failed"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleGetHistory(c *gin.Context) {
	f := c.Param("filename")
	if !s.Library.IsValidName(f) || f == "_structure.json" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	c.JSON(200, gin.H{"history": s.Library.GetHistory(f, limit)})
}

func (s *Server) handleGetVersion(c *gin.Context) {
	f := c.Param("filename")
	h := c.Param("hash")
	if !s.Library.IsValidName(f) || f == "_structure.json" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if !validGitHashRegex.MatchString(h) {
		c.JSON(400, gin.H{"error": "invalid_hash"})
		return
	}
	content, found := s.Library.GetContentAtHash(f, h)
	if !found {
		c.JSON(404, gin.H{"error": "version_not_found"})
		return
	}
	c.JSON(200, gin.H{"content": content})
}

func (s *Server) handleRollback(c *gin.Context) {
	f := c.Param("filename")
	if !s.Library.IsValidName(f) || f == "_structure.json" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	var req struct {
		Hash string `json:"hash"`
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}
	if !validGitHashRegex.MatchString(req.Hash) {
		c.JSON(400, gin.H{"error": "invalid_hash"})
		return
	}
	content, found := s.Library.GetContentAtHash(f, req.Hash)
	// Use the explicit found flag rather than checking content == "" to distinguish
	// "hash/file not found" from "file was committed with legitimately empty content".
	// The old check (content == "") would incorrectly return 404 when rolling back
	// to a version where the user had cleared the note body.
	if !found {
		c.JSON(404, gin.H{"error": "version_not_found"})
		return
	}
	// Quota check: rolled-back content counts the same as a new save.
	if err := s.Library.CheckNoteQuota(f, int64(len(content))); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := s.Library.SaveNoteAndCommit(f, content, "Rollback "+f); err != nil {
		c.JSON(500, gin.H{"error": "rollback_failed"})
		return
	}
	c.JSON(200, gin.H{"content": content})
}

// secHeaders adds security response headers to every response.
// X-Frame-Options: clickjacking defence.
// X-Content-Type-Options: prevents MIME-sniffing attacks on uploaded assets.
// Referrer-Policy: no referrer sent to third-party resources embedded in exported HTML.
// Permissions-Policy: restricts access to browser APIs not needed by this app.
// Content-Security-Policy: XSS defence in depth.
// 'unsafe-inline' for styles is required by the Tiptap/ProseMirror editor which applies
// dynamic inline styles. All script execution is locked to same-origin only.
// Mermaid v11 uses securityLevel:'strict' (sandboxed iframe + DOMPurify), so 'unsafe-eval'
// is no longer required.
func (s *Server) secHeaders() gin.HandlerFunc {
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"worker-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'"
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", csp)
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if s.UseTLS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// limitSize caps the request body to 20MB to prevent large-payload DoS attacks
// before any handler reads the body.
func (s *Server) limitSize() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20)
		c.Next()
	}
}

// authEntry tracks consecutive failures and the time of the last attempt for one IP.
type authEntry struct {
	count   int
	lastSeen time.Time
}

var (
	authFailures   = make(map[string]*authEntry)
	authFailuresMu sync.Mutex

	// authDelaySem limits the number of goroutines simultaneously sleeping in the
	// progressive-delay path. Without this cap, an attacker with many source IPs
	// could exhaust the goroutine pool by triggering delays from all of them at once.
	authDelaySem = make(chan struct{}, 20)

	// activeTokens holds the set of Bearer tokens issued by the SRP handshake.
	// Tokens are stored as their raw string; each token is a 32-byte random value
	// encoded as base64url. The map is cleared when the password is changed or reset.
	activeTokens       = make(map[string]struct{})
	activeTokenExpiry  = make(map[string]time.Time) // TTL: 24 hours from issue
	activeTokensMu     sync.Mutex
)

func init() {
	// Evict per-IP failure entries that have been idle for more than 30 minutes.
	// This prevents unbounded map growth from rotating-IP attacks while preserving
	// the delay for persistent attackers using the same IP.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			cutoff := time.Now().Add(-30 * time.Minute)
			authFailuresMu.Lock()
			for ip, e := range authFailures {
				if e.lastSeen.Before(cutoff) {
					delete(authFailures, ip)
				}
			}
			authFailuresMu.Unlock()
		}
	}()

	// Evict SRP Bearer tokens that have exceeded their 24-hour TTL.
	// This bounds the activeTokens map size on long-running servers.
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			activeTokensMu.Lock()
			for tok, exp := range activeTokenExpiry {
				if now.After(exp) {
					delete(activeTokens, tok)
					delete(activeTokenExpiry, tok)
				}
			}
			activeTokensMu.Unlock()
		}
	}()
}

// ── Shared auth helpers ───────────────────────────────────────────────────────
//
// applyAuthDelay, recordAuthFailure and clearAuthFailures centralise the
// per-IP progressive-delay logic so that both the Gin API middleware (apiAuth /
// basicAuth) and the plain-http WebDAV handler share the same brute-force
// protection without duplicating code.

// applyAuthDelay sleeps for a progressive duration if the given IP has
// accumulated more than 3 consecutive failures (same formula as before).
func applyAuthDelay(ip string) {
	authFailuresMu.Lock()
	entry := authFailures[ip]
	var count int
	if entry != nil {
		count = entry.count
	}
	authFailuresMu.Unlock()
	if count <= 3 {
		return
	}
	delay := time.Duration(count-3) * 500 * time.Millisecond
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	select {
	case authDelaySem <- struct{}{}:
		time.Sleep(delay)
		<-authDelaySem
	default:
	}
}

// recordAuthFailure increments the consecutive-failure counter for ip.
func recordAuthFailure(ip string) {
	authFailuresMu.Lock()
	if authFailures[ip] == nil {
		authFailures[ip] = &authEntry{}
	}
	authFailures[ip].count++
	authFailures[ip].lastSeen = time.Now()
	authFailuresMu.Unlock()
}

// clearAuthFailures resets the failure counter for ip after a successful auth.
func clearAuthFailures(ip string) {
	authFailuresMu.Lock()
	delete(authFailures, ip)
	authFailuresMu.Unlock()
}

// apiAuth is the primary authentication middleware.
// Priority:
//  1. SRPVerifier set → require "Authorization: Bearer <token>" that was issued
//     by a completed SRP-6a handshake and is present in the in-memory activeTokens set.
//  2. AUTH_USER env set → fall back to HTTP Basic Auth (legacy deployments).
//  3. Neither set → open access (keyless / initial setup phase).
//
// Progressive delay is applied on Bearer token mismatches to slow brute-force
// attempts without locking out legitimate users.
func (s *Server) apiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.Library.mu.Lock()
		srpVerifier := s.Library.Config.SRPVerifier
		s.Library.mu.Unlock()

		if srpVerifier != "" {
			// SRP auth is configured — require a valid Bearer token from the active set.
			ip := c.ClientIP()
			applyAuthDelay(ip)

			auth := c.GetHeader("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if !strings.HasPrefix(auth, "Bearer ") || token == "" {
				recordAuthFailure(ip)
				c.AbortWithStatus(401)
				return
			}

			activeTokensMu.Lock()
			_, valid := activeTokens[token]
			activeTokensMu.Unlock()

			if !valid {
				recordAuthFailure(ip)
				c.AbortWithStatus(401)
				return
			}
			clearAuthFailures(ip)
			c.Next()
			return
		}

		// No SRP verifier — fall back to Basic Auth if configured, otherwise open.
		if os.Getenv("AUTH_USER") != "" {
			s.basicAuth()(c)
			return
		}
		c.Next()
	}
}

// handleAuthStatus returns whether the server has a password configured,
// and the pbkdf2Salt for cross-device key derivation.
// Unauthenticated — used by new devices to decide whether to show the
// "enter password" flow instead of the "initialize library" (first-time) flow.
// pbkdf2Salt is returned here (not via GET /api/config) so new devices can
// import it before performing key derivation, without needing a Bearer token.
func (s *Server) handleAuthStatus(c *gin.Context) {
	s.Library.mu.Lock()
	initialized := s.Library.Config.SRPVerifier != ""
	pbkdf2Salt := s.Library.Config.Pbkdf2Salt
	s.Library.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"initialized": initialized,
		"pbkdf2Salt":  pbkdf2Salt,
	})
}

// handleAuthSetup stores the SRP verifier and salt for a new password, or
// clears them (reverts to open/keyless access) when the verifier is empty.
//
//   - First call (no SRPVerifier stored): succeeds unconditionally — "claim"
//     window immediately after a fresh deployment.
//   - srpVerifier non-empty: stores/rotates the verifier; requires a valid
//     Bearer token if a verifier is already set.
//   - srpVerifier empty string: clears auth (reverts to keyless access);
//     requires a valid Bearer token if a verifier is already set.
func (s *Server) handleAuthSetup(c *gin.Context) {
	var req struct {
		SRPSalt     string `json:"srpSalt"`
		SRPVerifier string `json:"srpVerifier"`
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	// If both are empty, treat as a clear request (handled below).
	// If setting, both fields must be present.
	if req.SRPVerifier != "" && req.SRPSalt == "" {
		c.JSON(400, gin.H{"error": "srpSalt required when setting srpVerifier"})
		return
	}

	ip := c.ClientIP()

	s.Library.mu.Lock()
	existingVerifier := s.Library.Config.SRPVerifier

	if existingVerifier != "" {
		// Verifier already set — caller must present a valid active Bearer token.
		s.Library.mu.Unlock()
		applyAuthDelay(ip)
		auth := c.GetHeader("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") || token == "" {
			recordAuthFailure(ip)
			c.AbortWithStatus(401)
			return
		}
		activeTokensMu.Lock()
		_, valid := activeTokens[token]
		activeTokensMu.Unlock()
		if !valid {
			recordAuthFailure(ip)
			c.AbortWithStatus(401)
			return
		}
		clearAuthFailures(ip)
		s.Library.mu.Lock()
		// TOCTOU guard: re-check verifier hasn't changed while we were authenticating.
		if s.Library.Config.SRPVerifier != existingVerifier {
			s.Library.mu.Unlock()
			c.JSON(409, gin.H{"error": "concurrent_modification"})
			return
		}
	}

	// Persist to disk before updating in-memory state.
	oldSalt := s.Library.Config.SRPSalt
	oldVerifier := s.Library.Config.SRPVerifier
	s.Library.Config.SRPSalt = req.SRPSalt
	s.Library.Config.SRPVerifier = req.SRPVerifier
	cfg := s.Library.Config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		s.Library.Config.SRPSalt = oldSalt
		s.Library.Config.SRPVerifier = oldVerifier
		s.Library.mu.Unlock()
		c.JSON(500, gin.H{"error": "marshal_failed"})
		return
	}
	if err := atomicWriteFile(s.Library.ConfigPath, data, 0600); err != nil {
		s.Library.Config.SRPSalt = oldSalt
		s.Library.Config.SRPVerifier = oldVerifier
		s.Library.mu.Unlock()
		c.JSON(500, gin.H{"error": "write_failed"})
		return
	}
	s.Library.mu.Unlock()

	// Invalidate all existing sessions whenever auth is changed or cleared.
	activeTokensMu.Lock()
	activeTokens = make(map[string]struct{})
	activeTokenExpiry = make(map[string]time.Time)
	activeTokensMu.Unlock()

	c.JSON(200, gin.H{"ok": true})
}

// handleSRPInit starts an SRP-6a handshake.
// The client sends its ephemeral public key A; the server responds with B
// and the stored SRP salt. The session is keyed by pad256(A) and expires
// after 5 minutes if not completed with /srp/verify.
func (s *Server) handleSRPInit(c *gin.Context) {
	var req struct {
		A string `json:"A"` // client ephemeral public key, hex string
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}
	if req.A == "" {
		c.JSON(400, gin.H{"error": "A required"})
		return
	}

	s.Library.mu.Lock()
	verifier := s.Library.Config.SRPVerifier
	salt := s.Library.Config.SRPSalt
	s.Library.mu.Unlock()

	if verifier == "" {
		// No password set — SRP handshake is meaningless.
		c.JSON(400, gin.H{"error": "not_initialized"})
		return
	}

	ip := c.ClientIP()
	applyAuthDelay(ip)

	bHex, err := srpInitHandshake(req.A, verifier)
	if err != nil {
		recordAuthFailure(ip)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"salt": salt,
		"B":    bHex,
	})
}

// handleSRPVerify completes the SRP-6a handshake.
// The client sends M1 (proof of password knowledge); the server validates it,
// issues a Bearer token, and returns M2 (proof of server knowledge).
func (s *Server) handleSRPVerify(c *gin.Context) {
	var req struct {
		A  string `json:"A"`  // same A sent to /srp/init, for session lookup
		M1 string `json:"M1"` // client proof
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}
	if req.A == "" || req.M1 == "" {
		c.JSON(400, gin.H{"error": "A and M1 required"})
		return
	}

	ip := c.ClientIP()
	applyAuthDelay(ip)

	m2Hex, token, err := srpVerifyHandshake(req.A, req.M1)
	if err != nil {
		recordAuthFailure(ip)
		// Return a uniform 401 for all verify failures to prevent the caller
		// from distinguishing "session not found" from "wrong password" via
		// HTTP status codes or error body strings.
		c.JSON(401, gin.H{"error": "authentication_failed"})
		return
	}

	// Register the issued token with a 24-hour TTL.
	activeTokensMu.Lock()
	activeTokens[token] = struct{}{}
	activeTokenExpiry[token] = time.Now().Add(24 * time.Hour)
	activeTokensMu.Unlock()

	clearAuthFailures(ip)
	c.JSON(200, gin.H{
		"token": token,
		"M2":    m2Hex,
	})
}

// handleTestResetAuth clears SRPVerifier and SRPSalt unconditionally.
// handleWebDAVSetToken generates a new WebDAV token, stores its SHA-256 hash in
// config.json, and returns the raw token once. Replaces any existing token.
func (s *Server) handleWebDAVSetToken(c *gin.Context) {
	rawToken := randomString(48)
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	if err := s.persistWebDAVTokenHash(hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": rawToken})
}

// handleWebDAVRevokeToken removes the WebDAV token hash, disabling WebDAV access.
func (s *Server) handleWebDAVRevokeToken(c *gin.Context) {
	if err := s.persistWebDAVTokenHash(""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) persistWebDAVTokenHash(hash string) error {
	s.Library.mu.Lock()
	defer s.Library.mu.Unlock()
	oldHash := s.Library.Config.WebDAVTokenHash
	s.Library.Config.WebDAVTokenHash = hash
	data, err := json.MarshalIndent(s.Library.Config, "", "  ")
	if err != nil {
		s.Library.Config.WebDAVTokenHash = oldHash
		return err
	}
	if err := atomicWriteFile(s.Library.ConfigPath, data, 0600); err != nil {
		s.Library.Config.WebDAVTokenHash = oldHash
		return err
	}
	return nil
}

// Registered ONLY when SYNC_COMMIT=1 (E2E test environment).
// Allows E2E teardown to restore open/keyless access after password-mode tests
// without needing the current session token.
func (s *Server) handleTestResetAuth(c *gin.Context) {
	s.Library.mu.Lock()
	oldSalt := s.Library.Config.SRPSalt
	oldVerifier := s.Library.Config.SRPVerifier
	s.Library.Config.SRPSalt = ""
	s.Library.Config.SRPVerifier = ""
	cfg := s.Library.Config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		s.Library.Config.SRPSalt = oldSalt
		s.Library.Config.SRPVerifier = oldVerifier
		s.Library.mu.Unlock()
		c.JSON(500, gin.H{"error": "marshal_failed"})
		return
	}
	if err := atomicWriteFile(s.Library.ConfigPath, data, 0600); err != nil {
		s.Library.Config.SRPSalt = oldSalt
		s.Library.Config.SRPVerifier = oldVerifier
		s.Library.mu.Unlock()
		c.JSON(500, gin.H{"error": "write_failed"})
		return
	}
	s.Library.mu.Unlock()

	// Invalidate all active Bearer tokens on reset.
	activeTokensMu.Lock()
	activeTokens = make(map[string]struct{})
	activeTokenExpiry = make(map[string]time.Time)
	activeTokensMu.Unlock()

	c.JSON(200, gin.H{"status": "ok"})
}

// basicAuth wraps Gin's standard BasicAuth with per-IP progressive delay.
// After 3 consecutive failures from the same IP, each additional failure adds
// 500ms of delay (capped at 5s). This is not a lockout — it slows brute-force
// attempts enough to make them impractical without locking out legitimate users
// who mistype their password. Successful auth resets the counter for that IP.
// Basic Auth is a network-access layer only and does not protect the encrypted data —
// even if bypassed, the attacker still needs the master encryption key.
func (s *Server) basicAuth() gin.HandlerFunc {
	u, p := os.Getenv("AUTH_USER"), os.Getenv("AUTH_PASS")
	if u == "" {
		return func(c *gin.Context) { c.Next() }
	}
	standardAuth := gin.BasicAuth(gin.Accounts{u: p})
	return func(c *gin.Context) {
		ip := c.ClientIP()
		applyAuthDelay(ip)
		standardAuth(c)
		// Use IsAborted() rather than checking status == 200: gin.BasicAuth calls
		// c.Abort() on failure, so !c.IsAborted() reliably indicates success even
		// if the downstream handler returns a non-200 success code.
		if c.IsAborted() {
			recordAuthFailure(ip)
		} else {
			clearAuthFailures(ip)
		}
	}
}

func (s *Server) handleSPA(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api") {
		c.Status(404)
		return
	}
	// Read index.html directly from the embed FS to avoid Go's net/http file
	// server redirect: ServeHTTP redirects any path ending in "/index.html"
	// to "./" (the parent directory), which causes a redirect loop when
	// c.FileFromFS("dist/index.html", ...) is used.
	data, err := fs.ReadFile(staticFiles, "dist/index.html")
	if err != nil {
		c.Status(404)
		return
	}
	// Prevent browsers from caching index.html so updated JS bundles are always
	// fetched after a redeployment. Static assets use content-hashed filenames
	// and are safe to cache indefinitely; index.html must always be fresh.
	// Set Cache-Control BEFORE c.Data() — c.Data writes headers and body together.
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// randomString generates a cryptographically secure random string of length n
// using the character set [a-z0-9]. Rejection sampling eliminates the modulo
// bias that would arise from naively taking byte % len(charset) when 256 is not
// divisible by len(charset) (36 chars → 256%36=4 → 1.6% of bytes rejected).
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	// limit is the largest multiple of len(charset) that fits in a byte (0–255).
	// Bytes at or above this threshold are discarded to ensure uniform distribution.
	limit := byte(256 - 256%len(charset)) // 252 for charset length 36
	result := make([]byte, 0, n)
	for len(result) < n {
		// Generate extra bytes to minimise the number of crypto/rand calls.
		buf := make([]byte, n*2)
		if _, err := rand.Read(buf); err != nil {
			panic("crypto/rand unavailable: " + err.Error())
		}
		for _, b := range buf {
			if b < limit && len(result) < n {
				result = append(result, charset[int(b)%len(charset)])
			}
		}
	}
	return string(result)
}
