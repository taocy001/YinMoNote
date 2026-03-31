package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// CheckStructureQuota validates structural integrity of the folder hierarchy.
//
// Only checks for cycles (which would cause infinite loops in traversal).
// Nesting depth and per-level item counts are NOT checked here — they are
// enforced at creation/import time on the client side. Blocking saves based
// on depth/count would prevent ALL structure modifications (delete, move,
// tag edit, pin, etc.) when the existing structure exceeds limits, which is
// a worse outcome than allowing a slightly over-limit structure to persist.
func (l *NoteLibrary) CheckStructureQuota(s Structure) error {
	visited := make(map[string]bool)
	processed := make(map[string]bool)
	var walk func(string) error
	walk = func(id string) error {
		if visited[id] {
			return fmt.Errorf("limit_cycle_detected")
		}
		if processed[id] {
			return nil
		}
		visited[id] = true
		for _, c := range s.ChildOrder[id] {
			if err := walk(c); err != nil {
				return err
			}
		}
		delete(visited, id)
		processed[id] = true
		return nil
	}
	for _, rid := range s.Order {
		if err := walk(rid); err != nil {
			return err
		}
	}
	return nil
}

// GetStructure loads the hierarchical structure of the library.
// If the file is missing it triggers reconcileStructure first so the client
// always receives a structure that at least lists every note on disk.
// The ReadFile is protected by structureMu to prevent a torn read concurrent
// with SaveStructure. Note: the preceding os.Stat check is not under structureMu,
// so reconcileStructure may be called redundantly in rare concurrent cases (benign).
func (l *NoteLibrary) GetStructure() string {
	p := l.FullPath("_structure.json")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		l.reconcileStructure()
	}
	l.structureMu.Lock()
	d, err := os.ReadFile(p)
	l.structureMu.Unlock()
	if err != nil {
		return "{}"
	}
	return string(d)
}

// SaveStructure persists the structure metadata blob. The blob may be an ENC1
// ciphertext (when server encryption is enabled) or a plain JSON object.
// Either way, the server stores it verbatim — it has no opinion on the content.
func (l *NoteLibrary) SaveStructure(s string) error {
	l.structureMu.Lock()
	defer l.structureMu.Unlock()
	if err := l.AtomicWrite("_structure.json", []byte(s)); err != nil {
		return err
	}
	l.markPending("_structure.json")
	return nil
}

// reconcileStructure ensures _structure.json is consistent with notes on disk.
//
// Rules:
//   - If the file is absent or contains corrupt JSON → rebuild from scratch.
//   - If the file is an ENC1 blob → skip (client-managed, server cannot read).
//   - If the file is plain JSON → remove references to .md files that no longer
//     exist on disk, and append any .md files missing from the structure to the
//     top-level order. Folder IDs (non-filename format) are left untouched.
//   - Stale Parents entries are pruned: keys for deleted .md files and keys whose
//     parent folder was removed during the ChildOrder cleanup are deleted (step 5b).
//
// The rebuilt file is written directly (no git commit) so the change is picked
// up silently; the next user operation will push it into the commit queue.
func (l *NoteLibrary) reconcileStructure() {
	l.structureMu.Lock()
	defer l.structureMu.Unlock()
	// 1. Collect all valid .md filenames that exist on disk.
	entries, err := os.ReadDir(l.DataDir)
	if err != nil {
		return
	}
	actualFiles := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && isExposableNote(e.Name()) {
			actualFiles[e.Name()] = true
		}
	}

	// 2. Read the existing structure file.
	structPath := l.FullPath("_structure.json")
	raw, err := os.ReadFile(structPath)
	if err != nil {
		// File absent — build minimal structure from disk contents.
		l.buildMinimalStructure(actualFiles)
		return
	}
	content := strings.TrimSpace(string(raw))
	if strings.HasPrefix(content, "ENC1:") {
		// Encrypted blob — leave untouched.
		return
	}

	// 3. Parse as plain JSON Structure.
	var st Structure
	if err := json.Unmarshal(raw, &st); err != nil || st.Order == nil {
		// Corrupt JSON — rebuild.
		l.buildMinimalStructure(actualFiles)
		return
	}

	// 4. Compute the set of all note IDs referenced in the structure.
	referenced := make(map[string]bool)
	for _, id := range st.Order {
		referenced[id] = true
	}
	for _, children := range st.ChildOrder {
		for _, id := range children {
			referenced[id] = true
		}
	}

	// 5. Fix: remove phantom note references; keep folder IDs intact.
	changed := false
	newOrder := make([]string, 0, len(st.Order))
	for _, id := range st.Order {
		if strings.HasSuffix(id, ".md") && !actualFiles[id] {
			changed = true // note referenced but file gone
		} else {
			newOrder = append(newOrder, id)
		}
	}
	st.Order = newOrder

	if st.ChildOrder == nil {
		st.ChildOrder = make(map[string][]string)
	}
	for folderID, children := range st.ChildOrder {
		newChildren := make([]string, 0, len(children))
		for _, id := range children {
			if strings.HasSuffix(id, ".md") && !actualFiles[id] {
				changed = true
			} else {
				newChildren = append(newChildren, id)
			}
		}
		if len(newChildren) == 0 && len(children) > 0 {
			delete(st.ChildOrder, folderID)
			changed = true
		} else {
			st.ChildOrder[folderID] = newChildren
		}
	}

	// 5b. Remove stale Parents entries: keys no longer on disk or whose parent
	// folder ID was deleted during the ChildOrder cleanup above.
	if st.Parents != nil {
		for key, parentID := range st.Parents {
			if strings.HasSuffix(key, ".md") && !actualFiles[key] {
				delete(st.Parents, key)
				changed = true
			} else if _, folderExists := st.ChildOrder[parentID]; !folderExists && parentID != "" {
				delete(st.Parents, key)
				changed = true
			}
		}
	}

	// 6. Append files that exist on disk but are absent from the structure.
	for name := range actualFiles {
		if !referenced[name] {
			st.Order = append(st.Order, name)
			changed = true
		}
	}

	if !changed {
		return
	}

	d, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = l.AtomicWrite("_structure.json", d)
}

// buildMinimalStructure creates a _structure.json containing all known note files
// in sorted order with no folder hierarchy. Used when the file is absent or corrupt.
func (l *NoteLibrary) buildMinimalStructure(actualFiles map[string]bool) {
	names := make([]string, 0, len(actualFiles))
	for name := range actualFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	st := Structure{
		Order:      names,
		ChildOrder: map[string][]string{},
		Parents:    map[string]string{},
	}
	d, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = l.AtomicWrite("_structure.json", d)
}

// isExposableNote returns true for .md files that should be visible via both
// the WebDAV interface and the web API. Applies a blacklist (not a whitelist)
// so that non-canonical filenames written by WebDAV clients are accepted while
// truly dangerous or internal names are still rejected.
//
// Rejected:
//   - "_structure.json" (internal index)
//   - "." prefix (hidden files, e.g. .git)
//   - "~" prefix or suffix (editor temp files)
//   - null byte (path injection guard)
//   - longer than 255 bytes (filesystem limit)
//   - does not end in ".md"
func isExposableNote(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	if name == "_structure.json" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return false
	}
	if strings.HasPrefix(name, "~") || strings.HasSuffix(name, "~") {
		return false
	}
	if strings.ContainsRune(name, 0) {
		return false
	}
	if len(name) > 255 {
		return false
	}
	return true
}

// StartReconcileDebouncer runs as a background goroutine and coalesces multiple
// WebDAV write/delete/rename events into a single reconcileStructure call.
// It checks reconcilePending every 2 seconds; if the flag is set it clears it
// and runs reconcileStructure once, regardless of how many WebDAV operations
// triggered the flag since the last check.
//
// This prevents the O(N²) reconcile overhead during bulk uploads (N files →
// N markPending calls → 1 reconcile instead of N reconciles).
func (l *NoteLibrary) StartReconcileDebouncer() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if l.reconcilePending.CompareAndSwap(true, false) {
			l.reconcileStructure()
		}
	}
}
