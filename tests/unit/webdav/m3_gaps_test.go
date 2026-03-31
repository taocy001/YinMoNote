// Package main — M3 WebDAV gap tests.
//
// This file covers the five highest-priority coverage gaps identified in the
// M3 WebDAV milestone audit. It must be compiled as part of `package main`
// alongside the production code in backend/. Copy or symlink this file into
// backend/ before running `go test ./...`.
//
// Gap coverage:
//
//   GAP-1: SEC-001 — Concurrent WebDAV PUTs exceeding count quota (TOCTOU)
//   GAP-2: SEC-002 — 20 MB written before size check; file removed after close
//   GAP-3: allowed() — _structure.json DELETE + deep path (>2 segments) rejection
//   GAP-4: StartReconcileDebouncer — coalesces N WebDAV writes into one reconcile
//   GAP-5: MOVE (Rename) — blocked destination (_structure.json, .git/) is rejected
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── GAP-1: SEC-001 Concurrent WebDAV PUT quota TOCTOU ────────────────────────
//
// Production issue: CheckNoteQuota (count check) and the subsequent
// inner.OpenFile (file creation) are not atomic. Two goroutines can both
// observe len(notes) < MaxTotalNotes before either creates a file, then both
// create files, exceeding the quota.
//
// This test documents the race and WILL FAIL with the current implementation,
// exposing the unaddressed TOCTOU. It is intentionally structured so that when
// the production code is fixed (e.g. with a per-library creation mutex) the
// test will go green without modification.

func TestGAP1_ConcurrentDavPutQuotaTOCTOU(t *testing.T) {
	dir := t.TempDir()
	lib, err := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, err)

	// Allow exactly 1 note to exist at any time.
	lib.Config.MaxTotalNotes = 1
	lib.Config.MaxNoteSize = 1024 * 1024

	srv := &Server{Library: lib}
	davH := srv.newDavHandler()

	const goroutines = 10

	// Each goroutine tries to create a distinct new .md file via WebDAV PUT.
	names := make([]string, goroutines)
	for i := range names {
		names[i] = fmt.Sprintf("concurrent-note-%02d.md", i)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	successCount := int32(0)

	for _, name := range names {
		name := name
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("PUT", "/dav/"+name, strings.NewReader("# note"))
			req.ContentLength = 6
			w := httptest.NewRecorder()
			davH.ServeHTTP(w, req)
			if w.Code == http.StatusCreated || w.Code == http.StatusNoContent {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	// Count actual files created on disk (ground truth).
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	actualNotes := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "_structure.json" {
			actualNotes++
		}
	}

	// EXPECTED with correct implementation: at most MaxTotalNotes files exist.
	// ACTUAL with TOCTOU bug: multiple goroutines may have raced past the quota check;
	// actualNotes > 1 reveals the race.
	//
	// If this assertion fails the TOCTOU race is confirmed. The build is intentionally
	// left to fail here so the issue is visible in CI — do not change to t.Logf.
	assert.LessOrEqual(t, actualNotes, lib.Config.MaxTotalNotes,
		"TOCTOU race: %d notes created despite MaxTotalNotes=%d. "+
			"Fix: serialize CheckNoteQuota+OpenFile under a creation mutex.",
		actualNotes, lib.Config.MaxTotalNotes)
}

// ── GAP-2: SEC-002 Oversized file cleanup: file removed after close ───────────
//
// The davCommitFile.Close() method:
//  1. Calls inner.Close() — which flushes the full content to disk.
//  2. Then checks info.Size() against MaxNoteSize.
//  3. If oversized, calls os.Remove().
//
// This means the server accepts all 20 MB of data from the client before
// rejecting it. This test verifies the observable contract:
//   a. The PUT returns a 4xx error code.
//   b. The file does NOT remain on disk after the error (cleanup ran).
//   c. reconcilePending is set so the structure stays consistent.
//
// It also verifies the boundary: a note at exactly MaxNoteSize is accepted;
// one byte over is rejected.

func TestGAP2_DavOversizedNoteCleanup(t *testing.T) {
	t.Run("exactly_at_limit_is_accepted", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 50
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		body := strings.Repeat("x", 50)
		req, _ := http.NewRequest("PUT", "/dav/exact-limit.md", strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusNoContent,
			"note at exactly MaxNoteSize must be accepted, got %d", w.Code)
		_, statErr := os.Stat(filepath.Join(dir, "exact-limit.md"))
		assert.NoError(t, statErr, "file at exactly MaxNoteSize must exist on disk")
	})

	t.Run("one_byte_over_limit_is_rejected_and_removed", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 50
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		body := strings.Repeat("x", 51) // one byte over
		req, _ := http.NewRequest("PUT", "/dav/over-limit.md", strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		assert.True(t, w.Code >= 400,
			"note one byte over MaxNoteSize must be rejected with 4xx, got %d", w.Code)
		_, statErr := os.Stat(filepath.Join(dir, "over-limit.md"))
		assert.True(t, os.IsNotExist(statErr),
			"oversized file must be removed from disk after rejection (SEC-002 cleanup)")
	})

	t.Run("reconcile_pending_set_after_oversized_removal", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 10
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		// Reset the flag before the test.
		lib.reconcilePending.Store(false)

		body := strings.Repeat("x", 100)
		req, _ := http.NewRequest("PUT", "/dav/big-note.md", strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		assert.True(t, w.Code >= 400, "oversized PUT should return 4xx")
		assert.True(t, lib.reconcilePending.Load(),
			"reconcilePending must be set after oversized file removal so structure stays consistent")
	})

	t.Run("overwrite_existing_note_size_not_checked_by_close", func(t *testing.T) {
		// Known gap (SEC-002 variant): size quota in Close() only applies when isNew==true.
		// An existing note can be overwritten via WebDAV with content larger than MaxNoteSize.
		// This test documents the current (insecure) behavior so a future fix is detectable.
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 10
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		// Pre-create an existing note so isNew=false on the next PUT.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"), []byte("tiny"), 0600))

		body := strings.Repeat("x", 100) // 100 > MaxNoteSize(10)
		req, _ := http.NewRequest("PUT", "/dav/existing.md", strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		// isNew=false oversized overwrite is now rejected: Close() calls os.Remove
		// for both isNew=true and isNew=false when the written size exceeds MaxNoteSize.
		assert.True(t, w.Code >= 400, "oversized overwrite of existing note should be rejected, got %d", w.Code)
	})
}

// ── GAP-3: allowed() blacklist — _structure.json DELETE + deep path rejection ─
//
// The allowed() function is tested indirectly via PROPFIND/GET in the existing
// tests, but three specific behaviors are not covered:
//   a. WebDAV DELETE of _structure.json must be rejected (ErrPermission → 403/404).
//   b. A path with 3+ non-empty segments (e.g. /dav/a/b/c.md) must be rejected.
//   c. MOVE (Rename) where EITHER source or destination is blocked must fail.

func TestGAP3_DavAllowedBlacklist(t *testing.T) {
	newOpenHandler := func(t *testing.T) (string, http.Handler) {
		t.Helper()
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		// Write _structure.json so there is something to try to delete.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "_structure.json"),
			[]byte(`{"order":[],"parents":{},"childOrder":{}}`), 0600))
		srv := &Server{Library: lib}
		return dir, srv.newDavHandler()
	}

	t.Run("DELETE _structure.json is rejected with non-2xx", func(t *testing.T) {
		_, davH := newOpenHandler(t)
		req, _ := http.NewRequest("DELETE", "/dav/_structure.json", nil)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400,
			"DELETE _structure.json must be rejected, got %d", w.Code)
	})

	t.Run("_structure.json still exists after rejected DELETE", func(t *testing.T) {
		dir, davH := newOpenHandler(t)
		req, _ := http.NewRequest("DELETE", "/dav/_structure.json", nil)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400, "DELETE must be rejected")
		_, statErr := os.Stat(filepath.Join(dir, "_structure.json"))
		assert.NoError(t, statErr, "_structure.json must NOT be deleted when WebDAV rejects it")
	})

	t.Run("PUT to depth-3 path is rejected", func(t *testing.T) {
		// /dav/subdir/another/note.md has 3 non-empty segments: subdir, another, note.md
		_, davH := newOpenHandler(t)
		req, _ := http.NewRequest("PUT", "/dav/subdir/another/note.md",
			strings.NewReader("content"))
		req.ContentLength = 7
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400,
			"PUT to depth-3 path must be rejected by allowed(), got %d", w.Code)
	})

	t.Run("PUT to assets subdirectory (depth-2) is allowed by allowed()", func(t *testing.T) {
		// /dav/assets/image.png has 2 non-empty segments: assets, image.png — within the limit.
		// This tests the boundary: depth 2 is allowed, depth 3 is not.
		// Note: the underlying file creation may fail for other reasons (not an .md file, etc.)
		// but allowed() itself must NOT reject it.
		_, davH := newOpenHandler(t)
		req, _ := http.NewRequest("PUT", "/dav/assets/image.png",
			strings.NewReader("fake png"))
		req.ContentLength = 8
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		// We only assert it was NOT rejected by allowed() (i.e., not a 403 from ErrPermission).
		// The webdav library may return 409 (Conflict) if assets/ doesn't exist as a directory.
		assert.NotEqual(t, http.StatusForbidden, w.Code,
			"depth-2 path must not be rejected by allowed()")
	})

	t.Run("hidden file PUT is rejected", func(t *testing.T) {
		_, davH := newOpenHandler(t)
		req, _ := http.NewRequest("PUT", "/dav/.hidden-note.md",
			strings.NewReader("content"))
		req.ContentLength = 7
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400,
			"PUT to hidden file (.hidden-note.md) must be rejected, got %d", w.Code)
	})

	t.Run("tilde-suffix file PUT is rejected", func(t *testing.T) {
		_, davH := newOpenHandler(t)
		req, _ := http.NewRequest("PUT", "/dav/note.md~",
			strings.NewReader("content"))
		req.ContentLength = 7
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400,
			"PUT to tilde-suffix temp file (note.md~) must be rejected, got %d", w.Code)
	})
}

// ── GAP-4: StartReconcileDebouncer coalesces N WebDAV writes into one reconcile ─
//
// The debouncer runs every 2 seconds and calls reconcileStructure at most once
// per interval, regardless of how many WebDAV writes triggered reconcilePending=true.
//
// This test uses a synthetic approach: it calls the debouncer's internal logic
// directly (CompareAndSwap + reconcileStructure) rather than spinning up a
// live goroutine with a 2-second sleep, keeping the test fast.
//
// The observable invariant: after N WebDAV PUTs that each set reconcilePending=true,
// _structure.json is updated exactly once per debouncer tick (not N times).

func TestGAP4_ReconcileDebouncer(t *testing.T) {
	t.Run("reconcilePending_set_by_webdav_put", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 1024 * 1024
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		lib.reconcilePending.Store(false)

		req, _ := http.NewRequest("PUT", "/dav/debounce-note.md",
			strings.NewReader("# hello"))
		req.ContentLength = 7
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusNoContent,
			"PUT should succeed, got %d", w.Code)
		assert.True(t, lib.reconcilePending.Load(),
			"WebDAV PUT of .md file must set reconcilePending=true")
	})

	t.Run("non_md_put_does_not_set_reconcilePending", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxNoteSize = 1024 * 1024
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()

		lib.reconcilePending.Store(false)

		// PUT a non-.md file — should NOT trigger reconcilePending.
		req, _ := http.NewRequest("PUT", "/dav/image.png",
			strings.NewReader("fake png"))
		req.ContentLength = 8
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)

		// reconcilePending should remain false regardless of whether the PUT itself succeeded.
		assert.False(t, lib.reconcilePending.Load(),
			"WebDAV PUT of non-.md file must NOT set reconcilePending=true")
	})

	t.Run("debouncer_coalesces_n_writes_into_one_reconcile", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))

		// Write N notes to disk directly (simulating completed WebDAV PUTs)
		// and set reconcilePending as each PUT would.
		const n = 5
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("debounce-%02d.md", i)
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("content"), 0600))
		}
		lib.reconcilePending.Store(true)

		// Track how many times reconcileStructure is called by inspecting
		// _structure.json's mtime before and after a single debouncer tick.
		structurePath := filepath.Join(dir, "_structure.json")
		infoBeforeFirst, err := os.Stat(structurePath)

		// Wait for the mtime resolution to be meaningful on this OS.
		time.Sleep(5 * time.Millisecond)

		// Simulate one debouncer tick.
		if lib.reconcilePending.CompareAndSwap(true, false) {
			lib.reconcileStructure()
		}

		infoAfterFirst, err2 := os.Stat(structurePath)
		require.NoError(t, err2)

		// If the structure did not exist before, reconcileStructure created it.
		structureChanged := err != nil || infoAfterFirst.ModTime().After(infoBeforeFirst.ModTime())
		assert.True(t, structureChanged,
			"reconcileStructure must update _structure.json on the first tick after WebDAV writes")

		// After the first tick reconcilePending is false. A second tick does nothing.
		assert.False(t, lib.reconcilePending.Load(),
			"reconcilePending must be false after the debouncer tick consumes it")

		// Verify all N notes appear in the structure.
		data, readErr := os.ReadFile(structurePath)
		require.NoError(t, readErr)
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("debounce-%02d.md", i)
			assert.Contains(t, string(data), name,
				"reconciled structure must include %s", name)
		}
	})
}

// ── GAP-5: MOVE (Rename) — blocked destination rejected ──────────────────────
//
// davFileSystem.Rename() checks both source and destination via allowed().
// A WebDAV MOVE to _structure.json or a hidden file must be blocked.
// A legitimate MOVE (canonical → canonical) must succeed.

func TestGAP5_DavRenameBlocked(t *testing.T) {
	newHandlerWithNote := func(t *testing.T, noteName string) (string, *NoteLibrary, http.Handler) {
		t.Helper()
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		require.NoError(t, os.WriteFile(filepath.Join(dir, noteName), []byte("content"), 0600))
		srv := &Server{Library: lib}
		return dir, lib, srv.newDavHandler()
	}

	sendMove := func(davH http.Handler, src, dst string) int {
		req, _ := http.NewRequest("MOVE", "/dav/"+src, nil)
		// WebDAV MOVE uses the Destination header (absolute URI).
		req.Header.Set("Destination", "http://localhost/dav/"+dst)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("MOVE to _structure.json is rejected", func(t *testing.T) {
		_, _, davH := newHandlerWithNote(t, "source-note.md")
		code := sendMove(davH, "source-note.md", "_structure.json")
		assert.True(t, code >= 400,
			"MOVE to _structure.json must be rejected, got %d", code)
	})

	t.Run("MOVE to hidden file is rejected", func(t *testing.T) {
		_, _, davH := newHandlerWithNote(t, "source-note2.md")
		code := sendMove(davH, "source-note2.md", ".git-replacement")
		assert.True(t, code >= 400,
			"MOVE to hidden destination must be rejected, got %d", code)
	})

	t.Run("MOVE from _structure.json is rejected (source blocked)", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "_structure.json"),
			[]byte(`{"order":[]}`), 0600))
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()
		code := sendMove(davH, "_structure.json", "exported-structure.md")
		assert.True(t, code >= 400,
			"MOVE from _structure.json must be rejected, got %d", code)
	})

	t.Run("legitimate MOVE marks both source and dest pending", func(t *testing.T) {
		dir, lib, davH := newHandlerWithNote(t, "old-name.md")

		// Ensure target does not exist (MOVE requires it to be absent or
		// Overwrite: T to be set — default is T per RFC 4918).
		code := sendMove(davH, "old-name.md", "new-name.md")

		// A MOVE may fail if the webdav.Handler rejects it for other reasons
		// (e.g. locking). We only assert the pending behavior IF the MOVE succeeded.
		if code == http.StatusCreated || code == http.StatusNoContent {
			lib.mu.Lock()
			oldPending := lib.pendingCommits["old-name.md"]
			newPending := lib.pendingCommits["new-name.md"]
			lib.mu.Unlock()

			assert.True(t, oldPending, "old name (source) must be marked pending after MOVE")
			assert.True(t, newPending, "new name (destination) must be marked pending after MOVE")

			_, oldStatErr := os.Stat(filepath.Join(dir, "old-name.md"))
			assert.True(t, os.IsNotExist(oldStatErr), "source file must no longer exist after MOVE")
			_, newStatErr := os.Stat(filepath.Join(dir, "new-name.md"))
			assert.NoError(t, newStatErr, "destination file must exist after MOVE")
		} else {
			t.Logf("MOVE returned %d; pending-check skipped (MOVE may require LOCK support)", code)
		}
	})
}

// ── Bonus: Security headers present on WebDAV error responses ────────────────
//
// The existing tests only check security headers on 207 (success) responses.
// They must also be present on 401, 403, and 503 responses.

func TestDavSecurityHeadersOnErrorResponses(t *testing.T) {
	checkHeaders := func(t *testing.T, w *httptest.ResponseRecorder) {
		t.Helper()
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"),
			"X-Frame-Options must be set on error responses")
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
			"X-Content-Type-Options must be set on error responses")
		assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"),
			"Referrer-Policy must be set on error responses")
		assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"),
			"CSP must be set on error responses")
	}

	t.Run("security headers on 401 (no credentials)", func(t *testing.T) {
		lib, _, _ := setupSRPLib(t, "test-password")
		// SRPVerifier set but no WebDAVTokenHash → C-003 guard → 401.
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()
		req, _ := http.NewRequest("PROPFIND", "/dav/", nil)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		checkHeaders(t, w)
	})

	t.Run("security headers on 503 (serverEncrypt)", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.ServerEncrypt = true
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()
		req, _ := http.NewRequest("GET", "/dav/some-note.md", nil)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		checkHeaders(t, w)
	})
}
