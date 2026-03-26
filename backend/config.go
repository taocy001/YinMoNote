package main

// MCPAccessLevel defines the permission level for AI access to a note.
type MCPAccessLevel string

const (
	MCPAccessDeny  MCPAccessLevel = "deny"  // note is invisible to AI
	MCPAccessRead  MCPAccessLevel = "read"  // AI can read, not write
	MCPAccessWrite MCPAccessLevel = "write" // AI can read and write
)

// MCPRule is a single access-control rule. Exactly one condition field
// (Tag, NoteID, TitleGlob, SubtreeOf) should be set per rule.
type MCPRule struct {
	Tag       string         `json:"tag,omitempty"`        // note carries this tag
	NoteID    string         `json:"note_id,omitempty"`    // exact note filename
	TitleGlob string         `json:"title_glob,omitempty"` // title matches glob pattern
	SubtreeOf string         `json:"subtree_of,omitempty"` // descendant of this note ID
	Access    MCPAccessLevel `json:"access"`
}

// MCPPolicy is the complete access-control configuration for the MCP server.
// Rules are evaluated top-to-bottom; the first match wins (first-match-wins).
// If no rule matches, DefaultAccess applies.
type MCPPolicy struct {
	Enabled       bool           `json:"enabled"`
	DefaultAccess MCPAccessLevel `json:"default_access"` // "" is treated as "read"
	Rules         []MCPRule      `json:"rules"`
}

// AppConfig defines the per-library quotas and UI preferences.
// It is persisted in a config file whose path is passed to NewNoteLibrary (default
// ./config.json in the working directory, separate from the data volume).
type AppConfig struct {
	MaxTotalNotes    int     `json:"maxTotalNotes"`    // Maximum number of notes allowed
	MaxTotalAssets   int     `json:"maxTotalAssets"`   // Maximum number of assets allowed
	MaxImagesPerNote int     `json:"maxImagesPerNote"` // Maximum images per individual note
	MaxNestingDepth  int     `json:"maxNestingDepth"`  // Maximum directory depth
	MaxItemsPerLevel int     `json:"maxItemsPerLevel"` // Maximum items in a single folder
	MaxNoteSize      int64   `json:"maxNoteSize"`      // Max size of a note in bytes
	MaxAssetSize     int64   `json:"maxAssetSize"`     // Max size of an asset in bytes
	Port             string  `json:"port"`             // Server port
	Lang             string  `json:"lang"`             // Default language
	IsDark           bool    `json:"isDark"`           // Legacy: migrated to ThemeMode
	ThemeMode        string  `json:"themeMode"`        // "auto" | "light" | "dark" (default "auto")
	EditorWidth      string  `json:"editorWidth"`      // Editor max width
	FontSize         int     `json:"fontSize"`         // Editor font size
	TypewriterMode   bool    `json:"typewriterMode"`   // Typewriter mode
	ServerEncrypt    bool    `json:"serverEncrypt"`    // Server-side encryption enabled
	Pbkdf2Salt             string  `json:"pbkdf2Salt,omitempty"`    // Base64-encoded PBKDF2 salt, synced across devices
	IdleTimeout            float64 `json:"idleTimeout"`            // Lock timeout in minutes
	AllowExternalImages    bool    `json:"allowExternalImages"`    // Load external HTTP images in notes
	// SessionTokenHash is the SHA-256 hex hash of the client-derived session token.
	// When non-empty, all API requests must carry a matching Bearer token.
	// Never returned to clients via GET /api/config.
	SessionTokenHash string `json:"sessionTokenHash,omitempty"`
	// MCPTokenHash is the SHA-256 hex hash of the MCP-specific bearer token.
	// Kept separate from SessionTokenHash so the two access paths are independently revocable.
	// Never returned to clients via GET /api/config.
	MCPTokenHash string    `json:"mcpTokenHash,omitempty"`
	MCPPolicy    MCPPolicy `json:"mcpPolicy"`
}

// DefaultConfig returns conservative quota defaults for a single-user personal library.
// The ServerEncrypt flag defaults to false — encryption is opt-in and controlled by the
// client; the server stores whatever bytes the client sends.
func DefaultConfig() AppConfig {
	return AppConfig{
		MaxTotalNotes:    500,
		MaxTotalAssets:   1000,
		MaxImagesPerNote: 10,
		MaxNestingDepth:  3,
		MaxItemsPerLevel: 200,
		MaxNoteSize:      512 * 1024,
		MaxAssetSize:     2 * 1024 * 1024,
		Port:             ":8080",
		Lang:             "zh",
		IsDark:           false,
		ThemeMode:        "auto",
		EditorWidth:      "full",
		FontSize:         16,
		TypewriterMode:   false,
		ServerEncrypt:    false, // Default to no encryption on cloud
		IdleTimeout:      0,
	}
}

// isValidMCPAccess reports whether a is one of the three recognised access levels.
func isValidMCPAccess(a MCPAccessLevel) bool {
	return a == MCPAccessDeny || a == MCPAccessRead || a == MCPAccessWrite
}

// sanitizeMCPPolicy drops or replaces any unrecognised access-level values so
// a hand-edited or API-supplied policy cannot introduce undefined behaviour.
// Invalid DefaultAccess falls back to "read". Rules are removed if:
//   - their Access value is not one of "read"/"write"/"deny", or
//   - all four condition fields (Tag/NoteID/TitleGlob/SubtreeOf) are empty,
//     which would make the rule a permanent no-op (it can never match any note).
func sanitizeMCPPolicy(p *MCPPolicy) {
	if p.DefaultAccess != "" && !isValidMCPAccess(p.DefaultAccess) {
		p.DefaultAccess = MCPAccessRead
	}
	valid := p.Rules[:0]
	for _, r := range p.Rules {
		if !isValidMCPAccess(r.Access) {
			continue
		}
		if r.Tag == "" && r.NoteID == "" && r.TitleGlob == "" && r.SubtreeOf == "" {
			continue // no-condition rule can never match — drop silently
		}
		valid = append(valid, r)
	}
	p.Rules = valid
}

// clampConfig enforces safe bounds on all quota fields.
// Called at startup (NewNoteLibrary) and at every runtime config update
// (handleUpdateConfig) to prevent hand-edited or API-supplied values from
// opening the server to disk-exhaustion or DoS.
func clampConfig(cfg *AppConfig) {
	if cfg.MaxTotalNotes < 10 || cfg.MaxTotalNotes > 5000 {
		cfg.MaxTotalNotes = 500
	}
	if cfg.MaxTotalAssets < 10 || cfg.MaxTotalAssets > 10000 {
		cfg.MaxTotalAssets = 1000
	}
	if cfg.MaxImagesPerNote < 1 || cfg.MaxImagesPerNote > 100 {
		cfg.MaxImagesPerNote = 10
	}
	if cfg.MaxNestingDepth < 1 || cfg.MaxNestingDepth > 20 {
		cfg.MaxNestingDepth = 10
	}
	if cfg.MaxItemsPerLevel < 5 || cfg.MaxItemsPerLevel > 5000 {
		cfg.MaxItemsPerLevel = 500
	}
	if cfg.MaxNoteSize < 1024 || cfg.MaxNoteSize > 10*1024*1024 {
		cfg.MaxNoteSize = 512 * 1024
	}
	if cfg.MaxAssetSize < 1024 || cfg.MaxAssetSize > 50*1024*1024 {
		cfg.MaxAssetSize = 2 * 1024 * 1024
	}
}
