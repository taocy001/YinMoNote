package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

// ── Virtual tree construction ─────────────────────────────────────────────────

// buildVirtualTree constructs the in-memory virtual directory tree from
// _structure.json.  When the structure is unavailable (missing / encrypted /
// corrupt), hasStructure is set to false and the tree contains a flat mapping
// of title-based paths identical to buildTitleMap, so callers can detect the
// fallback case and apply the legacy code path.
func (dfs *davFileSystem) buildVirtualTree() *davVirtualTree {
	vt := &davVirtualTree{
		byPath:       make(map[string]*davVirtualNode),
		children:     make(map[string][]os.FileInfo),
		noteToPath:   make(map[string]string),
		vaultProxies: make(map[string]bool),
	}

	st := dfs.lib.GetStructureParsed()
	if st == nil {
		log.Printf("[DAV-VT] hasStructure=false (structure nil)")
		// Structure unavailable: populate byPath from flat title map so that
		// individual file Stat/OpenFile calls still resolve correctly.
		m := dfs.buildTitleMap()
		for id, titleName := range m.idToTitle {
			title := strings.TrimSuffix(titleName, ".md")
			vt.byPath["/"+titleName] = &davVirtualNode{id: id, isDir: false, title: title}
			vt.noteToPath[id] = "/" + titleName
		}
		return vt // hasStructure remains false
	}
	log.Printf("[DAV-VT] hasStructure=true childOrder_keys=%d order_len=%d", len(st.ChildOrder), len(st.Order))
	vt.hasStructure = true

	// Populate vault proxies so Mkdir and RemoveAll can detect them quickly.
	for _, seg := range st.VaultProxies {
		vt.vaultProxies[seg] = true
	}

	// Collect on-disk note FileInfo for ModTime/Size lookups.
	type noteEntry struct {
		modTime time.Time
		size    int64
	}
	noteInfo := make(map[string]noteEntry)
	entries, _ := os.ReadDir(dfs.lib.DataDir)
	for _, e := range entries {
		if !e.IsDir() && isExposableNote(e.Name()) {
			if info, err := e.Info(); err == nil {
				noteInfo[e.Name()] = noteEntry{modTime: info.ModTime(), size: info.Size()}
			}
		}
	}

	// Build set of note IDs that are virtual directories.
	// Any ID that appears as a key in ChildOrder is a directory, even if it
	// currently has no children (e.g. a newly MKCOL-created empty folder).
	isDirID := make(map[string]bool)
	for id := range st.ChildOrder {
		isDirID[id] = true
	}

	// Build set of all trash IDs to exclude from listings.
	trashIDs := make(map[string]bool)
	for _, te := range st.Trash {
		trashIDs[te.ID] = true
	}

	// getTitle returns the display title for a note or folder ID.
	// Prefers the cached value in st.Titles, falls back to reading the file.
	// Non-canonical filenames (e.g. "My Note.md") are exposed under their own
	// stem rather than their content — the filename IS the human-readable label.
	getTitle := func(id string) string {
		if st.Titles != nil {
			if t, ok := st.Titles[id]; ok && t != "" {
				return t
			}
		}
		if strings.HasSuffix(id, ".md") {
			// Non-canonical filenames: the stem is already the display label.
			// Do not extract from content so that "existing.md" with content "old"
			// is still reachable at "/existing.md", not "/old.md".
			if !validFileRegex.MatchString(id) {
				return strings.TrimSuffix(id, ".md")
			}
			if t := extractNoteTitle(dfs.lib.FullPath(id)); t != "" {
				return t
			}
			return strings.TrimSuffix(id, ".md")
		}
		return id // non-.md folder ID: use ID as label
	}

	// computeMaxModTime returns the latest ModTime in the subtree rooted at id.
	var computeMaxModTime func(string, map[string]bool) time.Time
	computeMaxModTime = func(id string, visited map[string]bool) time.Time {
		if visited[id] {
			return time.Time{}
		}
		visited[id] = true
		var mt time.Time
		if ni, ok := noteInfo[id]; ok && ni.modTime.After(mt) {
			mt = ni.modTime
		}
		for _, child := range st.ChildOrder[id] {
			if ct := computeMaxModTime(child, visited); ct.After(mt) {
				mt = ct
			}
		}
		return mt
	}

	// processLevel populates vt for a list of IDs under a given parent path.
	// parentPath must end with "/" (use "/" for root).
	var processLevel func(parentPath string, ids []string)
	processLevel = func(parentPath string, ids []string) {
		// Collect non-trash IDs and sort alphabetically for deterministic dedup.
		sorted := make([]string, 0, len(ids))
		for _, id := range ids {
			if !trashIDs[id] {
				sorted = append(sorted, id)
			}
		}
		sort.Strings(sorted)

		seen := make(map[string]int) // base title → collision count
		for _, id := range sorted {
			rawTitle := davSanitizeTitle(getTitle(id))
			if rawTitle == "" {
				rawTitle = id
			}
			n := seen[rawTitle]
			seen[rawTitle]++
			displayTitle := rawTitle
			if n > 0 {
				displayTitle = fmt.Sprintf("%s (%d)", rawTitle, n+1)
			}

			if isDirID[id] {
				// Virtual directory node.
				dirPath := parentPath + displayTitle // e.g. "/ProjectA"
				mt := computeMaxModTime(id, make(map[string]bool))
				if mt.IsZero() {
					mt = time.Now()
				}
				node := &davVirtualNode{id: id, isDir: true, title: displayTitle, modTime: mt}
				vt.byPath[dirPath] = node
				vt.byPath[dirPath+"/"] = node // accept both forms

				// Add this directory entry to parent's children list.
				vt.children[parentPath] = append(vt.children[parentPath],
					&davVirtualDirInfo{name: displayTitle, modTime: mt})

				// If the directory node is a note (not a pure folder ID), also expose
				// its content as a same-named .md file inside the directory.
				// This mirrors the "folder note" pattern used by batch import.
				if strings.HasSuffix(id, ".md") {
					if ni, ok := noteInfo[id]; ok {
						filePath := dirPath + "/" + displayTitle + ".md"
						fileNode := &davVirtualNode{id: id, isDir: false, title: displayTitle, modTime: ni.modTime}
						vt.byPath[filePath] = fileNode
						vt.noteToPath[id] = filePath
						vt.children[dirPath+"/"] = append(vt.children[dirPath+"/"],
							&davVirtualFileInfo{name: displayTitle + ".md", size: ni.size, modTime: ni.modTime})
					}
				}

				// Recurse into children.
				processLevel(dirPath+"/", st.ChildOrder[id])

			} else if strings.HasSuffix(id, ".md") {
				// Leaf note: flat file inside parent.
				if ni, ok := noteInfo[id]; ok {
					filePath := parentPath + displayTitle + ".md"
					node := &davVirtualNode{id: id, isDir: false, title: displayTitle, modTime: ni.modTime}
					vt.byPath[filePath] = node
					vt.noteToPath[id] = filePath
					vt.children[parentPath] = append(vt.children[parentPath],
						&davVirtualFileInfo{name: displayTitle + ".md", size: ni.size, modTime: ni.modTime})
				}
			}
			// Non-.md IDs without children (empty folders) are silently omitted.
		}
	}

	// Build the set of all IDs referenced anywhere in the structure so orphans
	// can be detected.
	referenced := make(map[string]bool)
	for _, id := range st.Order {
		referenced[id] = true
	}
	for id, children := range st.ChildOrder {
		referenced[id] = true
		for _, c := range children {
			referenced[c] = true
		}
	}

	// Augment root order with orphan notes (on disk but absent from structure).
	augmentedOrder := make([]string, len(st.Order))
	copy(augmentedOrder, st.Order)
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && isExposableNote(name) && !referenced[name] && !trashIDs[name] {
			augmentedOrder = append(augmentedOrder, name)
		}
	}

	processLevel("/", augmentedOrder)
	return vt
}
