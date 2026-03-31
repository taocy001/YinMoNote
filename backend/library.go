package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// validFileRegex enforces the canonical filename format: 8-digit date + 16 alphanumeric chars + extension.
// This strict pattern is the primary path-traversal defence: any filename that does not match is
// rejected before ever reaching filepath.Join, so sequences like "../", "%2e%2e", or null bytes
// cannot form a valid name.
var validFileRegex = regexp.MustCompile(`^[0-9]{8}[a-z0-9]{16}\.(md|png|jpg|jpeg|gif|webp)$`)

// validGitHashRegex enforces the 40-character lowercase hex format of a git commit SHA-1 hash.
// Validated before passing to go-git to prevent arbitrary strings reaching the plumbing layer.
var validGitHashRegex = regexp.MustCompile(`^[0-9a-f]{40}$`)

// validUploadExts is the allowlist for user-uploaded asset extensions.
// Only image formats are permitted; text or executable files cannot be uploaded.
var validUploadExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// NoteLibrary is the storage engine for all notes and assets.
// It owns the git repository for version history and enforces all quota constraints.
// It has no knowledge of HTTP — callers (Server methods) are responsible for
// translating NoteLibrary errors into appropriate HTTP response codes.
type NoteLibrary struct {
	DataDir        string          // Root directory that maps to the Docker volume mount
	AssetsDir      string          // Sub-directory for uploaded image assets
	ConfigPath     string          // Absolute path to config.json (outside DataDir)
	Config         AppConfig       // Runtime quotas and UI preferences
	repo           *git.Repository // Git repo in DataDir; provides version history per file
	pendingCommits map[string]bool // Files written since the last auto-commit cycle
	lastWriteTime  time.Time       // Time of the most recent markPending call
	lastCommitTime time.Time       // Time of the most recent successful auto-commit
	// mu guards pendingCommits, lastWriteTime and Config (narrow scope).
	mu sync.Mutex
	// structureMu serialises all read-modify-write operations on _structure.json.
	// Held by SaveStructure (frontend writes) and purgeExpiredTrash (background cleanup)
	// to prevent the purger from overwriting concurrent frontend structure saves.
	structureMu sync.Mutex
	// gitMu serialises all go-git operations. go-git's Worktree is not safe for
	// concurrent use: simultaneous wt.Add/Remove/Commit calls from the auto-committer
	// goroutine and synchronous delete/rollback handlers can corrupt the index.
	gitMu sync.Mutex
	// reconcilePending is set to true by WebDAV write/delete/rename operations to
	// signal the reconcile debouncer that a structure refresh is needed.
	// Avoids O(N²) reconcile calls during bulk uploads.
	reconcilePending atomic.Bool
}

// NewNoteLibrary opens or creates the note library at dataDir.
// configPath is the path to config.json, kept outside dataDir so that the data
// volume contains only notes and assets (config holds auth secrets and quotas).
// On first run a git repository is initialised and a default config is written.
// On subsequent runs, config is read and any out-of-range quota values are
// clamped back to safe defaults before being written back — this prevents a
// manually-edited config from destabilising the server.
func NewNoteLibrary(dataDir, assetsDir, configPath string) (*NoteLibrary, error) {
	os.MkdirAll(dataDir, 0700)
	os.MkdirAll(filepath.Join(dataDir, assetsDir), 0700)
	config := DefaultConfig()
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
	}
	// Clamp quotas to prevent hand-edited configs from opening the server to abuse.
	clampConfig(&config)

	// Persist the clamped config; log on failure but continue — startup still succeeds.
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		if err := atomicWriteFile(configPath, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "YinMo: failed to persist clamped config: %v\n", err)
		}
	}
	// Ensure the assets directory exists (self-healing if manually deleted)
	os.MkdirAll(filepath.Join(dataDir, assetsDir), 0755)

	repo, err := git.PlainOpen(dataDir)
	if err == git.ErrRepositoryNotExists {
		repo, err = git.PlainInit(dataDir, false)
	}
	if err != nil {
		return nil, err
	}
	lib := &NoteLibrary{
		DataDir:        dataDir,
		AssetsDir:      assetsDir,
		ConfigPath:     configPath,
		Config:         config,
		repo:           repo,
		pendingCommits: make(map[string]bool),
	}
	lib.reconcileStructure()
	return lib, nil
}

// CheckNoteQuota returns an error if saving the note would exceed configured limits.
// The total-note count is only checked when the note is new (stat returns not-exist),
// avoiding a full directory scan on every save of an existing note.
func (l *NoteLibrary) CheckNoteQuota(n string, s int64) error {
	if s > l.Config.MaxNoteSize {
		return fmt.Errorf("limit_note_size")
	}
	if _, err := os.Stat(l.FullPath(n)); os.IsNotExist(err) {
		notes, listErr := l.ListNotes()
		if listErr != nil {
			return fmt.Errorf("quota_check_failed")
		}
		if len(notes) >= l.Config.MaxTotalNotes {
			return fmt.Errorf("limit_total_notes")
		}
	}
	return nil
}

// CheckAssetQuota verifies if an asset can be saved without exceeding quotas.
// Only files that pass both the extension allowlist and the canonical name regex are
// counted, matching the set visible in handleListAssets. Stray files (e.g. .gitkeep,
// OS artefacts, .tmp leftovers) are excluded so they cannot silently exhaust the quota.
func (l *NoteLibrary) CheckAssetQuota(s int64) error {
	if s > l.Config.MaxAssetSize {
		return fmt.Errorf("limit_asset_size")
	}
	entries, err := os.ReadDir(filepath.Join(l.DataDir, l.AssetsDir))
	if err != nil {
		return fmt.Errorf("quota_check_failed")
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if validUploadExts[ext] && validFileRegex.MatchString(e.Name()) {
				count++
			}
		}
	}
	if count >= l.Config.MaxTotalAssets {
		return fmt.Errorf("limit_total_assets")
	}
	return nil
}

// IsValidName returns true only for the structure file and note/asset filenames
// that match the canonical format generated by the client. Any other string —
// including path traversal sequences — is rejected.
func (l *NoteLibrary) IsValidName(n string) bool {
	return n == "_structure.json" || validFileRegex.MatchString(n)
}

// FullPath resolves a validated filename to its absolute path within the data directory.
func (l *NoteLibrary) FullPath(n string) string { return filepath.Join(l.DataDir, n) }

// AssetsPath returns the absolute path to the assets directory.
func (l *NoteLibrary) AssetsPath() string { return filepath.Join(l.DataDir, l.AssetsDir) }

// FullAssetPath resolves a validated asset filename to its absolute path within the assets directory.
func (l *NoteLibrary) FullAssetPath(n string) string { return filepath.Join(l.DataDir, l.AssetsDir, n) }

// AtomicWrite writes data to a uniquely-named temp file then renames it to the
// target path. os.Rename is atomic on POSIX systems within the same filesystem,
// so a crash between the write and the rename leaves the original file intact.
// os.CreateTemp produces a unique filename per call, preventing concurrent writes
// to the same note from corrupting each other's temp file.
func (l *NoteLibrary) AtomicWrite(n string, d []byte) error {
	return atomicWriteFile(l.FullPath(n), d, 0600)
}

// ListNotes returns a sorted list of all notes with their title (first line).
func (l *NoteLibrary) ListNotes() ([]NoteInfo, error) {
	fs, err := os.ReadDir(l.DataDir)
	if err != nil {
		return nil, err
	}
	res := make([]NoteInfo, 0)
	for _, f := range fs {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") && l.IsValidName(f.Name()) {
			info, err := f.Info()
			if err != nil {
				// File was deleted between ReadDir and Info (race with concurrent delete). Skip it.
				fmt.Fprintf(os.Stderr, "YinMo: ListNotes skipping %s: %v\n", f.Name(), err)
				continue
			}
			title := extractNoteTitle(l.FullPath(f.Name()))
			res = append(res, NoteInfo{Name: f.Name(), ModTime: info.ModTime().UnixMilli(), Title: title})
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ModTime > res[j].ModTime })
	return res, nil
}

// SaveNote writes note content atomically and registers the file for the next
// git auto-commit cycle. The note content is expected to be an ENC1 blob when
// the client has encryption enabled; the server treats it as opaque bytes.
// When the SYNC_COMMIT=1 environment variable is set (e.g. in E2E test runs),
// the note is committed immediately instead of batching into the auto-committer,
// so that version history is queryable right after each save.
func (l *NoteLibrary) SaveNote(n, c string) error {
	if os.Getenv("SYNC_COMMIT") == "1" {
		return l.SaveNoteAndCommit(n, c, "Auto-save")
	}
	if err := l.AtomicWrite(n, []byte(c)); err != nil {
		return err
	}
	l.markPending(n)
	return nil
}

// SaveAsset writes an uploaded image file atomically and registers it for git.
func (l *NoteLibrary) SaveAsset(n string, r io.Reader) error {
	c, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	rel := filepath.Join(l.AssetsDir, n)
	if err := l.AtomicWrite(rel, c); err != nil {
		return err
	}
	l.markPending(rel)
	return nil
}

// DeleteAsset removes an uploaded asset from disk and commits the removal to git.
// The disk removal is authoritative — returns an error only if os.Remove fails.
// Git operations are best-effort: a file that was never committed (still pending)
// won't exist in the git index, so wt.Remove / wt.Commit may be no-ops or fail;
// those errors are logged and ignored rather than surfaced to the caller.
func (l *NoteLibrary) DeleteAsset(n string) error {
	rel := filepath.Join(l.AssetsDir, n)
	if err := os.Remove(filepath.Join(l.DataDir, rel)); err != nil && !os.IsNotExist(err) {
		return err
	}
	l.mu.Lock()
	delete(l.pendingCommits, rel)
	l.mu.Unlock()
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	wt, err := l.repo.Worktree()
	if err != nil {
		return nil // git unavailable; disk delete succeeded — acceptable
	}
	if _, err := wt.Remove(filepath.ToSlash(rel)); err != nil {
		// File was never committed to git (still pending) — nothing to remove from index.
		fmt.Fprintf(os.Stderr, "YinMo: DeleteAsset wt.Remove(%s) skipped: %v\n", rel, err)
		return nil
	}
	if _, err := wt.Commit("Delete "+rel, &git.CommitOptions{Author: &object.Signature{Name: "YinMo", Email: "auto@local", When: time.Now()}}); err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: DeleteAsset commit failed: %v\n", err)
	}
	return nil
}

// DeleteNote removes the note from disk and immediately commits the removal to git.
// Unlike SaveNote (which batches into the auto-committer), deletions are committed
// synchronously so that the version history accurately reflects the moment of deletion.
// Returns an error only if the disk removal fails; git errors are best-effort.
func (l *NoteLibrary) DeleteNote(n string) error {
	if err := os.Remove(l.FullPath(n)); err != nil && !os.IsNotExist(err) {
		return err
	}
	l.mu.Lock()
	delete(l.pendingCommits, n)
	l.mu.Unlock()
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	wt, err := l.repo.Worktree()
	if err != nil {
		return nil // git unavailable; disk delete succeeded — acceptable
	}
	if _, err := wt.Remove(filepath.ToSlash(n)); err != nil {
		// File was never committed to git (still pending) — nothing to remove from index.
		fmt.Fprintf(os.Stderr, "YinMo: DeleteNote wt.Remove(%s) skipped: %v\n", n, err)
		return nil
	}
	if _, err := wt.Commit("Delete "+n, &git.CommitOptions{Author: &object.Signature{Name: "YinMo", Email: "auto@local", When: time.Now()}}); err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: DeleteNote commit failed: %v\n", err)
	}
	return nil
}

// SaveNoteAndCommit writes a note atomically and immediately commits it to git.
// Used for rollback operations where consistency between file state and commit
// history requires synchronous persistence rather than the background auto-committer.
func (l *NoteLibrary) SaveNoteAndCommit(n, content, msg string) error {
	if err := l.AtomicWrite(n, []byte(content)); err != nil {
		return err
	}
	// Remove from pending to avoid a duplicate auto-commit cycle.
	l.mu.Lock()
	delete(l.pendingCommits, n)
	l.mu.Unlock()
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	wt, err := l.repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add(filepath.ToSlash(n)); err != nil {
		return err
	}
	_, err = wt.Commit(msg, &git.CommitOptions{Author: &object.Signature{Name: "YinMo", Email: "auto@local", When: time.Now()}, AllowEmptyCommits: true})
	return err
}
