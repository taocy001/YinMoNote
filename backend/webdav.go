package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// davFileSystem wraps webdav.Dir to:
//  1. Normalize vault-prefix paths so Obsidian vaults map to the note root.
//  2. Block access to internal YinMoNote files (_structure.json, hidden files).
//  3. Filter blocked entries from directory listings so clients never see them.
//  4. Translate canonical-ID note filenames to human-readable title names so that
//     WebDAV clients (e.g. Obsidian) see "数据中心网络.md" instead of "20260329…md".
//  5. Queue written/deleted files for git commit via markPending.
type davFileSystem struct {
	inner webdav.Dir
	lib   *NoteLibrary
}

// normalizeDavPath applies vault-prefix normalization (normalizePath) unless
// the original path should be preserved because it refers to (or is inside)
// the active virtual tree.  Three conditions prevent stripping:
//
//  1. The original path itself is a known virtual node (existing virtual file or dir).
//  2. The parent segment of the original path is a known virtual directory — so
//     paths like "/ProjectA/NewNote.md" or "/ProjectA/SubDir" (new items inside a
//     virtual dir) are not stripped to "/NewNote.md" or "/SubDir".
//  3. normalizePath would collapse the path to "/" (bare dir → root heuristic) but
//     the original path had a non-empty segment — preserve it so MKCOL/MKDIR of
//     a new root-level virtual directory name like "/NewFolder" is not lost.
//
// When the virtual tree is inactive (no _structure.json), behaviour is
// identical to normalizePath alone.
//
// Examples with virtual dir "ProjectA" active:
//
//	"/ProjectA/Task One.md" → kept (matches virtual node)
//	"/ProjectA/Task Two.md" → kept (parent "/ProjectA" is virtual dir)
//	"/ProjectA/SubDir"      → kept (parent "/ProjectA" is virtual dir)
//	"/NewFolder"            → kept (would collapse to "/"; preserved for MKCOL)
//	"/YinMo/note.md"        → "/note.md" (parent not a virtual dir; vault prefix stripped)
func (dfs *davFileSystem) normalizeDavPath(name string, vt *davVirtualTree) string {
	normalized := normalizePath(name)
	if normalized == name || !vt.hasStructure {
		return normalized
	}
	cleanOrig := "/" + strings.Trim(name, "/")

	// Condition 1: exact virtual node match.
	if _, ok := vt.byPath[cleanOrig]; ok {
		return name
	}

	// Condition 2: parent is a known virtual directory.
	lastSlash := strings.LastIndex(cleanOrig, "/")
	if lastSlash > 0 {
		parentPath := cleanOrig[:lastSlash]
		if node, ok := vt.byPath[parentPath]; ok && node.isDir {
			return name
		}
	}

	return normalized
}

func (dfs *davFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	vt := dfs.buildVirtualTree()
	origName := name
	name = dfs.normalizeDavPath(name, vt)
	// normalizeDavPath strips single-segment vault-prefix paths to "/" for read
	// operations.  For MKCOL we need the original segment name as the directory
	// title (e.g. "/NewFolder"), so re-apply it when normalization collapsed to "/".
	if name == "/" && origName != "/" {
		name = "/" + strings.Trim(origName, "/")
	}
	if !dfs.allowed(name) {
		return os.ErrPermission
	}

	// When structure is available, create a "folder note" — a canonical note
	// with H1 title equal to the directory name — and register it in the
	// structure.  This is the same mechanism used by the batch-import path.
	cleanName := "/" + strings.Trim(name, "/")
	lastSlash := strings.LastIndex(cleanName, "/")
	dirTitle := cleanName[lastSlash+1:]
	if dirTitle == "" {
		return os.ErrInvalid
	}
	if vt.hasStructure {
		// Reject if path already exists in virtual tree.
		if _, exists := vt.byPath[cleanName]; exists {
			return os.ErrExist
		}

		// ── Vault-proxy detection ──────────────────────────────────────────────
		// A MKCOL whose path is a single dot-free segment (i.e. normalizePath
		// would collapse it to "/") issued at root level is a vault-name MKCOL
		// from a WebDAV client like Obsidian Remotely Save.  Registering it as a
		// real folder note would shadow the vault-prefix stripping in all
		// subsequent normalizeDavPath calls, causing the client to see an empty
		// vault and creating spurious duplicate notes.
		//
		// Instead we record the segment name in Structure.VaultProxies.  The
		// virtual tree purposely excludes vault proxies so normalizeDavPath
		// continues to treat them as transparent vault-name prefixes:
		//   PROPFIND /vault/ → root notes  (prefix stripped)
		//   PUT /vault/note.md → root note  (prefix stripped)
		//   PROPFIND / → root notes only (proxy not listed as a subdirectory)
		// A vault-proxy MKCOL is a root-level single-segment path that:
		//   1. Has no dot (so normalizePath would collapse it to "/")
		//   2. Is non-empty after whitespace trimming
		//   3. Is not composed entirely of ASCII digits (e.g. "2024" should
		//      create a real folder note, not a transparent vault prefix)
		isAllDigits := func(s string) bool {
			if s == "" {
				return false
			}
			for _, r := range s {
				if r < '0' || r > '9' {
					return false
				}
			}
			return true
		}
		isVaultProxyMKCOL := lastSlash == 0 && // root-level dir
			!strings.ContainsRune(dirTitle, '.') && // no dot → normalizePath strips it
			strings.TrimSpace(dirTitle) == dirTitle && // no leading/trailing whitespace
			dirTitle != "" && // non-empty
			!isAllDigits(dirTitle) // not pure digits (e.g. year folders)

		if isVaultProxyMKCOL {
			if vt.isVaultProxy(dirTitle) {
				// Idempotent: vault proxy already registered — treat as success so
				// WebDAV clients that re-issue MKCOL on every sync (e.g. Remotely Save)
				// do not receive a 405 error that could abort the sync session.
				return nil
			}
			return dfs.lib.UpdateStructureFunc(func(st *Structure) {
				// Deduplicate: only add if not already present.
				for _, existing := range st.VaultProxies {
					if existing == dirTitle {
						return
					}
				}
				st.VaultProxies = append(st.VaultProxies, dirTitle)
			})
		}

		// Determine parent virtual dir.
		parentPath := cleanName[:lastSlash+1]
		var parentID string
		if parentPath != "/" {
			parentClean := strings.TrimSuffix(parentPath, "/")
			parentNode, ok := vt.byPath[parentClean]
			if !ok || !parentNode.isDir {
				return os.ErrNotExist
			}
			parentID = parentNode.id
		}
		// Quota check.
		newID := newCanonicalNoteID()
		if err := dfs.lib.CheckNoteQuota(newID, 0); err != nil {
			return &davQuotaError{reason: err.Error()}
		}
		// Write a minimal note containing only the H1 title.
		content := "# " + dirTitle + "\n"
		if err := dfs.lib.AtomicWrite(newID, []byte(content)); err != nil {
			return err
		}
		// Update structure.
		if err := dfs.lib.UpdateStructureFunc(func(st *Structure) {
			if st.Titles == nil {
				st.Titles = make(map[string]string)
			}
			if st.ChildOrder == nil {
				st.ChildOrder = make(map[string][]string)
			}
			st.Titles[newID] = dirTitle
			// Mark as a virtual directory by adding an empty ChildOrder entry.
			// buildVirtualTree uses "id in ChildOrder" to decide dir vs file;
			// without this entry the new folder would render as a flat .md file.
			if _, exists := st.ChildOrder[newID]; !exists {
				st.ChildOrder[newID] = []string{}
			}
			if parentID == "" {
				st.Order = append(st.Order, newID)
			} else {
				if st.Parents == nil {
					st.Parents = make(map[string]string)
				}
				st.ChildOrder[parentID] = append(st.ChildOrder[parentID], newID)
				st.Parents[newID] = parentID
			}
		}); err != nil {
			// Structure update failed; remove the orphan note we just created.
			_ = os.Remove(dfs.lib.FullPath(newID))
			return err
		}
		dfs.lib.markPending(newID)
		dfs.lib.reconcilePending.Store(true)
		return nil
	}

	// Fallback: no structure available.
	// Single-segment dot-free paths are vault-name candidates: silently succeed
	// so the client can proceed without creating a real on-disk directory that
	// would conflict once structure is later established.
	if lastSlash == 0 && !strings.ContainsRune(dirTitle, '.') {
		return nil
	}
	return dfs.inner.Mkdir(ctx, name, perm)
}

func (dfs *davFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	vt := dfs.buildVirtualTree()
	name = dfs.normalizeDavPath(name, vt)
	if !dfs.allowed(name) {
		return nil, os.ErrPermission
	}

	rel := strings.TrimPrefix(name, "/")

	// ── Root directory open ────────────────────────────────────────────────────
	if rel == "" || name == "/" {
		if vt.hasStructure {
			inner, err := dfs.inner.OpenFile(ctx, "/", os.O_RDONLY, 0)
			if err != nil {
				return nil, err
			}
			return &davVirtualDirFile{
				File:     inner,
				dirInfo:  &davVirtualDirInfo{name: "", modTime: time.Now()},
				children: vt.children["/"],
			}, nil
		}
		// Fallback: no structure — use legacy directory listing.
		f, err := dfs.inner.OpenFile(ctx, "/", flag, perm)
		if err != nil {
			return nil, err
		}
		m := dfs.buildTitleMap()
		return &davDirFile{File: f, m: m}, nil
	}

	cleanName := "/" + strings.Trim(name, "/")

	// ── Virtual directory open ─────────────────────────────────────────────────
	if node, ok := vt.byPath[cleanName]; ok && node.isDir {
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) != 0 {
			return nil, os.ErrPermission // cannot write-open a directory
		}
		inner, err := dfs.inner.OpenFile(ctx, "/", os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		dirPath := cleanName + "/"
		return &davVirtualDirFile{
			File:     inner,
			dirInfo:  &davVirtualDirInfo{name: node.title, modTime: node.modTime},
			children: vt.children[dirPath],
		}, nil
	}

	// ── Virtual file open ──────────────────────────────────────────────────────
	if node, ok := vt.byPath[cleanName]; ok && !node.isDir {
		canonicalPath := "/" + node.id
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
			isNew := false
			if flag&os.O_CREATE != 0 {
				if _, statErr := os.Stat(dfs.lib.FullPath(node.id)); os.IsNotExist(statErr) {
					isNew = true
					if err := dfs.lib.CheckNoteQuota(node.id, 0); err != nil {
						return nil, &davQuotaError{reason: err.Error()}
					}
				}
			}
			f, err := dfs.inner.OpenFile(ctx, canonicalPath, flag, perm)
			if err != nil {
				return nil, err
			}
			truncated := flag&os.O_TRUNC != 0
			return &davCommitFile{File: f, lib: dfs.lib, rel: node.id, written: truncated, isNew: isNew}, nil
		}
		// Read-only open: wrap with a virtual stat so the webdav library reports
		// the display name (e.g. "Task One.md") rather than the canonical ID.
		f, err := dfs.inner.OpenFile(ctx, canonicalPath, flag, perm)
		if err != nil {
			return nil, err
		}
		innerInfo, serr := dfs.inner.Stat(ctx, canonicalPath)
		if serr != nil {
			return f, nil // stat failed; return unwrapped rather than erroring
		}
		return &davStatWrappedFile{
			File: f,
			info: &davTitleFileInfo{FileInfo: innerInfo, name: node.title + ".md"},
		}, nil
	}

	// ── Canonical note ID direct access ──────────────────────────────────────
	// When a client accesses a note by its canonical ID (e.g. sync tools that
	// remember canonical IDs, or test code), bypass the virtual tree and serve
	// the underlying file directly.  Applies even when the virtual tree is active.
	// This block runs BEFORE the new-file-create block so that a PUT to an
	// existing canonical ID path overwrites the note in-place rather than minting
	// a second canonical ID.
	if validFileRegex.MatchString(rel) {
		isNew := false
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
			if flag&os.O_CREATE != 0 {
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
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
			truncated := flag&os.O_TRUNC != 0
			return &davCommitFile{File: f, lib: dfs.lib, rel: rel, written: truncated, isNew: isNew}, nil
		}
		return f, nil
	}

	// ── New file create inside virtual directory ──────────────────────────────
	// Path not in virtual tree; only proceed for write-with-create.
	if flag&(os.O_CREATE) != 0 && vt.hasStructure {
		return dfs.openNewVirtualFile(ctx, vt, cleanName, flag, perm)
	}

	// ── Legacy fallback (no structure) ────────────────────────────────────────
	// Handles root-level files when structure is unavailable.
	if !vt.hasStructure {
		isRootMD := !strings.ContainsRune(rel, '/') && strings.HasSuffix(rel, ".md")
		if isRootMD {
			m := dfs.buildTitleMap()
			name = m.translateToID(name)
			rel = strings.TrimPrefix(name, "/")
		}
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
		if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
			return &davDirFile{File: f, m: nil}, nil
		}
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
			truncated := flag&os.O_TRUNC != 0
			return &davCommitFile{File: f, lib: dfs.lib, rel: rel, written: truncated, isNew: isNew}, nil
		}
		return f, nil
	}

	// Non-.md asset fallback: path is under a virtual dir (has slash) but not in
	// the tree; try to serve the file by basename from DataDir flat storage.
	if strings.Contains(rel, "/") {
		basename := rel[strings.LastIndex(rel, "/")+1:]
		if basename != "" && !strings.HasSuffix(basename, ".md") && !isBlockedSegment(basename) {
			if f, err := dfs.inner.OpenFile(ctx, "/"+basename, flag, perm); err == nil {
				return f, nil
			}
		}
	}

	return nil, os.ErrNotExist
}

// openNewVirtualFile creates a new canonical note file under the resolved
// virtual parent directory and registers it in _structure.json.
// name must be a clean path like "/ParentTitle/NoteTitle.md".
func (dfs *davFileSystem) openNewVirtualFile(ctx context.Context, vt *davVirtualTree, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Non-.md files (attachments, test files, etc.) are stored flat in DataDir
	// using just the basename, bypassing the virtual-tree note registration.
	// This lets Remotely Save connection tests and basic asset sync work.
	if !strings.HasSuffix(name, ".md") {
		basename := name[strings.LastIndex(name, "/")+1:]
		if basename == "" {
			return nil, os.ErrInvalid
		}
		if isBlockedSegment(basename) {
			return nil, os.ErrPermission
		}
		return dfs.inner.OpenFile(ctx, "/"+basename, flag, perm)
	}

	lastSlash := strings.LastIndex(name, "/")
	if lastSlash < 0 {
		return nil, os.ErrInvalid
	}
	parentPath := name[:lastSlash+1] // includes trailing /
	filename := name[lastSlash+1:]
	displayTitle := strings.TrimSuffix(filename, ".md")

	// Determine parent ID.
	var parentID string
	if parentPath != "/" {
		parentClean := strings.TrimSuffix(parentPath, "/")
		parentNode, ok := vt.byPath[parentClean]
		if !ok || !parentNode.isDir {
			return nil, os.ErrNotExist // parent virtual dir does not exist
		}
		parentID = parentNode.id
	}

	newID := newCanonicalNoteID()
	if err := dfs.lib.CheckNoteQuota(newID, 0); err != nil {
		return nil, &davQuotaError{reason: err.Error()}
	}

	f, err := dfs.inner.OpenFile(ctx, "/"+newID, flag, perm)
	if err != nil {
		return nil, err
	}
	truncated := flag&os.O_TRUNC != 0
	return &davNewNoteFile{
		davCommitFile: davCommitFile{File: f, lib: dfs.lib, rel: newID, written: truncated, isNew: true},
		parentID:      parentID,
		displayTitle:  displayTitle,
	}, nil
}

func (dfs *davFileSystem) RemoveAll(ctx context.Context, name string) error {
	vt := dfs.buildVirtualTree()

	// ── Vault-proxy DELETE (must check before normalizeDavPath) ───────────────
	// normalizeDavPath strips single-segment dot-free paths to "/", so vault
	// proxy names would be lost.  Intercept them here using the raw path.
	if vt.hasStructure {
		rawClean := "/" + strings.Trim(name, "/")
		rawSeg := strings.Trim(rawClean, "/")
		if !strings.ContainsRune(rawSeg, '/') && vt.isVaultProxy(rawSeg) {
			return dfs.lib.UpdateStructureFunc(func(st *Structure) {
				updated := st.VaultProxies[:0]
				for _, s := range st.VaultProxies {
					if s != rawSeg {
						updated = append(updated, s)
					}
				}
				st.VaultProxies = updated
			})
		}
	}

	name = dfs.normalizeDavPath(name, vt)
	if !dfs.allowed(name) {
		return os.ErrPermission
	}

	cleanName := "/" + strings.Trim(name, "/")

	if vt.hasStructure {
		node, ok := vt.byPath[cleanName]
		if !ok {
			// Canonical note ID: delete the physical file directly.
			relName := strings.TrimPrefix(cleanName, "/")
			if validFileRegex.MatchString(relName) {
				err := dfs.inner.RemoveAll(ctx, cleanName)
				if err == nil || os.IsNotExist(err) {
					if err == nil {
						dfs.lib.markPending(relName)
						dfs.lib.reconcilePending.Store(true)
					}
					return nil
				}
				return err
			}
			// Non-.md asset stored flat in DataDir: delete by basename.
			if strings.Contains(relName, "/") {
				basename := relName[strings.LastIndex(relName, "/")+1:]
				if basename != "" && !strings.HasSuffix(basename, ".md") && !isBlockedSegment(basename) {
					_ = dfs.inner.RemoveAll(ctx, "/"+basename)
				}
			}
			// Not in virtual tree: nothing to delete (return nil — idempotent).
			return nil
		}

		if node.isDir {
			// Collect all note IDs in the subtree.
			st := dfs.lib.GetStructureParsed()
			var noteIDs []string
			if st != nil {
				noteIDs = collectSubtreeNoteIDs(node.id, st)
			} else {
				noteIDs = []string{node.id}
			}

			// Delete physical files.
			for _, noteID := range noteIDs {
				if err := dfs.inner.RemoveAll(ctx, "/"+noteID); err != nil && !os.IsNotExist(err) {
					return err
				}
				dfs.lib.markPending(noteID)
			}

			// Remove from structure synchronously (fixes C-004).
			if updateErr := dfs.lib.UpdateStructureFunc(func(s *Structure) {
				for _, noteID := range noteIDs {
					removeNoteFromStructure(s, noteID)
				}
				// Also remove the dir node itself from its parent if it's a non-.md folder ID.
				if !strings.HasSuffix(node.id, ".md") {
					removeNoteFromStructure(s, node.id)
				}
			}); updateErr != nil {
				fmt.Fprintf(os.Stderr, "YinMo: WebDAV dir delete structure update failed: %v\n", updateErr)
			}
			dfs.lib.reconcilePending.Store(true)
			return nil
		}

		// Single note file delete.
		noteID := node.id
		if err := dfs.inner.RemoveAll(ctx, "/"+noteID); err != nil && !os.IsNotExist(err) {
			return err
		}
		// Update structure synchronously so the next PROPFIND does not see the
		// deleted file (fixes C-004: was previously async via reconcilePending).
		if updateErr := dfs.lib.UpdateStructureFunc(func(s *Structure) {
			removeNoteFromStructure(s, noteID)
		}); updateErr != nil {
			fmt.Fprintf(os.Stderr, "YinMo: WebDAV file delete structure update failed: %v\n", updateErr)
		}
		dfs.lib.markPending(noteID)
		dfs.lib.reconcilePending.Store(true)
		return nil
	}

	// Fallback: no structure — use legacy title-map + inner delete.
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
	vt := dfs.buildVirtualTree()
	oldName = dfs.normalizeDavPath(oldName, vt)
	newName = dfs.normalizeDavPath(newName, vt)
	if !dfs.allowed(oldName) || !dfs.allowed(newName) {
		return os.ErrPermission
	}

	oldClean := "/" + strings.Trim(oldName, "/")
	newClean := "/" + strings.Trim(newName, "/")

	if vt.hasStructure {
		oldNode, oldOK := vt.byPath[oldClean]
		if !oldOK {
			return os.ErrNotExist
		}
		if oldNode.isDir {
			// Renaming a virtual directory: update structure.Titles for the dir node
			// (and update H1 if it is a note).
			newLastSlash := strings.LastIndex(newClean, "/")
			newTitle := davSanitizeTitle(newClean[newLastSlash+1:])
			if newTitle == "" {
				newTitle = newClean[newLastSlash+1:]
			}
			if strings.HasSuffix(oldNode.id, ".md") {
				if err := dfs.updateNoteH1(oldNode.id, newTitle); err != nil {
					return err
				}
			}
			return dfs.lib.UpdateStructureFunc(func(st *Structure) {
				if st.Titles == nil {
					st.Titles = make(map[string]string)
				}
				st.Titles[oldNode.id] = newTitle
			})
		}

		// Single note rename or move.
		noteID := oldNode.id

		// Resolve old and new parent paths.
		oldLastSlash := strings.LastIndex(oldClean, "/")
		newLastSlash := strings.LastIndex(newClean, "/")
		oldParentPath := oldClean[:oldLastSlash+1]
		newParentPath := newClean[:newLastSlash+1]
		newFilename := newClean[newLastSlash+1:]
		newTitle := davSanitizeTitle(strings.TrimSuffix(newFilename, ".md"))
		if newTitle == "" {
			newTitle = strings.TrimSuffix(newFilename, ".md")
		}

		resolveParentID := func(parentPath string) string {
			if parentPath == "/" {
				return ""
			}
			parentClean := strings.TrimSuffix(parentPath, "/")
			if n, ok := vt.byPath[parentClean]; ok {
				return n.id
			}
			return ""
		}
		oldParentID := resolveParentID(oldParentPath)
		newParentID := resolveParentID(newParentPath)

		// Determine if this is a same-parent rename vs. a structural move.
		sameParent := oldParentPath == newParentPath
		if sameParent {
			// Title rename: update H1 content and structure.Titles.
			if err := dfs.updateNoteH1(noteID, newTitle); err != nil {
				return err
			}
			return dfs.lib.UpdateStructureFunc(func(st *Structure) {
				if st.Titles == nil {
					st.Titles = make(map[string]string)
				}
				st.Titles[noteID] = newTitle
			})
		}

		// Structural move: update parents and childOrder.
		return dfs.lib.UpdateStructureFunc(func(st *Structure) {
			// Remove from old parent.
			if oldParentID == "" {
				st.Order = removeFromStringSlice(st.Order, noteID)
			} else if st.ChildOrder != nil {
				st.ChildOrder[oldParentID] = removeFromStringSlice(st.ChildOrder[oldParentID], noteID)
				if len(st.ChildOrder[oldParentID]) == 0 {
					delete(st.ChildOrder, oldParentID)
				}
			}
			if st.Parents != nil {
				delete(st.Parents, noteID)
			}
			// Add to new parent.
			if newParentID == "" {
				st.Order = append(st.Order, noteID)
			} else {
				if st.ChildOrder == nil {
					st.ChildOrder = make(map[string][]string)
				}
				if st.Parents == nil {
					st.Parents = make(map[string]string)
				}
				st.ChildOrder[newParentID] = append(st.ChildOrder[newParentID], noteID)
				st.Parents[noteID] = newParentID
			}
			// Update title.
			if st.Titles == nil {
				st.Titles = make(map[string]string)
			}
			st.Titles[noteID] = newTitle
		})
	}

	// Fallback: no structure — use legacy title-map rename.
	m := dfs.buildTitleMap()
	oldTranslated := m.translateToID(oldName)
	oldRel := strings.TrimPrefix(oldTranslated, "/")
	newRel := strings.TrimPrefix(newName, "/")

	isOldCanonical := validFileRegex.MatchString(oldRel)
	isNewRootMD := !strings.ContainsRune(newRel, '/') && strings.HasSuffix(newRel, ".md")

	if isOldCanonical && isNewRootMD {
		newTitle := davSanitizeTitle(strings.TrimSuffix(newRel, ".md"))
		if newTitle == "" {
			newTitle = strings.TrimSuffix(newRel, ".md")
		}
		return dfs.updateNoteH1(oldRel, newTitle)
	}

	newTranslated := m.translateToID(newName)
	newRel = strings.TrimPrefix(newTranslated, "/")
	err := dfs.inner.Rename(ctx, oldTranslated, newTranslated)
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
	vt := dfs.buildVirtualTree()
	name = dfs.normalizeDavPath(name, vt)
	if !dfs.allowed(name) {
		// Return ErrPermission (not ErrNotExist) so that the webdav PROPFIND
		// walkFS silently skips blocked entries via handlePropfindError rather
		// than aborting the response with "Internal Server Error".
		return nil, os.ErrPermission
	}

	rel := strings.TrimPrefix(name, "/")
	if rel == "" {
		return dfs.inner.Stat(ctx, "/")
	}

	cleanName := "/" + strings.Trim(name, "/")

	if node, ok := vt.byPath[cleanName]; ok {
		if node.isDir {
			return &davVirtualDirInfo{name: node.title, modTime: node.modTime}, nil
		}
		// File: stat the underlying canonical file and wrap with virtual name.
		info, err := dfs.inner.Stat(ctx, "/"+node.id)
		if err != nil {
			return nil, err
		}
		return &davTitleFileInfo{FileInfo: info, name: node.title + ".md"}, nil
	}

	// Not in virtual tree: for legacy fallback (no structure), attempt title→ID.
	if !vt.hasStructure {
		m := dfs.buildTitleMap()
		translated := m.translateToID(name)
		info, err := dfs.inner.Stat(ctx, translated)
		if err != nil {
			return nil, err
		}
		if translated != name {
			rel := strings.TrimPrefix(name, "/")
			return &davTitleFileInfo{FileInfo: info, name: rel}, nil
		}
		return info, nil
	}

	// Canonical note IDs are always accessible directly, even with virtual tree active.
	if validFileRegex.MatchString(rel) {
		return dfs.inner.Stat(ctx, cleanName)
	}

	// Non-.md asset fallback: path under a virtual dir; try basename in DataDir.
	if strings.Contains(rel, "/") {
		basename := rel[strings.LastIndex(rel, "/")+1:]
		if basename != "" && !strings.HasSuffix(basename, ".md") && !isBlockedSegment(basename) {
			if info, err := dfs.inner.Stat(ctx, "/"+basename); err == nil {
				return info, nil
			}
		}
	}

	return nil, os.ErrNotExist
}
