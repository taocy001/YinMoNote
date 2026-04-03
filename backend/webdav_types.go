package main

import (
	"fmt"
	"io"
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

func (e *davQuotaError) Error() string       { return "quota exceeded: " + e.reason }
func (e *davQuotaError) Is(target error) bool { return target == os.ErrPermission }
func (e *davQuotaError) Unwrap() error        { return os.ErrPermission }

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
	hasStructure bool                       // false when structure is absent/encrypted/corrupt
	byPath       map[string]*davVirtualNode // virtual path → node
	children     map[string][]os.FileInfo   // dir path (with trailing /) → ordered children FileInfo
	noteToPath   map[string]string          // canonical note ID → virtual file path
	vaultProxies map[string]bool            // segment name → true; vault-proxy dirs registered via MKCOL
}

// isVaultProxy reports whether the given path-segment (no slashes) is a
// registered vault proxy.  Thread-safe: the tree is rebuilt per-request.
func (vt *davVirtualTree) isVaultProxy(seg string) bool {
	return vt.vaultProxies[seg]
}

// davVirtualDirInfo implements os.FileInfo for virtual directories that have
// no physical counterpart on disk.
type davVirtualDirInfo struct {
	name    string
	modTime time.Time
}

func (i *davVirtualDirInfo) Name() string       { return i.name }
func (i *davVirtualDirInfo) Size() int64        { return 0 }
func (i *davVirtualDirInfo) Mode() os.FileMode  { return os.ModeDir | 0555 }
func (i *davVirtualDirInfo) ModTime() time.Time { return i.modTime }
func (i *davVirtualDirInfo) IsDir() bool        { return true }
func (i *davVirtualDirInfo) Sys() interface{}   { return nil }

// davVirtualFileInfo implements os.FileInfo for files inside virtual directories,
// returned by davVirtualDirFile.Readdir.  Size and ModTime come from the real
// os.FileInfo of the underlying canonical-ID file gathered during tree build.
type davVirtualFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i *davVirtualFileInfo) Name() string       { return i.name }
func (i *davVirtualFileInfo) Size() int64        { return i.size }
func (i *davVirtualFileInfo) Mode() os.FileMode  { return 0444 }
func (i *davVirtualFileInfo) ModTime() time.Time { return i.modTime }
func (i *davVirtualFileInfo) IsDir() bool        { return false }
func (i *davVirtualFileInfo) Sys() interface{}   { return nil }

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

func (f *davVirtualDirFile) Read([]byte) (int, error)       { return 0, io.EOF }
func (f *davVirtualDirFile) Seek(int64, int) (int64, error) { return 0, os.ErrPermission }
func (f *davVirtualDirFile) Stat() (os.FileInfo, error)     { return f.dirInfo, nil }

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
