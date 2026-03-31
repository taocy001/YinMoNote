package main

import (
	"context"
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

// davQuotaError is returned by davFileSystem.OpenFile and davCommitFile.Close
// when a WebDAV write would exceed a configured quota.
// It wraps os.ErrPermission so the webdav package translates it to 403 Forbidden.
type davQuotaError struct{ reason string }

func (e *davQuotaError) Error() string  { return "quota exceeded: " + e.reason }
func (e *davQuotaError) Is(target error) bool { return target == os.ErrPermission }
func (e *davQuotaError) Unwrap() error  { return os.ErrPermission }

// davFileSystem wraps webdav.Dir to:
//  1. Block access to internal YinMoNote files (_structure.json, hidden files).
//  2. Filter blocked entries from directory listings so clients never see them.
//  3. Queue written/deleted files for git commit via markPending.
type davFileSystem struct {
	inner webdav.Dir
	lib   *NoteLibrary
}

func (dfs *davFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if !dfs.allowed(name) {
		return os.ErrPermission
	}
	return dfs.inner.Mkdir(ctx, name, perm)
}

func (dfs *davFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if !dfs.allowed(name) {
		return nil, os.ErrPermission
	}
	// For new .md files (O_CREATE and file absent), enforce the total-note-count quota
	// BEFORE inner.OpenFile creates the file on disk. This prevents the race where
	// inner.OpenFile creates the file and the subsequent stat finds it already present.
	// Size quota is deferred to Close (the content is not yet known at open time).
	isNew := false
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		rel := strings.TrimPrefix(name, "/")
		if flag&os.O_CREATE != 0 && strings.HasSuffix(rel, ".md") {
			if _, statErr := os.Stat(dfs.lib.FullPath(rel)); os.IsNotExist(statErr) {
				isNew = true
				if err := dfs.lib.CheckNoteQuota(rel, 0); err != nil {
					return nil, &davQuotaError{reason: err.Error()}
				}
			}
		}
	}
	f, err := dfs.inner.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}
	// Wrap directory opens to filter blocked entries from PROPFIND listings.
	if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
		return &davDirFile{File: f}, nil
	}
	// Wrap write-mode opens to queue a git commit when the file is closed.
	// O_TRUNC pre-sets written=true so zero-byte truncation is also committed.
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		rel := strings.TrimPrefix(name, "/")
		truncated := flag&os.O_TRUNC != 0
		return &davCommitFile{File: f, lib: dfs.lib, rel: rel, written: truncated, isNew: isNew}, nil
	}
	return f, nil
}

func (dfs *davFileSystem) RemoveAll(ctx context.Context, name string) error {
	if !dfs.allowed(name) {
		return os.ErrPermission
	}
	err := dfs.inner.RemoveAll(ctx, name)
	if err == nil {
		rel := strings.TrimPrefix(name, "/")
		if rel != "" {
			dfs.lib.markPending(rel)
			if strings.HasSuffix(rel, ".md") {
				dfs.lib.reconcilePending.Store(true)
			}
		}
	}
	return err
}

func (dfs *davFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	if !dfs.allowed(oldName) || !dfs.allowed(newName) {
		return os.ErrPermission
	}
	err := dfs.inner.Rename(ctx, oldName, newName)
	if err == nil {
		oldRel := strings.TrimPrefix(oldName, "/")
		newRel := strings.TrimPrefix(newName, "/")
		dfs.lib.markPending(oldRel)
		dfs.lib.markPending(newRel)
		if strings.HasSuffix(oldRel, ".md") || strings.HasSuffix(newRel, ".md") {
			dfs.lib.reconcilePending.Store(true)
		}
	}
	return err
}

func (dfs *davFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if !dfs.allowed(name) {
		// Return ErrPermission (not ErrNotExist) so that the webdav PROPFIND
		// walkFS silently skips blocked entries via handlePropfindError rather
		// than aborting the response with "Internal Server Error".
		return nil, os.ErrPermission
	}
	return dfs.inner.Stat(ctx, name)
}

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
// Path depth: at most 2 segments are allowed (root-level files and one
// subdirectory, e.g. "assets/filename"). Deeper paths are rejected.
func (dfs *davFileSystem) allowed(name string) bool {
	segments := strings.Split(strings.TrimPrefix(name, "/"), "/")
	// Reject paths deeper than assets/<file> (2 levels).
	nonEmpty := 0
	for _, seg := range segments {
		if seg != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 2 {
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

// davDirFile wraps a directory webdav.File and filters out blocked entries
// (_structure.json, hidden files) from Readdir so clients never see them.
type davDirFile struct {
	webdav.File
}

func (f *davDirFile) Readdir(count int) ([]os.FileInfo, error) {
	entries, err := f.File.Readdir(count)
	filtered := entries[:0]
	for _, e := range entries {
		if isBlockedSegment(e.Name()) {
			continue
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
			info, statErr := os.Stat(f.lib.FullPath(f.rel))
			if statErr == nil && info.Size() > f.lib.Config.MaxNoteSize {
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
				return &davQuotaError{reason: "limit_note_size"}
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
			if i := strings.LastIndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[i+1:])
			}
			return strings.TrimSpace(xff)
		}
	}
	return host
}
