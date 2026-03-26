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
		return &davCommitFile{File: f, lib: dfs.lib, rel: rel, written: truncated}, nil
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
		dfs.lib.markPending(strings.TrimPrefix(oldName, "/"))
		dfs.lib.markPending(strings.TrimPrefix(newName, "/"))
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

// allowed returns false for paths that must not be exposed via WebDAV.
// Blocked: _structure.json (YinMoNote internal), hidden files/dirs (e.g. .git),
// and any filename that does not match the canonical note/asset format.
func (dfs *davFileSystem) allowed(name string) bool {
	// Walk each segment of the path so that e.g. /.git/config is also blocked.
	for _, seg := range strings.Split(name, "/") {
		if seg == "" {
			continue
		}
		if seg == "_structure.json" || strings.HasPrefix(seg, ".") {
			return false
		}
		// Reject non-canonical filenames — only note and asset filenames
		// matching the canonical format (date + random + extension) are permitted.
		if !dfs.lib.IsValidName(seg) {
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
		n := e.Name()
		// Skip internal files and hidden paths (e.g. .git).
		if n == "_structure.json" || strings.HasPrefix(n, ".") {
			continue
		}
		// Skip subdirectories (e.g. assets/) and any file that does not match
		// the canonical note/asset filename format. This prevents walkFS from
		// calling Stat on paths that allowed() would reject, which would
		// otherwise cause PROPFIND to return "Internal Server Error".
		if e.IsDir() || !validFileRegex.MatchString(n) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, err
}

// ── davCommitFile ─────────────────────────────────────────────────────────────

// davCommitFile wraps a webdav.File opened for writing and calls markPending
// when the file is closed after at least one successful Write call.
type davCommitFile struct {
	webdav.File
	lib     *NoteLibrary
	rel     string
	written bool
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
		f.lib.markPending(f.rel)
	}
	return err
}

// ── Handler factory ───────────────────────────────────────────────────────────

// newDavHandler returns an http.Handler that serves WebDAV at the /dav prefix.
//
// Authentication mirrors the REST API session-token model:
//   - SessionTokenHash set in config → require Basic Auth where the password is
//     the raw session token (SHA-256 of password must match the stored hash).
//     The username field is ignored — any value is accepted.
//   - SessionTokenHash not set → open access (same as keyless REST API mode).
//
// This means WebDAV clients use the same credential as the web app.
// In Obsidian/iA Writer: username = anything (e.g. "yinmonote"), password = the
// session token shown in the app's Settings → WebDAV section.
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
		}
		ip := davClientIP(r)

		s.Library.mu.Lock()
		tokenHash := s.Library.Config.SessionTokenHash
		s.Library.mu.Unlock()

		if tokenHash != "" {
			applyAuthDelay(ip)
			_, pass, ok := r.BasicAuth()
			if !ok || pass == "" {
				recordAuthFailure(ip)
				w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			got := sha256.Sum256([]byte(pass))
			want, err := hex.DecodeString(tokenHash)
			if err != nil {
				// tokenHash in config.json is not valid hex — treat as permanent auth failure.
				fmt.Fprintf(os.Stderr, "YinMo: WebDAV tokenHash is not valid hex: %v\n", err)
				w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare(got[:], want) != 1 {
				recordAuthFailure(ip)
				w.Header().Set("WWW-Authenticate", `Basic realm="YinMoNote"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			clearAuthFailures(ip)
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
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	return host
}
