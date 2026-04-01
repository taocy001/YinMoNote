// Package main — WebDAV title-virtualisation tests.
//
// Tests for the title-based virtual filesystem layer in webdav.go.
// Copy or symlink this file into backend/ before running `go test ./...`.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// writeCanonicalNote creates a canonical-ID note file in dir with the given content.
func writeCanonicalNote(t *testing.T, dir, id, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, id), []byte(content), 0600))
}

// davTitleServer sets up a full test WebDAV server backed by a temp NoteLibrary.
// Returns the server and library; no WebDAV token is set (keyless mode).
func davTitleServer(t *testing.T) (*httptest.Server, *NoteLibrary, string) {
	t.Helper()
	dir := t.TempDir()
	lib, err := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, err)
	s := &Server{Library: lib}
	handler := s.newDavHandler()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, lib, dir
}

// propfindNames returns all <D:displayname> values from a PROPFIND Depth:1 response.
func propfindNames(t *testing.T, ts *httptest.Server, path string) []string {
	t.Helper()
	req, _ := http.NewRequest("PROPFIND", ts.URL+path, nil)
	req.Header.Set("Depth", "1")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 207, resp.StatusCode)

	var bodyBytes strings.Builder
	_, err = io.Copy(&bodyBytes, resp.Body)
	require.NoError(t, err)
	body := bodyBytes.String()

	var names []string
	for _, chunk := range strings.Split(body, "<D:href>") {
		if !strings.Contains(chunk, "</D:href>") {
			continue
		}
		raw := chunk[:strings.Index(chunk, "</D:href>")]
		raw = strings.TrimPrefix(raw, "/dav/")
		raw = strings.TrimSuffix(raw, "/")
		if raw == "" {
			continue
		}
		// Hrefs in WebDAV XML responses are URL-encoded; decode for human-readable comparison.
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			decoded = raw
		}
		names = append(names, decoded)
	}
	return names
}

// ── davSanitizeTitle ──────────────────────────────────────────────────────────

func TestDavSanitizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "Hello World"},
		{"数据中心网络", "数据中心网络"},
		{"Note/With/Slashes", "Note_With_Slashes"},
		{`Col:on "Quot" <html>`, "Col_on _Quot_ _html_"},
		{"  leading spaces  ", "leading spaces"},
		{"", ""},
		{strings.Repeat("a", 250), strings.Repeat("a", 200)},
	}
	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 30)], func(t *testing.T) {
			assert.Equal(t, tt.want, davSanitizeTitle(tt.input))
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── extractNoteTitle fixes (C-008) ────────────────────────────────────────────

func TestExtractNoteTitle_MultiHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")

	t.Run("H1", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("# My Title\nContent"), 0600))
		assert.Equal(t, "My Title", extractNoteTitle(path))
	})
	t.Run("H2", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("## Section\nContent"), 0600))
		assert.Equal(t, "Section", extractNoteTitle(path))
	})
	t.Run("H3", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("### Deep\nContent"), 0600))
		assert.Equal(t, "Deep", extractNoteTitle(path))
	})
	t.Run("no heading", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("Plain text\nMore"), 0600))
		assert.Equal(t, "Plain text", extractNoteTitle(path))
	})
}

// ── PROPFIND listing shows title names ────────────────────────────────────────

func TestDavTitle_PropfindShowsTitles(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	// Create two canonical-ID notes.
	id1 := "20260401aaaaaaaaaaaaaaa1.md"
	id2 := "20260401aaaaaaaaaaaaaaa2.md"
	writeCanonicalNote(t, lib.DataDir, id1, "# 数据中心网络\nContent A")
	writeCanonicalNote(t, lib.DataDir, id2, "# SONiC编译\nContent B")

	names := propfindNames(t, ts, "/dav/")

	assert.Contains(t, names, "数据中心网络.md", "title-based name should appear")
	assert.Contains(t, names, "SONiC编译.md", "title-based name should appear")
	assert.NotContains(t, names, id1, "raw ID should not appear")
	assert.NotContains(t, names, id2, "raw ID should not appear")
}

// ── GET by title returns correct content ─────────────────────────────────────

func TestDavTitle_GetByTitle(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "# My Note\nHello content")

	resp, err := http.Get(ts.URL + "/dav/My%20Note.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var buf strings.Builder
	_, _ = io.Copy(&buf, resp.Body)
	assert.Contains(t, buf.String(), "Hello content")
}

// ── PUT by title overwrites the canonical-ID file ────────────────────────────

func TestDavTitle_PutByTitleOverwrites(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "# My Note\nOriginal")

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/dav/My%20Note.md",
		strings.NewReader("# My Note\nUpdated content"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent)

	got, err := os.ReadFile(filepath.Join(lib.DataDir, id))
	require.NoError(t, err)
	assert.Contains(t, string(got), "Updated content")
}

// ── DELETE by title removes the canonical-ID file ────────────────────────────

func TestDavTitle_DeleteByTitle(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "# Delete Me\nContent")

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/dav/Delete%20Me.md", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, statErr := os.Stat(filepath.Join(lib.DataDir, id))
	assert.True(t, os.IsNotExist(statErr), "canonical ID file should be deleted")
}

// ── Duplicate title deduplication ────────────────────────────────────────────

func TestDavTitle_DuplicateTitleDedup(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	// Two notes with the same title but different IDs.
	// ListNotes sorts newest-first by modtime; write id2 first so id1 is newer.
	id2 := "20260401aaaaaaaaaaaaaaa2.md"
	id1 := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id2, "# Same Title\nOlder")
	writeCanonicalNote(t, lib.DataDir, id1, "# Same Title\nNewer")

	names := propfindNames(t, ts, "/dav/")
	assert.Contains(t, names, "Same Title.md")
	assert.Contains(t, names, "Same Title (2).md")
}

// ── Non-canonical files pass through unchanged ───────────────────────────────

func TestDavTitle_NonCanonicalPassThrough(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	// Non-canonical file written directly to disk (e.g. by a previous Obsidian sync).
	require.NoError(t, os.WriteFile(
		filepath.Join(lib.DataDir, "My Obsidian Note.md"),
		[]byte("# My Obsidian Note\nContent"), 0600))

	names := propfindNames(t, ts, "/dav/")
	assert.Contains(t, names, "My Obsidian Note.md",
		"non-canonical note should appear with its own filename")
}

// ── MOVE updates H1 content, keeps canonical ID ──────────────────────────────

func TestDavTitle_RenameUpdatesH1(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "# Old Title\nBody text")

	req, _ := http.NewRequest("MOVE", ts.URL+"/dav/Old%20Title.md", nil)
	req.Header.Set("Destination", ts.URL+"/dav/New%20Title.md")
	req.Header.Set("Overwrite", "T")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	// 201 Created or 204 No Content are both valid MOVE success codes.
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent,
		"MOVE should succeed, got %d", resp.StatusCode)

	// Canonical ID file must still exist.
	got, err := os.ReadFile(filepath.Join(lib.DataDir, id))
	require.NoError(t, err)
	content := string(got)
	assert.Contains(t, content, "# New Title", "H1 should be updated")
	assert.Contains(t, content, "Body text", "body should be preserved")
	assert.NotContains(t, content, "# Old Title")
}

// ── _structure.json never appears in listing ─────────────────────────────────

func TestDavTitle_StructureJsonHidden(t *testing.T) {
	ts, _, _ := davTitleServer(t)
	names := propfindNames(t, ts, "/dav/")
	for _, n := range names {
		assert.NotContains(t, n, "_structure.json")
	}
}

// ── Stat by title returns FileInfo with title name ────────────────────────────

func TestDavTitle_StatByTitle(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "# Stat Me\nContent")

	// PROPFIND Depth:0 on the title path acts as a Stat.
	req, _ := http.NewRequest("PROPFIND", ts.URL+"/dav/Stat%20Me.md", nil)
	req.Header.Set("Depth", "0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 207, resp.StatusCode)

	var buf strings.Builder
	_, _ = io.Copy(&buf, resp.Body)
	assert.Contains(t, buf.String(), "Stat%20Me.md")
}

// ── buildTitleMap degenerate cases ───────────────────────────────────────────

func TestDavTitle_EmptyLib(t *testing.T) {
	ts, _, _ := davTitleServer(t)
	names := propfindNames(t, ts, "/dav/")
	// Should return at least the root entry with no error.
	_ = names
}

func TestDavTitle_EmptyTitleFallsBackToID(t *testing.T) {
	_, lib, dir := davTitleServer(t)
	_ = dir

	// A note with no content: extractNoteTitle returns "".
	id := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, id, "")

	m := &davFileSystem{
		inner: webdav.Dir(lib.DataDir),
		lib:   lib,
	}
	nm := m.buildTitleMap()
	// Should fall back to the ID stem.
	stem := strings.TrimSuffix(id, ".md")
	_, found := nm.idToTitle[id]
	assert.True(t, found, "empty-title note should still have an entry")
	assert.Equal(t, stem+".md", nm.idToTitle[id])
}

// ── Quota: title-PUT to non-existent note triggers quota check ────────────────

func TestDavTitle_PutNewNoteRespectsQuota(t *testing.T) {
	ts, lib, _ := davTitleServer(t)

	lib.mu.Lock()
	lib.Config.MaxTotalNotes = 1
	lib.mu.Unlock()

	// Pre-fill the quota.
	existing := "20260401aaaaaaaaaaaaaaa1.md"
	writeCanonicalNote(t, lib.DataDir, existing, "# Existing\nContent")

	// Try to PUT a new note with a title name (not in the mapping → new file).
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/dav/Brand%20New.md",
		strings.NewReader("# Brand New\nContent"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.GreaterOrEqual(t, resp.StatusCode, 400,
		"PUT of a new note should be rejected when quota is full, got %d", resp.StatusCode)

	// Verify no "Brand New.md" or any extra .md was created.
	entries, _ := os.ReadDir(lib.DataDir)
	mdCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") && e.Name() != "_structure.json" {
			mdCount++
		}
	}
	assert.Equal(t, 1, mdCount, "only the pre-existing note should exist")
}

// ── Large title is truncated to 200 bytes ────────────────────────────────────

func TestDavTitle_LongTitleTruncated(t *testing.T) {
	_, lib, _ := davTitleServer(t)

	id := "20260401aaaaaaaaaaaaaaa1.md"
	longTitle := strings.Repeat("A", 300)
	writeCanonicalNote(t, lib.DataDir, id, "# "+longTitle+"\nBody")

	dfs := &davFileSystem{inner: webdav.Dir(lib.DataDir), lib: lib}
	nm := dfs.buildTitleMap()

	davName, ok := nm.idToTitle[id]
	require.True(t, ok)
	stem := strings.TrimSuffix(davName, ".md")
	assert.LessOrEqual(t, len(stem), 200,
		"sanitized title should not exceed 200 bytes")
}

// ── buildTitleMap: verify mapping keys are correct ────────────────────────────

func TestDavTitle_BuildTitleMap(t *testing.T) {
	lib, err := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, err)

	id1 := "20260401aaaaaaaaaaaaaaa1.md"
	id2 := "20260401aaaaaaaaaaaaaaa2.md"
	writeCanonicalNote(t, lib.DataDir, id1, "# First Note\nBody")
	writeCanonicalNote(t, lib.DataDir, id2, "# Second Note\nBody")

	dfs := &davFileSystem{inner: webdav.Dir(lib.DataDir), lib: lib}
	nm := dfs.buildTitleMap()

	assert.Equal(t, id1, nm.titleToID["First Note.md"])
	assert.Equal(t, id2, nm.titleToID["Second Note.md"])
	assert.Equal(t, "First Note.md", nm.idToTitle[id1])
	assert.Equal(t, "Second Note.md", nm.idToTitle[id2])
	fmt.Println("titleToID:", nm.titleToID)
}
