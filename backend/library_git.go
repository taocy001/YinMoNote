package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Auto-commit timing constants.
// commitIdleDelay: commit pending files after 5 minutes of write inactivity
//   (the user has stopped editing — commit their current work).
// commitMaxInterval: absolute upper bound between commits during continuous editing
//   (safety net so no more than 10 minutes of work can be un-committed).
const (
	commitIdleDelay   = 5 * time.Minute
	commitMaxInterval = 10 * time.Minute
)

// GetHistory returns the commit history for a specific file, capped at limit entries.
// Returns an empty slice if the repo has no commits yet (fresh library).
// A limit of 0 is treated as the default maximum of 50 to prevent unbounded responses.
func (l *NoteLibrary) GetHistory(n string, limit int) []CommitInfo {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	p := filepath.ToSlash(n)
	// Hold gitMu for the entire operation (Log + ForEach iteration) because
	// go-git's iterator reads from the object store, which can race with
	// concurrent wt.Add/wt.Commit calls from the auto-committer goroutine.
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	it, err := l.repo.Log(&git.LogOptions{FileName: &p})
	// Use a non-nil slice so JSON serialisation always produces [] instead of null.
	res := make([]CommitInfo, 0)
	if err != nil || it == nil {
		return res
	}
	it.ForEach(func(c *object.Commit) error {
		if len(res) >= limit {
			return fmt.Errorf("stop") // non-nil stops iteration without propagating an error
		}
		res = append(res, CommitInfo{Hash: c.Hash.String(), Message: c.Message, Author: c.Author.Name, Date: c.Author.When})
		return nil
	})
	return res
}

// GetContentAtHash retrieves a file's content from a specific git commit.
// Returns (content, true) when the file exists at that commit — content may be an empty
// string if the file was committed with no bytes. Returns ("", false) when the commit or
// file cannot be found; callers must distinguish this from legitimately empty content.
func (l *NoteLibrary) GetContentAtHash(n, h string) (string, bool) {
	// Hold gitMu for the entire read chain (CommitObject → Tree → File → Contents)
	// to prevent inconsistent reads during concurrent git commits.
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	c, err := l.repo.CommitObject(plumbing.NewHash(h))
	if err != nil {
		return "", false
	}
	t, err := c.Tree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: GetContentAtHash tree error for %s@%s: %v\n", n, h, err)
		return "", false
	}
	f, err := t.File(filepath.ToSlash(n))
	if err != nil {
		return "", false // File not present in this commit — normal for new files.
	}
	s, err := f.Contents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: GetContentAtHash contents error for %s@%s: %v\n", n, h, err)
		return "", false
	}
	return s, true
}

// commitPendingFiles stages and commits a list of files under gitMu.
// Extracted into its own function so that defer l.gitMu.Unlock() fires at the
// end of this call rather than waiting for StartAutoCommitter to return.
func (l *NoteLibrary) commitPendingFiles(fs []string) error {
	l.gitMu.Lock()
	defer l.gitMu.Unlock()
	wt, err := l.repo.Worktree()
	if err != nil {
		return err
	}
	for _, f := range fs {
		slug := filepath.ToSlash(f)
		fullPath := filepath.Join(l.DataDir, f)
		if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
			// File was deleted — stage the removal in git index
			if _, err := wt.Remove(slug); err != nil {
				fmt.Fprintf(os.Stderr, "YinMo: auto-commit wt.Remove(%s) failed: %v\n", f, err)
			}
		} else {
			if _, err := wt.Add(slug); err != nil {
				fmt.Fprintf(os.Stderr, "YinMo: auto-commit wt.Add(%s) failed: %v\n", f, err)
			}
		}
	}
	if _, err := wt.Commit("Auto-save", &git.CommitOptions{Author: &object.Signature{Name: "YinMo", Email: "auto@local", When: time.Now()}}); err != nil {
		// ErrEmptyCommit means all staged files had no actual changes — this is
		// harmless (e.g. a save wrote identical content). Treat as success to
		// avoid re-enqueue loops in StartAutoCommitter.
		if err.Error() == "nothing to commit" {
			return nil
		}
		fmt.Fprintf(os.Stderr, "YinMo: auto-commit wt.Commit failed: %v\n", err)
		return err
	}
	return nil
}

// markPending records a filename for inclusion in the next auto-commit batch.
// The lock is held only for map access, not during any I/O.
func (l *NoteLibrary) markPending(n string) {
	l.mu.Lock()
	l.pendingCommits[n] = true
	l.lastWriteTime = time.Now()
	l.mu.Unlock()
}

// StartAutoCommitter runs as a background goroutine. Instead of a fixed 30-second
// interval, it uses an idle-aware strategy: pending files are committed when the
// user stops writing for commitIdleDelay, or when commitMaxInterval has elapsed
// since the last commit — whichever comes first. This produces far fewer commits
// during active editing while still ensuring timely persistence.
func (l *NoteLibrary) StartAutoCommitter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		pending := len(l.pendingCommits)
		lastWrite := l.lastWriteTime
		lastCommit := l.lastCommitTime
		l.mu.Unlock()
		if pending == 0 {
			continue
		}
		now := time.Now()
		idle := now.Sub(lastWrite) >= commitIdleDelay
		maxAge := !lastCommit.IsZero() && now.Sub(lastCommit) >= commitMaxInterval
		// On very first commit, lastCommitTime is zero — treat as exceeding maxInterval.
		if lastCommit.IsZero() {
			maxAge = now.Sub(lastWrite) >= commitIdleDelay
		}
		if !idle && !maxAge {
			continue
		}
		l.mu.Lock()
		fs := make([]string, 0, len(l.pendingCommits))
		for f := range l.pendingCommits {
			fs = append(fs, f)
		}
		l.pendingCommits = make(map[string]bool)
		l.mu.Unlock()
		if len(fs) == 0 {
			continue
		}
		if err := l.commitPendingFiles(fs); err != nil {
			// Re-enqueue failed files so they are retried on the next cycle.
			l.mu.Lock()
			for _, f := range fs {
				l.pendingCommits[f] = true
			}
			l.mu.Unlock()
			continue
		}
		l.mu.Lock()
		l.lastCommitTime = time.Now()
		l.mu.Unlock()
	}
}

// StartGitGC runs periodic `git gc --auto` to keep the repository size in check.
// Prevents unbounded repository growth from binary assets (images).
func (l *NoteLibrary) StartGitGC() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		cmd := exec.Command("git", "gc", "--auto", "--quiet")
		cmd.Dir = l.DataDir
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "YinMo: git gc failed: %v\n", err)
		}
	}
}
