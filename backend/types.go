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
type Structure struct {
	Order      []string            `json:"order"`                // IDs of top-level items
	Parents    map[string]string   `json:"parents"`              // Mapping of child ID to parent ID
	ChildOrder map[string][]string `json:"childOrder"`           // Mapping of parent ID to list of child IDs
	Trash      []TrashEntry        `json:"trash,omitempty"`      // Soft-deleted notes awaiting permanent removal
	Dark       bool                `json:"dark"`                 // User interface theme preference
}

// CommitInfo provides information about a specific git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`    // Commit hash
	Message string    `json:"message"` // Commit message
	Author  string    `json:"author"`  // Author of the commit
	Date    time.Time `json:"date"`    // Date and time of the commit
}
