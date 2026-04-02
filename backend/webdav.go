package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
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
//
// Notes are sorted by canonical ID (alphabetical) before building the dedup map so
// that collision numbers are stable regardless of file ModTime changes (C-006 fix).
func (dfs *davFileSystem) buildTitleMap() *davNameMap {
	m := &davNameMap{
		titleToID: make(map[string]string),
		idToTitle: make(map[string]string),
	}
	notes, err := dfs.lib.ListNotes()
	if err != nil {
		return m
	}
	// Sort by canonical ID for deterministic dedup ordering regardless of ModTime.
	sort.Slice(notes, func(i, j int) bool { return notes[i].Name < notes[j].Name })
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

// ── Virtual directory tree ─────────────────────────────────────────────────────

// davVirtualNode represents one item in the virtual WebDAV directory tree
// derived from _structure.json.  All IDs are canonical note filenames
// (e.g. "20260329…md"); there are no separate "folder ID" objects in the
// current frontend model — a note that has children acts as a directory.
type davVirtualNode struct {
	id      string    // canonical note filename (e.g. "20260329abc.md")
	isDir   bool      // true when the node maps to a WebDAV directory (has children)
	title   string    // sanitized, deduped display name (without .md extension)
	modTime time.Time // for files: file ModTime; for dirs: max child ModTime
}

// davVirtualTree is the complete in-memory virtual filesystem snapshot derived
// from _structure.json.  It is rebuilt on every davFileSystem method call and
// is therefore always consistent with the current state of _structure.json.
// Paths use a canonical form: "/" for root, "/DirTitle" or "/DirTitle/" for
// virtual directories (both stored), "/DirTitle/NoteTitle.md" for files.
type davVirtualTree struct {
	hasStructure bool                     // false when structure is absent/encrypted/corrupt
	byPath       map[string]*davVirtualNode // virtual path → node
	children     map[string][]os.FileInfo   // dir path (with trailing /) → ordered children FileInfo
	noteToPath   map[string]string          // canonical note ID → virtual file path
}

// davVirtualDirInfo implements os.FileInfo for virtual directories that have
// no physical counterpart on disk.
type davVirtualDirInfo struct {
	name    string
	modTime time.Time
}

func (i *davVirtualDirInfo) Name() string      { return i.name }
func (i *davVirtualDirInfo) Size() int64       { return 0 }
func (i *davVirtualDirInfo) Mode() os.FileMode { return os.ModeDir | 0555 }
func (i *davVirtualDirInfo) ModTime() time.Time { return i.modTime }
func (i *davVirtualDirInfo) IsDir() bool       { return true }
func (i *davVirtualDirInfo) Sys() interface{}  { return nil }

// davVirtualFileInfo implements os.FileInfo for files inside virtual directories,
// returned by davVirtualDirFile.Readdir.  Size and ModTime come from the real
// os.FileInfo of the underlying canonical-ID file gathered during tree build.
type davVirtualFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i *davVirtualFileInfo) Name() string      { return i.name }
func (i *davVirtualFileInfo) Size() int64       { return i.size }
func (i *davVirtualFileInfo) Mode() os.FileMode { return 0444 }
func (i *davVirtualFileInfo) ModTime() time.Time { return i.modTime }
func (i *davVirtualFileInfo) IsDir() bool       { return false }
func (i *davVirtualFileInfo) Sys() interface{}  { return nil }

// davStatWrappedFile wraps a webdav.File and overrides its Stat method to
// return a caller-supplied os.FileInfo.  Used for read-only virtual file opens
// so that the webdav library receives the virtual display name (e.g. "Task
// One.md") in Stat() rather than the underlying canonical-ID name
// (e.g. "20260101…aa02.md") that the OS file would report.
type davStatWrappedFile struct {
	webdav.File
	info os.FileInfo
}

func (f *davStatWrappedFile) Stat() (os.FileInfo, error) { return f.info, nil }

// davVirtualDirFile implements webdav.File for virtual directories.
// It embeds a real webdav.File opened on the DataDir root so that DeadProps
// and Patch are handled correctly by the inner implementation.
type davVirtualDirFile struct {
	webdav.File           // real root dir file — provides DeadProps/Patch/Close
	dirInfo  *davVirtualDirInfo
	children []os.FileInfo
	pos      int
}

func (f *davVirtualDirFile) Read([]byte) (int, error)            { return 0, io.EOF }
func (f *davVirtualDirFile) Seek(int64, int) (int64, error)      { return 0, os.ErrPermission }
func (f *davVirtualDirFile) Stat() (os.FileInfo, error)          { return f.dirInfo, nil }

func (f *davVirtualDirFile) Readdir(count int) ([]os.FileInfo, error) {
	if count <= 0 {
		// Return all remaining entries (webdav package calls Readdir(-1)).
		result := f.children[f.pos:]
		f.pos = len(f.children)
		return result, nil
	}
	if f.pos >= len(f.children) {
		return nil, io.EOF
	}
	end := f.pos + count
	if end > len(f.children) {
		end = len(f.children)
	}
	result := f.children[f.pos:end]
	f.pos = end
	return result, nil
}

// davNewNoteFile wraps davCommitFile and additionally updates _structure.json
// when Close is called after a successful write: the new note is added to the
// structure under its parent (or at root level when parentID is empty).
type davNewNoteFile struct {
	davCommitFile
	parentID     string // empty = root; otherwise the parent note's canonical ID
	displayTitle string // used to set structure.Titles for the new note
}

func (f *davNewNoteFile) Close() error {
	err := f.davCommitFile.Close()
	if err == nil && f.written {
		noteID := f.davCommitFile.rel
		updateErr := f.lib.UpdateStructureFunc(func(st *Structure) {
			if st.Titles == nil {
				st.Titles = make(map[string]string)
			}
			st.Titles[noteID] = f.displayTitle
			if f.parentID == "" {
				st.Order = append(st.Order, noteID)
			} else {
				if st.ChildOrder == nil {
					st.ChildOrder = make(map[string][]string)
				}
				if st.Parents == nil {
					st.Parents = make(map[string]string)
				}
				st.ChildOrder[f.parentID] = append(st.ChildOrder[f.parentID], noteID)
				st.Parents[noteID] = f.parentID
			}
		})
		if updateErr != nil {
			fmt.Fprintf(os.Stderr, "YinMo: WebDAV new-note structure update failed: %v\n", updateErr)
		}
	}
	return err
}

// ── Virtual tree construction ─────────────────────────────────────────────────

// buildVirtualTree constructs the in-memory virtual directory tree from
// _structure.json.  When the structure is unavailable (missing / encrypted /
// corrupt), hasStructure is set to false and the tree contains a flat mapping
// of title-based paths identical to buildTitleMap, so callers can detect the
// fallback case and apply the legacy code path.
func (dfs *davFileSystem) buildVirtualTree() *davVirtualTree {
	vt := &davVirtualTree{
		byPath:     make(map[string]*davVirtualNode),
		children:   make(map[string][]os.FileInfo),
		noteToPath: make(map[string]string),
	}

	st := dfs.lib.GetStructureParsed()
	if st == nil {
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
	vt.hasStructure = true

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

	// Build set of note IDs that have children (→ virtual directories).
	isDirID := make(map[string]bool)
	for id, children := range st.ChildOrder {
		if len(children) > 0 {
			isDirID[id] = true
		}
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

// ── davFileSystem ─────────────────────────────────────────────────────────────

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

	// Condition 3: normalization would discard a meaningful path segment by
	// collapsing to root (bare-directory heuristic in normalizePath).
	if normalized == "/" && cleanOrig != "/" {
		return name
	}

	return normalized
}

func (dfs *davFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	vt := dfs.buildVirtualTree()
	name = dfs.normalizeDavPath(name, vt)
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
			st.Titles[newID] = dirTitle
			if parentID == "" {
				st.Order = append(st.Order, newID)
			} else {
				if st.ChildOrder == nil {
					st.ChildOrder = make(map[string][]string)
				}
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

	// Fallback: no structure available — create a real directory on disk.
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

	return nil, os.ErrNotExist
}

// openNewVirtualFile creates a new canonical note file under the resolved
// virtual parent directory and registers it in _structure.json.
// name must be a clean path like "/ParentTitle/NoteTitle.md".
func (dfs *davFileSystem) openNewVirtualFile(ctx context.Context, vt *davVirtualTree, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Only .md files are handled as notes; non-.md assets are unsupported inside
	// virtual dirs (the flat note store has no subdirectory for them).
	if !strings.HasSuffix(name, ".md") {
		return nil, os.ErrPermission
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

	return nil, os.ErrNotExist
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
// Path depth: at most 5 segments are allowed (measured after normalizePath has
// stripped the leading vault prefix).  The note store is intentionally flat so
// only depth 1 is expected in practice; the higher cap accommodates clients
// that create subdirectories for attachments or templates.
//
//	depth 1 — root files            e.g. "note.md"
//	depth 2 — one subfolder         e.g. "assets/image.png"
//	depth 3 — two subfolders        e.g. "assets/2024/photo.png"
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
