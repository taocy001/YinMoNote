package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartTrashPurger runs a background goroutine that auto-deletes notes that have
// been in the trash for more than 30 days. Runs once per hour.
func (l *NoteLibrary) StartTrashPurger() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		l.purgeExpiredTrash()
	}
}

// purgeExpiredTrash reads _structure.json, identifies trash entries older than 30
// days, deletes their files, and writes back the updated structure.
//
// Design constraints:
//   - Must NOT hold l.mu during file I/O (l.mu guards only pendingCommits/config).
//   - Must NOT use a shadow struct for JSON round-trip — uses json.RawMessage to
//     preserve all unknown fields and avoid data loss when the frontend adds new
//     fields to the structure.
//   - Validates trash entry IDs with validFileRegex before os.Remove to prevent
//     path-traversal attacks from a tampered _structure.json.
func (l *NoteLibrary) purgeExpiredTrash() {
	l.structureMu.Lock()
	defer l.structureMu.Unlock()

	structPath := filepath.Join(l.DataDir, "_structure.json")
	data, err := os.ReadFile(structPath)
	if err != nil {
		return
	}
	raw := string(data)
	// Skip encrypted structures — client handles purging on next unlock.
	if strings.HasPrefix(raw, "ENC1:") || strings.HasPrefix(raw, "\"ENC1:") {
		return
	}
	// Unwrap JSON-quoted string if needed.
	if strings.HasPrefix(raw, "\"") {
		var unquoted string
		if json.Unmarshal(data, &unquoted) == nil {
			data = []byte(unquoted)
		}
	}

	// Parse only the trash field using json.RawMessage to preserve all other fields.
	var generic map[string]json.RawMessage
	if json.Unmarshal(data, &generic) != nil {
		return
	}
	trashRaw, ok := generic["trash"]
	if !ok {
		return
	}
	type trashEntry struct {
		ID        string `json:"id"`
		DeletedAt int64  `json:"deletedAt"`
	}
	var entries []trashEntry
	if json.Unmarshal(trashRaw, &entries) != nil || len(entries) == 0 {
		return
	}

	now := time.Now().UnixMilli()
	thirtyDays := int64(30 * 24 * 60 * 60 * 1000)
	var remaining []trashEntry
	var purged []string
	for _, entry := range entries {
		if now-entry.DeletedAt > thirtyDays {
			purged = append(purged, entry.ID)
		} else {
			remaining = append(remaining, entry)
		}
	}
	if len(purged) == 0 {
		return
	}

	// Delete files — validate each ID to prevent path-traversal.
	for _, id := range purged {
		if !validFileRegex.MatchString(id) {
			continue
		}
		os.Remove(filepath.Join(l.DataDir, id))
	}

	// Also remove purged IDs from titles and tags maps if present.
	purgedSet := make(map[string]bool, len(purged))
	for _, id := range purged {
		purgedSet[id] = true
	}
	if titlesRaw, ok := generic["titles"]; ok {
		var titles map[string]string
		if json.Unmarshal(titlesRaw, &titles) == nil {
			for id := range purgedSet {
				delete(titles, id)
			}
			if b, err := json.Marshal(titles); err == nil {
				generic["titles"] = b
			}
		}
	}
	if tagsRaw, ok := generic["tags"]; ok {
		var tags map[string][]string
		if json.Unmarshal(tagsRaw, &tags) == nil {
			for id := range purgedSet {
				delete(tags, id)
			}
			if b, err := json.Marshal(tags); err == nil {
				generic["tags"] = b
			}
		}
	}

	// Write back updated trash array (preserve all other fields via generic map).
	if remaining == nil {
		remaining = make([]trashEntry, 0)
	}
	if b, err := json.Marshal(remaining); err == nil {
		generic["trash"] = b
	}
	newData, err := json.Marshal(generic)
	if err != nil {
		return
	}
	atomicWriteFile(structPath, newData, 0600)
	l.markPending("_structure.json")
}
