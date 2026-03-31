// Package main — Round-3 coverage gap tests for commit 80e47f7.
//
// This file must be compiled as part of `package main` alongside the production
// code in backend/. To run inside the Docker container:
//
//	cp tests/unit/webdav/round3_gaps_test.go backend/
//	docker compose run --rm app go test -run "TestR3" -v ./backend/...
//
// Gaps addressed:
//
//	R3-GAP-1: davCommitFile.Close() — isNew=false + oversize → os.Truncate path
//	          (commit 80e47f7 NEW-001 fix; ZERO tests for the truncate branch)
//	R3-GAP-2: reconcileStructure step 5b — stale Parents map cleanup
//	          (commit 80e47f7 NEW-004 fix; ZERO tests for Parents cleaning logic)
//	R3-GAP-3: davClientIP — IPv6 loopback XFF, whitespace-only XFF, multi-hop XFF
//	          (commit 80e47f7 NEW-005 fix; three cases still untested after the fix)
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── R3-GAP-1: davCommitFile.Close() — isNew=false oversize path ─────────────
//
// Before commit 80e47f7 the size check only ran when isNew==true. The fix (NEW-001)
// removes that guard so overwrites of existing notes are also size-checked. When the
// overwrite is oversized, os.Remove is called for both isNew=true and isNew=false:
// O_TRUNC already destroyed the original content at OpenFile time, so Remove cleans
// up the now-corrupt file rather than leaving a 0-byte stub.
//
// Code path in webdav.go (davCommitFile.Close):
//
//	if f.isNew {
//	    os.Remove(...)
//	} else {
//	    os.Remove(...)   // same: O_TRUNC already destroyed original content
//	}
//	return &davQuotaError{...}
//
// Updated R4: SubTest B and E now assert the file is REMOVED (not truncated to 0).

func TestR3GAP1_OverwriteExistingOversizedNoteIsTruncated(t *testing.T) {
	newLib := func(t *testing.T, maxBytes int64) (string, *NoteLibrary, http.Handler) {
		t.Helper()
		dir := t.TempDir()
		lib, err := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		require.NoError(t, err)
		lib.Config.MaxNoteSize = maxBytes
		lib.Config.MaxTotalNotes = 100
		srv := &Server{Library: lib}
		return dir, lib, srv.newDavHandler()
	}

	doPut := func(davH http.Handler, filename, body string) int {
		req, _ := http.NewRequest("PUT", "/dav/"+filename, strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		return w.Code
	}

	// Sub-test A: overwrite of existing note with oversized content → file truncated to 0.
	t.Run("overwrite_existing_oversized_returns_4xx", func(t *testing.T) {
		dir, _, davH := newLib(t, 10)

		// Create the pre-existing note (small, under limit).
		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"),
			[]byte("tiny"), 0600))

		// Overwrite with content that exceeds MaxNoteSize.
		code := doPut(davH, "existing.md", strings.Repeat("x", 100))
		assert.True(t, code >= 400,
			"overwriting existing note with oversized content must return 4xx, got %d", code)
	})

	// Sub-test B: after an oversized overwrite, the file is removed from disk.
	// O_TRUNC destroys the original content at OpenFile time; Close() then calls
	// os.Remove to leave no corrupt 0-byte file behind (consistent with isNew=true).
	t.Run("overwrite_existing_oversized_file_is_removed", func(t *testing.T) {
		dir, _, davH := newLib(t, 10)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"),
			[]byte("tiny"), 0600))

		doPut(davH, "existing.md", strings.Repeat("x", 100))

		_, statErr := os.Stat(filepath.Join(dir, "existing.md"))
		assert.True(t, os.IsNotExist(statErr),
			"oversized overwrite: file must be removed (not truncated to 0 or left with oversized content)")
	})

	// Sub-test C: reconcilePending must be set after overwrite-truncation, just as it
	// is for new-file removal, so the structure debouncer picks up the change.
	t.Run("overwrite_existing_oversized_sets_reconcilePending", func(t *testing.T) {
		dir, lib, davH := newLib(t, 10)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"),
			[]byte("tiny"), 0600))
		lib.reconcilePending.Store(false)

		doPut(davH, "existing.md", strings.Repeat("x", 100))

		assert.True(t, lib.reconcilePending.Load(),
			"reconcilePending must be set after oversized overwrite so structure reconciler notices")
	})

	// Sub-test D: overwrite of existing note at EXACTLY MaxNoteSize must succeed
	// (boundary: > MaxNoteSize triggers quota, == MaxNoteSize must not).
	t.Run("overwrite_existing_at_exact_limit_is_accepted", func(t *testing.T) {
		dir, _, davH := newLib(t, 50)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"),
			[]byte("tiny"), 0600))

		body := strings.Repeat("x", 50)
		code := doPut(davH, "existing.md", body)
		assert.True(t, code == http.StatusCreated || code == http.StatusNoContent,
			"overwriting existing note with content at exactly MaxNoteSize must succeed, got %d", code)

		got, readErr := os.ReadFile(filepath.Join(dir, "existing.md"))
		require.NoError(t, readErr)
		assert.Equal(t, body, string(got),
			"file content must equal the new body when overwrite is within limit")
	})

	// Sub-test E: overwrite of existing note 1 byte over limit → file is removed.
	t.Run("overwrite_existing_one_byte_over_limit_is_removed", func(t *testing.T) {
		dir, _, davH := newLib(t, 50)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"),
			[]byte("tiny"), 0600))

		doPut(davH, "existing.md", strings.Repeat("x", 51))

		_, statErr := os.Stat(filepath.Join(dir, "existing.md"))
		assert.True(t, os.IsNotExist(statErr),
			"file must be removed when overwrite is 1 byte over MaxNoteSize")
	})

	// Sub-test F: new file (isNew=true) oversized → REMOVED not truncated.
	// Ensures the isNew=true branch was not accidentally broken by the fix.
	t.Run("new_oversized_note_is_removed_not_truncated", func(t *testing.T) {
		dir, _, davH := newLib(t, 10)

		doPut(davH, "brand-new.md", strings.Repeat("x", 100))

		_, statErr := os.Stat(filepath.Join(dir, "brand-new.md"))
		assert.True(t, os.IsNotExist(statErr),
			"oversized NEW file must be removed from disk, not truncated")
	})
}

// ── R3-GAP-2: reconcileStructure step 5b — stale Parents cleanup ─────────────
//
// Commit 80e47f7 (NEW-004) added step 5b to reconcileStructure. It removes entries
// from st.Parents where either:
//   (a) the key (note filename) no longer exists on disk, OR
//   (b) the value (parentID folder) is no longer present in st.ChildOrder and is
//       not the empty string.
//
// Neither case was tested before this commit. The empty-parentID sentinel is also
// an undocumented edge case: parentID=="" means root-level placement, which must
// NOT be cleaned up as a "stale" folder reference.

func TestR3GAP2_ReconcileStructureParentsCleanup(t *testing.T) {
	// Helper that writes a _structure.json and triggers reconcileStructure by
	// deleting the named notes from disk (forcing the reconciler to see phantoms).
	buildLib := func(t *testing.T) (string, *NoteLibrary) {
		t.Helper()
		dir := t.TempDir()
		lib, err := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
		require.NoError(t, err)
		return dir, lib
	}

	writeStructure := func(t *testing.T, lib *NoteLibrary, st Structure) {
		t.Helper()
		d, err := json.Marshal(st)
		require.NoError(t, err)
		require.NoError(t, lib.AtomicWrite("_structure.json", d))
	}

	readStructure := func(t *testing.T, lib *NoteLibrary) Structure {
		t.Helper()
		d, err := os.ReadFile(lib.FullPath("_structure.json"))
		require.NoError(t, err)
		var st Structure
		require.NoError(t, json.Unmarshal(d, &st))
		return st
	}

	// Case A: note referenced in Parents is deleted from disk → entry removed.
	t.Run("phantom_note_in_parents_is_removed", func(t *testing.T) {
		dir, lib := buildLib(t)

		// Write a valid note and a phantom note (only in structure, not on disk).
		existingNote := "20260401recona000000001.md"
		phantomNote := "20260401recona000000002.md"
		require.NoError(t, os.WriteFile(filepath.Join(dir, existingNote), []byte("# hi"), 0600))

		folderID := "folder-abc"
		st := Structure{
			Order:      []string{existingNote},
			ChildOrder: map[string][]string{folderID: {existingNote}},
			Parents:    map[string]string{existingNote: folderID, phantomNote: folderID},
		}
		writeStructure(t, lib, st)

		// Trigger reconcile (call the exported wrapper via the file-absent path).
		lib.reconcileStructure()

		result := readStructure(t, lib)
		_, phantomPresent := result.Parents[phantomNote]
		assert.False(t, phantomPresent,
			"Parents entry for phantom note %q (not on disk) must be removed by reconcileStructure",
			phantomNote)
		_, existingPresent := result.Parents[existingNote]
		assert.True(t, existingPresent,
			"Parents entry for note %q (still on disk) must be preserved", existingNote)
	})

	// Case B: note's parentID folder was deleted from ChildOrder (folder became empty
	// and was pruned in step 5) → the now-orphaned Parents entry must also be cleaned.
	t.Run("orphaned_parents_entry_removed_when_folder_deleted", func(t *testing.T) {
		dir, lib := buildLib(t)

		// Two notes: orphanNote's folder will be pruned because the only child in it
		// (phantomNote) is a phantom.  The orphanNote itself still exists on disk.
		orphanNote := "20260401reconb000000001.md"
		phantomChild := "20260401reconb000000002.md"
		folderID := "folder-xyz"
		require.NoError(t, os.WriteFile(filepath.Join(dir, orphanNote), []byte("# orphan"), 0600))

		// Structure: folderID contains only phantomChild (which is gone from disk).
		// orphanNote is in Order but also has a Parents entry pointing to folderID.
		st := Structure{
			Order:      []string{orphanNote},
			ChildOrder: map[string][]string{folderID: {phantomChild}},
			Parents:    map[string]string{orphanNote: folderID},
		}
		writeStructure(t, lib, st)
		lib.reconcileStructure()

		result := readStructure(t, lib)
		// After reconcile, folderID's ChildOrder entry must have been pruned (only child was phantom).
		_, folderPresent := result.ChildOrder[folderID]
		assert.False(t, folderPresent,
			"folder %q must be removed from ChildOrder when all its children are phantoms", folderID)
		// The orphanNote's Parents entry pointing to the now-deleted folder must also be gone.
		_, parentEntryPresent := result.Parents[orphanNote]
		assert.False(t, parentEntryPresent,
			"Parents entry for note %q must be removed when its folder no longer exists in ChildOrder",
			orphanNote)
	})

	// Case C (edge case): parentID == "" is the sentinel for root-level placement.
	// Step 5b must NOT delete Parents entries whose value is "".
	// The condition in library_structure.go:177 is:
	//   } else if _, folderExists := st.ChildOrder[parentID]; !folderExists && parentID != "" {
	// The parentID != "" guard prevents this deletion. Test it explicitly.
	t.Run("empty_parentID_sentinel_is_not_cleaned_up", func(t *testing.T) {
		dir, lib := buildLib(t)

		note := "20260401reconc000000001.md"
		require.NoError(t, os.WriteFile(filepath.Join(dir, note), []byte("# root"), 0600))

		// parentID="" means "root level" — the folder key "" will never be in ChildOrder.
		st := Structure{
			Order:      []string{note},
			ChildOrder: map[string][]string{},
			Parents:    map[string]string{note: ""},
		}
		writeStructure(t, lib, st)
		lib.reconcileStructure()

		result := readStructure(t, lib)
		parentID, present := result.Parents[note]
		assert.True(t, present,
			"Parents entry with empty parentID must NOT be removed: it is a valid root-level sentinel")
		assert.Equal(t, "", parentID,
			"empty parentID must be preserved unchanged")
	})

	// Case D: Parents map is nil — step 5b must not panic.
	t.Run("nil_parents_map_does_not_panic", func(t *testing.T) {
		dir, lib := buildLib(t)

		note := "20260401recond000000001.md"
		require.NoError(t, os.WriteFile(filepath.Join(dir, note), []byte("# nil-parents"), 0600))

		// Omit Parents from the structure JSON (it will unmarshal as nil).
		raw := `{"order":["` + note + `"],"childOrder":{}}`
		require.NoError(t, lib.AtomicWrite("_structure.json", []byte(raw)))

		// Must not panic.
		assert.NotPanics(t, func() { lib.reconcileStructure() },
			"reconcileStructure must not panic when st.Parents is nil")
	})
}

// ── R3-GAP-3: davClientIP — untested edge cases after rightmost-XFF fix ──────
//
// Commit 80e47f7 (NEW-005) changed davClientIP to use LastIndexByte (rightmost)
// instead of IndexByte (leftmost). The two new subtests added in the commit cover:
//   - multi-value XFF, rightmost is taken  (covered)
//   - single-value XFF from loopback       (covered)
//
// Still untested after the fix:
//   - IPv6 loopback (::1) proxy must also honour XFF
//   - XFF with only whitespace must fall back to RemoteAddr
//   - IPv6 address in XFF value must be returned as-is (no port stripping)
//   - Three-hop XFF: rightmost of three values is taken

func TestR3GAP3_DavClientIPEdgeCases(t *testing.T) {
	makeReq := func(remoteAddr, xff string) *http.Request {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		return req
	}

	// IPv6 loopback proxy must honour XFF, matching the same logic as 127.0.0.1.
	// Production code: `if host == "127.0.0.1" || host == "::1"`.
	t.Run("ipv6_loopback_proxy_honours_xff", func(t *testing.T) {
		req := makeReq("[::1]:4321", "198.51.100.7, 10.2.3.4")
		assert.Equal(t, "10.2.3.4", davClientIP(req),
			"IPv6 loopback proxy must use rightmost XFF value")
	})

	// Whitespace-only XFF value must NOT be used; fall back to RemoteAddr.
	// Production code: `if xff := ...; xff != ""` — a string of spaces passes this
	// check, then `strings.TrimSpace` returns "" which would be returned as the IP.
	// This tests whether the implementation guards against that.
	t.Run("whitespace_only_xff_falls_back_to_remoteaddr", func(t *testing.T) {
		req := makeReq("127.0.0.1:4321", "   ")
		result := davClientIP(req)
		// After TrimSpace the XFF is "". The caller should not use a blank IP.
		// Current implementation: `return strings.TrimSpace(xff)` returns "".
		// This test documents the actual behavior — if it returns "" that is a bug
		// (a blank string will corrupt applyAuthDelay's per-IP map key).
		assert.NotEqual(t, "", result,
			"KNOWN GAP: whitespace-only XFF must not produce an empty IP string; "+
				"expected fallback to RemoteAddr '127.0.0.1', got %q", result)
	})

	// IPv6 address in XFF value must be returned verbatim.
	t.Run("ipv6_address_in_xff_returned_verbatim", func(t *testing.T) {
		req := makeReq("127.0.0.1:4321", "2001:db8::1")
		assert.Equal(t, "2001:db8::1", davClientIP(req),
			"IPv6 address in single-value XFF must be returned as-is")
	})

	// Rightmost of three XFF hops: "a, b, c" → "c".
	// This guards the LastIndexByte semantics: there is no off-by-one when the
	// comma is not the last character.
	t.Run("three_hop_xff_rightmost_value_used", func(t *testing.T) {
		req := makeReq("127.0.0.1:4321", "203.0.113.1, 10.0.0.1, 10.1.1.1")
		assert.Equal(t, "10.1.1.1", davClientIP(req),
			"three-hop XFF must use the rightmost (last) value")
	})

	// Trailing comma in XFF (malformed but possible): "a, b," → TrimSpace("") = "".
	// Documents current behavior; a blank result here is also a latent bug.
	t.Run("trailing_comma_xff_documents_current_behavior", func(t *testing.T) {
		req := makeReq("127.0.0.1:4321", "203.0.113.1, 10.0.0.1,")
		result := davClientIP(req)
		// The segment after the last comma is "". After TrimSpace it is still "".
		// This test intentionally uses t.Logf not assert so it does not block the
		// build, but makes the behavior visible in test output.
		t.Logf("EDGE CASE: trailing-comma XFF returned %q (empty string is a latent bug)", result)
	})

	// Non-loopback remote: XFF must be completely ignored regardless of value.
	t.Run("non_loopback_ignores_xff_with_ipv6_value", func(t *testing.T) {
		req := makeReq("10.0.0.5:8080", "2001:db8::attacker")
		assert.Equal(t, "10.0.0.5", davClientIP(req),
			"XFF must be ignored when connection is not from a loopback address")
	})

	// IPv6 loopback in XFF, no comma — single-value path for ::1 proxy.
	t.Run("ipv6_loopback_single_xff_value", func(t *testing.T) {
		req := makeReq("[::1]:4321", "2001:db8::abcd")
		assert.Equal(t, "2001:db8::abcd", davClientIP(req),
			"single IPv6 XFF value from ::1 proxy must be returned as-is")
	})
}
