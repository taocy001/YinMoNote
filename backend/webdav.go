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
	"unicode/utf8"

	"golang.org/x/net/webdav"
)

// davQuotaError is returned by davFileSystem.OpenFile and davCommitFile.Close
// when a WebDAV write would exceed a configured quota.
// It wraps os.ErrPermission so the webdav package translates it to 403 Forbidden.
type davQuotaError struct{ reason string }

func (e *davQuotaError) Error() string      { return "quota exceeded: " + e.reason }
func (e *davQuotaError) Is(target error) bool { return target == os.ErrPermission }
func (e *davQuotaError) Unwrap() error       { return os.ErrPermission }

// ── Title virtualisation layer ────────────────────────────────────────────────

// davNameMap holds a bidirectional mapping between canonical-ID filenames and
// their human-readable title names for root-level .md files.
// Only files whose names match validFileRegex are translated; non-canonical .md
// files (e.g. "My Note.md" written by Obsidian) are exposed unchanged.
type davNameMap struct {
	titleToID map[string]string // "数据中心网络.md" → "20260329g6k7v44jm13kq9av.md"
	idToTitle map[string]string // "20260329g6k7v44jm13kq9av.md" → "数据中心网络.md"
}

// davTitleFileInfo wraps an os.FileInfo but returns a different (title-based) name.
// Used by davDirFile.Readdir to present canonical-ID notes under readable filenames.
type davTitleFileInfo struct {
	os.FileInfo
	name string
}

func (f *davTitleFileInfo) Name() string { return f.name }

// davSanitizeTitle converts a note title to a safe filename stem by replacing
// characters that are invalid or problematic on common filesystems.
func davSanitizeTitle(title string) string {
	if title == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range title {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', 0:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.TrimSpace(b.String())
	const maxBytes = 200
	if len(s) > maxBytes {
		s = s[:maxBytes]
		for !utf8.ValidString(s) && len(s) > 0 {
			s = s[:len(s)-1]
		}
	}
	return s
}

// buildTitleMap constructs the bidirectional ID↔title mapping by scanning the library.
// It is called on every write/stat/readdir operation; cost is one os.ReadDir + up to
// MaxTotalNotes × 512-byte file peeks, which is fast enough for interactive use.
func (dfs *davFileSystem) buildTitleMap() *davNameMap {
	m := &davNameMap{
		titleToID: make(map[string]string),
		idToTitle: make(map[string]string),
	}
	notes, err := dfs.lib.ListNotes()
	if err != nil {
		return m
	}
	seen := make(map[string]int) // sanitized base → count of collisions
	for _, note := range notes {
		if !validFileRegex.MatchString(note.Name) {
			continue // non-canonical filename: shown as-is, no translation needed
		}
		base := davSanitizeTitle(note.Title)
		if base == "" {
			base = strings.TrimSuffix(note.Name, ".md")
		}
		n := seen[base]
		seen[base]++
		var davName string
		if n == 0 {
			davName = base + ".md"
		} else {
			davName = fmt.Sprintf("%s (%d).md", base, n+1)
		}
		m.titleToID[davName] = note.Name
		m.idToTitle[note.Name] = davName
	}
	return m
}

// translateToID converts a root-level title-based .md path to the corresponding
// canonical-ID path, or returns the original path when no mapping exists.
// Subdirectory paths and non-.md names are always returned unchanged.
func (m *davNameMap) translateToID(name string) string {
	rel := strings.TrimPrefix(name, "/")
	if strings.ContainsRune(rel, '/') || !strings.HasSuffix(rel, ".md") {
		return name // subdirectory path or non-.md: pass through
	}
	if id, ok := m.titleToID[rel]; ok {
		return "/" + id
	}
	return name // not in map (new file or already canonical): pass through
}

// updateNoteH1 replaces the first non-empty line of the note file (its title line)
// with a new H1 heading formed from newTitle. Used by Rename to implement
// title-rename via content update rather than a file-system rename.
// If the file is encrypted (ENC1: prefix), the call is a no-op.
func (dfs *davFileSystem) updateNoteH1(idFilename, newTitle string) error {
	path := dfs.lib.FullPath(idFilename)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(raw)
	if strings.HasPrefix(content, "ENC1:") {
		return os.ErrPermission // cannot rewrite encrypted content
	}
	lines := strings.Split(content, "\n")
	replaced := false
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = "# " + newTitle
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append([]string{"# " + newTitle}, lines...)
	}
	if err := atomicWriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return err
	}
	dfs.lib.markPending(idFilename)
	dfs.lib.reconcilePending.Store(true)
	return nil
}

// ── davFileSystem ─────────────────────────────────────────────────────────────

// davFileSystem wraps webdav.Dir to:
//  1. Block access to internal YinMoNote files (_structure.json, hidden files).
//  2. Filter blocked entries from directory listings so clients never see them.
//  3. Translate canonical-ID note filenames to human-readable title names so that
//     WebDAV clients (e.g. Obsidian) see "数据中心网络.md" instead of "20260329…md".
//  4. Queue written/deleted files for git commit via markPending.
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

	// ── Title→ID translation for root-level .md files ─────────────────────────
	// Build the mapping on every open so we always have a fresh view. This covers
	// reads (GET, PROPFIND stat), writes (PUT existing), and directory opens.
	rel := strings.TrimPrefix(name, "/")
	isRootMD := !strings.ContainsRune(rel, '/') && strings.HasSuffix(rel, ".md")

	if isRootMD {
		m := dfs.buildTitleMap()
		name = m.translateToID(name)
		rel = strings.TrimPrefix(name, "/")
	}

	// ── Quota check for new canonical .md files ───────────────────────────────
	isNew := false
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
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

	// Wrap directory opens to filter blocked entries and attach the title map.
	if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
		// Only pass the title map for the root directory listing.
		var m *davNameMap
		if rel == "" || rel == "." {
			m = dfs.buildTitleMap()
		}
		return &davDirFile{File: f, m: m}, nil
	}

	// Wrap write-mode opens to queue a git commit when the file is closed.
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		truncated := flag&os.O_TRUNC != 0
		return &davCommitFile{File: f, lib: dfs.lib, rel: rel, written: truncated, isNew: isNew}, nil
	}
	return f, nil
}

func (dfs *davFileSystem) RemoveAll(ctx context.Context, name string) error {
	if !dfs.allowed(name) {
		return os.ErrPermission
	}
	m := dfs.buildTitleMap()
	name = m.translateToID(name)
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

	m := dfs.buildTitleMap()
	oldTranslated := m.translateToID(oldName)
	oldRel := strings.TrimPrefix(oldTranslated, "/")
	newRel := strings.TrimPrefix(newName, "/")

	// Title-rename: old path maps to a canonical ID and new path is a root-level .md.
	// Implement as an H1 content update rather than a file-system rename so the
	// canonical ID is preserved.
	isOldCanonical := validFileRegex.MatchString(oldRel)
	isNewRootMD := !strings.ContainsRune(newRel, '/') && strings.HasSuffix(newRel, ".md")

	if isOldCanonical && isNewRootMD {
		newTitle := davSanitizeTitle(strings.TrimSuffix(newRel, ".md"))
		if newTitle == "" {
			newTitle = strings.TrimSuffix(newRel, ".md")
		}
		return dfs.updateNoteH1(oldRel, newTitle)
	}

	// Standard rename (non-canonical source, or subdirectory move).
	err := dfs.inner.Rename(ctx, oldTranslated, newName)
	if err == nil {
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
	m := dfs.buildTitleMap()
	translated := m.translateToID(name)
	info, err := dfs.inner.Stat(ctx, translated)
	if err != nil {
		return nil, err
	}
	// If we translated the name, wrap FileInfo to return the title-based name.
	if translated != name {
		rel := strings.TrimPrefix(name, "/")
		return &davTitleFileInfo{FileInfo: info, name: rel}, nil
	}
	return info, nil
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
// Path depth: at most 5 segments are allowed, supporting WebDAV clients that
// use a base directory (e.g. Obsidian Remotely Save with vault root "test"):
//
//	depth 1 — root files            e.g. "note.md"
//	depth 2 — one subfolder         e.g. "assets/image.png", "test/note.md"
//	depth 3 — two subfolders        e.g. "test/Daily Notes/note.md"
//	depth 4 — three subfolders      e.g. "test/Projects/Work/note.md"
//	depth 5 — four subfolders       e.g. "test/A/B/C/note.md"
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
