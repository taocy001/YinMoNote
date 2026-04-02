package main

import "time"

// NoteRequest represents a request to save note content.
type NoteRequest struct {
	Content string `json:"content"` // Encrypted note content
}

// NoteInfo contains metadata about a note file.
type NoteInfo struct {
	Name    string `json:"name"`            // Filename
	ModTime int64  `json:"modTime"`         // Last modification time in milliseconds
	Title   string `json:"title,omitempty"` // First heading line, empty for encrypted notes
}

// TrashEntry represents a note that has been soft-deleted.
type TrashEntry struct {
	ID        string `json:"id"`
	DeletedAt int64  `json:"deletedAt"`
}

// Structure defines the organizational hierarchy of notes and folders.
// All fields mirror the LibraryStructure interface in the frontend so that
// UpdateStructureFunc round-trips the full JSON blob without data loss.
type Structure struct {
	Order        []string            `json:"order"`                        // IDs of top-level items
	Parents      map[string]string   `json:"parents"`                      // child ID → parent ID
	ChildOrder   map[string][]string `json:"childOrder"`                   // parent ID → ordered child IDs
	Titles       map[string]string   `json:"titles,omitempty"`             // note/folder ID → display title
	Tags         map[string][]string `json:"tags,omitempty"`               // note ID → tag list
	Dark         bool                `json:"dark"`                         // user interface theme preference
	Pinned       []string            `json:"pinned,omitempty"`             // pinned note IDs (shown first)
	Trash        []TrashEntry        `json:"trash,omitempty"`              // soft-deleted notes
	CommitLabels map[string]string   `json:"commitLabels,omitempty"`       // note ID → custom commit message label
}

// CommitInfo provides information about a specific git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`    // Commit hash
	Message string    `json:"message"` // Commit message
	Author  string    `json:"author"`  // Author of the commit
	Date    time.Time `json:"date"`    // Date and time of the commit
}
