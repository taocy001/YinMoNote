package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// mcpStructure is a superset of Structure used only by the MCP layer.
// It parses the client-written titles and tags maps that are persisted
// in _structure.json but are not consumed by the backend's own Structure type.
// IMPORTANT: JSON tags MUST match the keys the frontend uses in saveStructure()
// (useLibrary.ts), which are "titles" and "tags" — NOT "noteTitles"/"noteTags".
type mcpStructure struct {
	Order      []string            `json:"order"`
	Parents    map[string]string   `json:"parents"`
	ChildOrder map[string][]string `json:"childOrder"`
	NoteTitles map[string]string   `json:"titles"` // set by frontend, may be absent
	NoteTags   map[string][]string `json:"tags"`   // set by frontend, may be absent
}

// loadMCPStructure reads _structure.json and parses it into mcpStructure.
// Returns an empty struct on any error (file absent, encrypted, malformed).
// Encrypted blobs (ENC1: prefix) are unreadable server-side; callers receive
// an empty struct and should treat all tag/title conditions as non-matching.
func (l *NoteLibrary) loadMCPStructure() mcpStructure {
	raw, err := os.ReadFile(l.FullPath("_structure.json"))
	if err != nil {
		return mcpStructure{}
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "ENC1:") {
		return mcpStructure{} // encrypted — tag/title metadata not accessible
	}
	var st mcpStructure
	if err := json.Unmarshal(raw, &st); err != nil {
		return mcpStructure{}
	}
	return st
}

// evaluateMCPAccess returns the effective access level for the given note
// under the active policy. Rules are evaluated in order; the first match wins.
func evaluateMCPAccess(filename, title string, tags []string, policy MCPPolicy, st mcpStructure) MCPAccessLevel {
	if !policy.Enabled {
		return MCPAccessRead
	}
	for _, rule := range policy.Rules {
		if mcpRuleMatches(rule, filename, title, tags, st) {
			return rule.Access
		}
	}
	if policy.DefaultAccess == "" {
		return MCPAccessRead
	}
	return policy.DefaultAccess
}

// mcpRuleMatches reports whether a single rule applies to the given note.
func mcpRuleMatches(rule MCPRule, filename, title string, tags []string, st mcpStructure) bool {
	switch {
	case rule.NoteID != "":
		return filename == rule.NoteID

	case rule.Tag != "":
		for _, t := range tags {
			if t == rule.Tag {
				return true
			}
		}

	case rule.TitleGlob != "":
		// Use case-insensitive matching so that deny rules like "Secret*" also
		// cover notes titled "secret diary". filepath.Match is case-sensitive on
		// all platforms; normalise both sides to lower-case.
		matched, err := filepath.Match(strings.ToLower(rule.TitleGlob), strings.ToLower(title))
		return err == nil && matched

	case rule.SubtreeOf != "":
		return mcpIsDescendantOf(filename, rule.SubtreeOf, st)
	}
	return false
}

// mcpIsDescendantOf returns true if filename appears anywhere in the subtree
// rooted at parentID. BFS traversal with a visited guard prevents infinite
// loops from malformed or cyclic structures.
func mcpIsDescendantOf(filename, parentID string, st mcpStructure) bool {
	visited := map[string]bool{}
	queue := []string{parentID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, child := range st.ChildOrder[cur] {
			if child == filename {
				return true
			}
			if !visited[child] {
				queue = append(queue, child)
			}
		}
	}
	return false
}
