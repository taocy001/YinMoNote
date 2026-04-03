package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/webdav"
)

// isBlockedSegment returns true for filename segments that must never be
// exposed via WebDAV, either in path access checks or directory listings.
func isBlockedSegment(seg string) bool {
	if seg == "_structure.json" {
		return true
	}
	if strings.HasPrefix(seg, ".") {
		return true
	}
	if strings.HasPrefix(seg, "~") || strings.HasSuffix(seg, "~") {
		return true
	}
	if strings.ContainsRune(seg, 0) {
		return true
	}
	if len(seg) > 255 {
		return true
	}
	// Remotely Save connection-test artefacts — never create real notes for these.
	// Case-insensitive so RS-TEST- / Rs-Test- variants are also blocked.
	if strings.HasPrefix(strings.ToLower(seg), "rs-test-") {
		return true
	}
	return false
}

// allowed returns false for paths that must not be exposed via WebDAV.
//
// Blacklisted (rejected regardless of position in path):
//   - "_structure.json" (YinMoNote internal index)
//   - "." prefix (hidden files/dirs, e.g. .git)
//   - "~" prefix or suffix (editor temp files, e.g. foo~, ~foo)
//   - null byte in any segment (path-injection guard)
//   - any segment longer than 255 bytes (filesystem limit)
//
// Path depth: at most 5 segments are allowed (measured after normalizePath has
// stripped the leading vault prefix).  The note store is intentionally flat so
// only depth 1 is expected in practice; the higher cap accommodates clients
// that create subdirectories for attachments or templates.
//
//	depth 1 — root files            e.g. "note.md"
//	depth 2 — one subfolder         e.g. "assets/image.png"
//	depth 3 — two subfolders        e.g. "assets/2024/photo.png"
//
// Paths deeper than 5 segments are rejected to bound directory tree growth.
func (dfs *davFileSystem) allowed(name string) bool {
	segments := strings.Split(strings.TrimPrefix(name, "/"), "/")
	nonEmpty := 0
	for _, seg := range segments {
		if seg != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 5 {
		return false
	}
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if isBlockedSegment(seg) {
			return false
		}
	}
	return true
}

// ── davDirFile ────────────────────────────────────────────────────────────────

// davDirFile wraps a directory webdav.File and:
//  1. Filters out blocked entries (_structure.json, hidden files) from Readdir.
//  2. Translates canonical-ID note filenames to human-readable title names
//     when m is non-nil (root directory only).
type davDirFile struct {
	webdav.File
	m *davNameMap // non-nil only for root directory; translates ID→title in listings
}

func (f *davDirFile) Readdir(count int) ([]os.FileInfo, error) {
	entries, err := f.File.Readdir(count)
	filtered := entries[:0]
	for _, e := range entries {
		if isBlockedSegment(e.Name()) {
			continue
		}
		// Translate canonical-ID note filenames to title names for root listings.
		if f.m != nil && !e.IsDir() {
			if title, ok := f.m.idToTitle[e.Name()]; ok {
				filtered = append(filtered, &davTitleFileInfo{FileInfo: e, name: title})
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered, err
}

// ── davCommitFile ─────────────────────────────────────────────────────────────

// davCommitFile wraps a webdav.File opened for writing and calls markPending
// when the file is closed after at least one successful Write call.
// isNew is set when the target file did not exist at OpenFile time, enabling
// total-note-count quota enforcement (checked at open) and per-file size quota
// enforcement (checked at close via Stat on the written file).
type davCommitFile struct {
	webdav.File
	lib     *NoteLibrary
	rel     string
	written bool
	isNew   bool // target file was absent when OpenFile was called
}

func (f *davCommitFile) Write(p []byte) (int, error) {
	n, err := f.File.Write(p)
	if n > 0 {
		f.written = true
	}
	return n, err
}

func (f *davCommitFile) Close() error {
	err := f.File.Close()
	if err == nil && f.rel != "" && f.written {
		if strings.HasSuffix(f.rel, ".md") {
			f.lib.mu.Lock()
			maxNoteSize := f.lib.Config.MaxNoteSize
			f.lib.mu.Unlock()
			info, statErr := os.Stat(f.lib.FullPath(f.rel))
			if statErr == nil && info.Size() > maxNoteSize {
				if f.isNew {
					if rmErr := os.Remove(f.lib.FullPath(f.rel)); rmErr != nil {
						fmt.Fprintf(os.Stderr, "YinMo: WebDAV oversized note cleanup failed: %v\n", rmErr)
					}
				} else {
					// Existing file: O_TRUNC already destroyed the original content at
					// OpenFile time. Remove the partially-written oversized file so the
					// note does not silently linger at 0 bytes or at a partial size.
					if rmErr := os.Remove(f.lib.FullPath(f.rel)); rmErr != nil {
						fmt.Fprintf(os.Stderr, "YinMo: WebDAV oversized note cleanup (overwrite) failed: %v\n", rmErr)
					}
				}
				f.lib.reconcilePending.Store(true)
				return &davQuotaError{reason: ErrLimitNoteSize.Error()}
			}
		}
		f.lib.markPending(f.rel)
		if strings.HasSuffix(f.rel, ".md") {
			f.lib.reconcilePending.Store(true)
		}
	}
	return err
}

// ── Handler factory ───────────────────────────────────────────────────────────

// newDavHandler returns an http.Handler that serves WebDAV at the /dav prefix.
//
// Authentication uses a persistent static WebDAV token (SHA-256 hash stored in
// config.json as webdavTokenHash).  The token is generated via the Settings UI
// and survives server restarts.
//
// Access rules:
//   - webdavTokenHash set → require Basic Auth; password must be the raw token.
//     Username is ignored — any value is accepted.
//   - webdavTokenHash not set, SRPVerifier set → deny (server has a password but
//     no WebDAV token has been issued; do not open WebDAV to anonymous access).
//   - webdavTokenHash not set, SRPVerifier not set → open access (keyless mode).
//
// Brute-force protection: the same per-IP progressive-delay mechanism
// (applyAuthDelay / recordAuthFailure / clearAuthFailures) is applied here.
//
// Note: WebDAV only exposes plaintext note content. When serverEncrypt is enabled
// files on disk contain an "ENC1:" prefix unreadable by third-party apps.
// Disable serverEncrypt before using WebDAV clients.
func (s *Server) newDavHandler() http.Handler {
	dfs := &davFileSystem{
		inner: webdav.Dir(s.Library.DataDir),
		lib:   s.Library,
	}
	davH := &webdav.Handler{
		Prefix:     "/dav",
		FileSystem: dfs,
		LockSystem: webdav.NewMemLS(),
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply security headers to all WebDAV responses, matching the Gin router's
		// secHeaders middleware which is bypassed by the WebDAV dispatch path.
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if s.UseTLS {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Enforce a 20 MB body size limit on all request methods that carry a body.
		// PROPFIND requests can carry arbitrary XML (e.g. deeply nested property trees)
		// and are included here to prevent memory-exhaustion DoS.
		if r.Method == http.MethodPut || r.Method == "PROPFIND" ||
			r.Method == "PROPPATCH" || r.Method == "MKCOL" {
			r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
		} else if r.Method == "LOCK" {
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		}

		ip := davClientIP(r)

		s.Library.mu.Lock()
		srpVerifier := s.Library.Config.SRPVerifier
		webdavTokenHash := s.Library.Config.WebDAVTokenHash
		serverEncrypt := s.Library.Config.ServerEncrypt
		s.Library.mu.Unlock()

		// If the server has a password but no WebDAV token has been issued,
		// deny all WebDAV access rather than leaving it open.
		if srpVerifier != "" && webdavTokenHash == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if webdavTokenHash != "" {
			applyAuthDelay(ip)
			_, pass, ok := r.BasicAuth()
			if !ok || pass == "" {
				recordAuthFailure(ip)
				w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			sum := sha256.Sum256([]byte(pass))
			got := hex.EncodeToString(sum[:])
			if subtle.ConstantTimeCompare([]byte(got), []byte(webdavTokenHash)) != 1 {
				recordAuthFailure(ip)
				w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			clearAuthFailures(ip)
		}
		// RFC 4918 §9.1: servers MAY reject Depth:infinity PROPFIND requests.
		// An unlimited depth traversal over a large note library can generate a
		// multi-megabyte response with no server-side bound; we reject it to
		// prevent authenticated DoS. Depth:0 and Depth:1 are always accepted.
		// Check is placed after authentication so unauthenticated clients cannot
		// probe server behaviour via the response status code.
		if r.Method == "PROPFIND" && strings.EqualFold(r.Header.Get("Depth"), "infinity") {
			http.Error(w, "Depth: infinity is not supported", http.StatusForbidden)
			return
		}
		// Reject WebDAV access when server-side encryption is enabled.
		// Files on disk contain ENC1-prefixed ciphertext unreadable by third-party
		// WebDAV apps, and any write would corrupt the encrypted note store.
		if serverEncrypt {
			w.Header().Set("X-YinMo-Error", "server-encrypt-incompatible")
			http.Error(w,
				"WebDAV is not available when server-side encryption is enabled. "+
					"Disable server encryption in Settings before using WebDAV clients.",
				http.StatusServiceUnavailable)
			return
		}
		davH.ServeHTTP(w, r)
	})
}

// davClientIP extracts the client IP from r.RemoteAddr, honouring X-Forwarded-For
// when the connection arrives from a trusted loopback proxy (127.0.0.1 or ::1),
// mirroring the trusted-proxy logic configured in the Gin router.
func davClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			var candidate string
			if i := strings.LastIndexByte(xff, ','); i >= 0 {
				candidate = strings.TrimSpace(xff[i+1:])
			} else {
				candidate = strings.TrimSpace(xff)
			}
			// If the rightmost XFF segment is empty (trailing comma or whitespace-only),
			// fall back to the direct RemoteAddr rather than returning "" which would
			// cause all such requests to share a single empty-string auth-failure bucket.
			if candidate != "" {
				return candidate
			}
		}
	}
	return host
}
