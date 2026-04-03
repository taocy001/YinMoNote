package main

import (
	"strings"
	"time"
)

// ── Structure-update helpers ──────────────────────────────────────────────────

// removeFromStringSlice returns s with all occurrences of item removed.
// Modifies the underlying slice header only (slice is not rearranged in memory).
func removeFromStringSlice(s []string, item string) []string {
	out := s[:0]
	for _, v := range s {
		if v != item {
			out = append(out, v)
		}
	}
	return out
}

// removeNoteFromStructure removes noteID from all structure fields where it
// appears: Order, ChildOrder (as a child), Parents (as a key), Titles, Tags.
// It is deliberately defensive: no error is returned if noteID is not found.
func removeNoteFromStructure(st *Structure, noteID string) {
	st.Order = removeFromStringSlice(st.Order, noteID)
	if st.Parents != nil {
		parentID := st.Parents[noteID]
		if parentID != "" && st.ChildOrder != nil {
			st.ChildOrder[parentID] = removeFromStringSlice(st.ChildOrder[parentID], noteID)
			if len(st.ChildOrder[parentID]) == 0 {
				delete(st.ChildOrder, parentID)
			}
		}
		delete(st.Parents, noteID)
	}
	if st.Titles != nil {
		delete(st.Titles, noteID)
	}
	if st.Tags != nil {
		delete(st.Tags, noteID)
	}
	if st.CommitLabels != nil {
		delete(st.CommitLabels, noteID)
	}
}

// collectSubtreeNoteIDs returns all canonical note IDs (ending in .md) in the
// subtree rooted at rootID, including rootID itself if it is a note.
// Uses a visited set to guard against cycles (which reconcileStructure prevents
// but which may transiently appear during concurrent structure updates).
func collectSubtreeNoteIDs(rootID string, st *Structure) []string {
	var ids []string
	visited := make(map[string]bool)
	var walk func(string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		if strings.HasSuffix(id, ".md") {
			ids = append(ids, id)
		}
		for _, child := range st.ChildOrder[id] {
			walk(child)
		}
	}
	walk(rootID)
	return ids
}

// newCanonicalNoteID generates a fresh canonical note filename using the
// current date and a cryptographically random 16-character suffix.
func newCanonicalNoteID() string {
	return time.Now().Format("20060102") + randomString(16) + ".md"
}

// normalizePath strips the leading vault-name segment from WebDAV paths so that
// clients whose "Remote Base Directory" is set to their vault name (e.g. Obsidian
// Remotely Save) transparently access the flat note root.
//
//	"/"                     → "/"          root access unchanged
//	"/note.md"              → "/note.md"   root-level file unchanged
//	"/YinMo/"               → "/"          vault-dir with trailing slash → root
//	"/YinMo"                → "/"          vault-dir without trailing slash → root
//	"/YinMo/note.md"        → "/note.md"
//	"/YinMo/assets/img.png" → "/assets/img.png"
//
// The vault name is not stored; any single leading directory segment is stripped.
// Because our note store is intentionally flat (no user-created subdirectories),
// a leading directory can only be a client-side vault prefix.
//
// Single-segment paths that contain a "." are assumed to be files at the root
// (e.g. "note.md") and are left unchanged. Single-segment paths with no "."
// (e.g. "YinMo") are assumed to be bare vault-directory names and mapped to "/".
// Vault names that contain a "." (e.g. "My.Vault") are not supported by this
// heuristic; such clients should leave Remote Base Directory empty.
func normalizePath(name string) string {
	rel := strings.TrimPrefix(name, "/")
	if idx := strings.Index(rel, "/"); idx != -1 {
		// Multi-segment: strip the first segment.
		return "/" + rel[idx+1:]
	}
	// Single-segment with no dot → bare directory name (vault prefix) → root.
	// Single-segment with a dot → file at the root (e.g. "note.md") → unchanged.
	if rel != "" && !strings.ContainsRune(rel, '.') {
		return "/"
	}
	return name
}
