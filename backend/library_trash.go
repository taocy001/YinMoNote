package main

import (
	"encoding/json"
	"fmt"
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
//
// Deletion failure semantics: if os.Remove fails with a genuine error (not
// os.IsNotExist), the entry is kept in the trash array and retried on the next
// hourly run. This prevents the file from becoming a silent orphan — a note that
// exists on disk but is invisible to the user and unrecoverable.
// See TD-M3-033 in docs/arch/tech-debt.md.
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
	var purgedEntries []trashEntry
	for _, entry := range entries {
		if now-entry.DeletedAt > thirtyDays {
			purgedEntries = append(purgedEntries, entry)
		} else {
			remaining = append(remaining, entry)
		}
	}
	if len(purgedEntries) == 0 {
		return
	}

	// Delete files — validate each ID to prevent path-traversal.
	// Track only successfully deleted IDs so that metadata (titles/tags) is only
	// cleared for files that were actually removed from disk.
	// Entries where os.Remove fails are added back to remaining so the purger
	// retries them on the next hourly run rather than silently orphaning the file.
	var deletedIDs []string
	for _, entry := range purgedEntries {
		id := entry.ID
		if !validFileRegex.MatchString(id) {
			continue
		}
		if err := os.Remove(filepath.Join(l.DataDir, id)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[YinMo] purgeExpiredTrash: failed to remove %q: %v\n", id, err)
			// Keep in trash for retry next hour; do not orphan the file.
			remaining = append(remaining, entry)
			continue
		}
		l.markPending(id) // notify git so the deletion is committed
		deletedIDs = append(deletedIDs, id)
	}

	// Only remove titles/tags for files that were actually deleted from disk.
	purgedSet := make(map[string]bool, len(deletedIDs))
	for _, id := range deletedIDs {
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
	if err := atomicWriteFile(structPath, newData, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "YinMo: purgeExpiredTrash failed to update structure: %v\n", err)
	}
	l.markPending("_structure.json")
}
