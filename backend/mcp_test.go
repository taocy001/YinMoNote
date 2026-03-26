package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Policy evaluation tests ───────────────────────────────────────────────────

func TestMCPPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	st := mcpStructure{
		Order:      []string{"a.md", "b.md", "c.md"},
		Parents:    map[string]string{"b.md": "a.md", "c.md": "b.md"},
		ChildOrder: map[string][]string{"a.md": {"b.md"}, "b.md": {"c.md"}},
		NoteTitles: map[string]string{"a.md": "Alpha", "b.md": "Beta", "c.md": "Gamma"},
		NoteTags:   map[string][]string{"b.md": {"secret", "work"}, "c.md": {"public"}},
	}

	t.Run("disabled policy always returns read", func(t *testing.T) {
		p := MCPPolicy{Enabled: false, DefaultAccess: MCPAccessDeny}
		got := evaluateMCPAccess("b.md", "Beta", []string{"secret"}, p, st)
		assert.Equal(t, MCPAccessRead, got, "disabled policy must always grant read")
	})

	t.Run("default access used when no rule matches", func(t *testing.T) {
		p := MCPPolicy{Enabled: true, DefaultAccess: MCPAccessDeny}
		got := evaluateMCPAccess("a.md", "Alpha", nil, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("empty default access falls back to read", func(t *testing.T) {
		p := MCPPolicy{Enabled: true, DefaultAccess: ""}
		got := evaluateMCPAccess("a.md", "Alpha", nil, p, st)
		assert.Equal(t, MCPAccessRead, got)
	})

	t.Run("rule: tag match grants configured access", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{Tag: "secret", Access: MCPAccessDeny}},
		}
		got := evaluateMCPAccess("b.md", "Beta", []string{"secret", "work"}, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("rule: tag no match falls to default", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{Tag: "secret", Access: MCPAccessDeny}},
		}
		got := evaluateMCPAccess("c.md", "Gamma", []string{"public"}, p, st)
		assert.Equal(t, MCPAccessRead, got)
	})

	t.Run("rule: note_id exact match", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{NoteID: "a.md", Access: MCPAccessWrite}},
		}
		got := evaluateMCPAccess("a.md", "Alpha", nil, p, st)
		assert.Equal(t, MCPAccessWrite, got)
	})

	t.Run("rule: note_id does not match different note", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{NoteID: "a.md", Access: MCPAccessWrite}},
		}
		got := evaluateMCPAccess("b.md", "Beta", nil, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("rule: title_glob wildcard match", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{TitleGlob: "B*", Access: MCPAccessRead}},
		}
		got := evaluateMCPAccess("b.md", "Beta", nil, p, st)
		assert.Equal(t, MCPAccessRead, got)
	})

	t.Run("rule: title_glob no match", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{TitleGlob: "Z*", Access: MCPAccessRead}},
		}
		got := evaluateMCPAccess("a.md", "Alpha", nil, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("rule: subtree_of direct child", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{SubtreeOf: "a.md", Access: MCPAccessDeny}},
		}
		// b.md is a direct child of a.md
		got := evaluateMCPAccess("b.md", "Beta", nil, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("rule: subtree_of deep descendant", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{SubtreeOf: "a.md", Access: MCPAccessDeny}},
		}
		// c.md is a grandchild of a.md (via b.md)
		got := evaluateMCPAccess("c.md", "Gamma", nil, p, st)
		assert.Equal(t, MCPAccessDeny, got)
	})

	t.Run("rule: subtree_of root itself is not a descendant", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{SubtreeOf: "a.md", Access: MCPAccessDeny}},
		}
		// a.md itself is the root of the subtree — not a descendant
		got := evaluateMCPAccess("a.md", "Alpha", nil, p, st)
		assert.Equal(t, MCPAccessRead, got)
	})

	t.Run("first-match-wins ordering", func(t *testing.T) {
		p := MCPPolicy{
			Enabled: true,
			Rules: []MCPRule{
				{Tag: "secret", Access: MCPAccessDeny},
				{NoteID: "b.md", Access: MCPAccessWrite}, // would match, but tag rule fires first
			},
		}
		got := evaluateMCPAccess("b.md", "Beta", []string{"secret"}, p, st)
		assert.Equal(t, MCPAccessDeny, got, "first matching rule must win")
	})
}

// ── Subtree BFS cycle guard ───────────────────────────────────────────────────

func TestMCPIsDescendantOfCycle(t *testing.T) {
	// Construct a cyclic child structure (malformed data guard)
	st := mcpStructure{
		ChildOrder: map[string][]string{
			"x.md": {"y.md"},
			"y.md": {"x.md"}, // cycle
		},
	}
	// Should not infinite-loop; c.md is not in the cycle
	got := mcpIsDescendantOf("c.md", "x.md", st)
	assert.False(t, got, "note outside cycle must not be found")
}

// ── Token management HTTP handlers ───────────────────────────────────────────

func TestMCPTokenManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newServer := func() (*Server, *gin.Engine) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		r := s.SetupRouter()
		return s, r
	}

	t.Run("generate token returns raw token and stores hash", func(t *testing.T) {
		s, r := newServer()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/mcp/token", nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var resp map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		rawToken := resp["token"]
		assert.NotEmpty(t, rawToken, "response must include raw token")
		assert.Len(t, rawToken, 48, "token should be 48 characters")

		// Verify the stored hash matches SHA-256 of the raw token
		sum := sha256.Sum256([]byte(rawToken))
		want := hex.EncodeToString(sum[:])
		s.Library.mu.Lock()
		got := s.Library.Config.MCPTokenHash
		s.Library.mu.Unlock()
		assert.Equal(t, want, got, "stored hash must match SHA-256 of raw token")
	})

	t.Run("revoke token clears the hash", func(t *testing.T) {
		s, r := newServer()

		// First generate
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/mcp/token", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		// Then revoke
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("DELETE", "/api/mcp/token", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		s.Library.mu.Lock()
		hash := s.Library.Config.MCPTokenHash
		s.Library.mu.Unlock()
		assert.Empty(t, hash, "MCPTokenHash must be empty after revoke")
	})

	t.Run("GET /api/config includes mcpTokenSet boolean", func(t *testing.T) {
		s, r := newServer()

		// Initially no token
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/config", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var cfg map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cfg))
		assert.Equal(t, false, cfg["mcpTokenSet"], "mcpTokenSet should be false initially")
		assert.Nil(t, cfg["mcpTokenHash"], "mcpTokenHash must not be in the response")

		// Generate a token
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("POST", "/api/mcp/token", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		// Now mcpTokenSet should be true
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/config", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cfg))
		assert.Equal(t, true, cfg["mcpTokenSet"], "mcpTokenSet must be true after token generation")

		// Ensure raw hash is never exposed
		s.Library.mu.Lock()
		hash := s.Library.Config.MCPTokenHash
		s.Library.mu.Unlock()
		assert.NotEmpty(t, hash)
		respBody := w.Body.String()
		assert.NotContains(t, respBody, hash, "MCPTokenHash must never appear in /api/config response")
	})
}

// ── MCP auth middleware ───────────────────────────────────────────────────────

func TestMCPAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newServerWithToken := func(rawToken string) *gin.Engine {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		if rawToken != "" {
			sum := sha256.Sum256([]byte(rawToken))
			lib.Config.MCPTokenHash = hex.EncodeToString(sum[:])
		}
		return s.SetupRouter()
	}

	t.Run("missing token returns 401", func(t *testing.T) {
		r := newServerWithToken("secret-token-abc123")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp/sse", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		r := newServerWithToken("correct-token")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp/sse", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("no token configured returns 503", func(t *testing.T) {
		r := newServerWithToken("") // no token configured
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp/sse", nil)
		req.Header.Set("Authorization", "Bearer anything")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// ── sanitizeMCPPolicy ─────────────────────────────────────────────────────────

func TestSanitizeMCPPolicy(t *testing.T) {
	t.Run("valid policy is unchanged", func(t *testing.T) {
		p := MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules: []MCPRule{
				{Tag: "work", Access: MCPAccessRead},
				{NoteID: "x.md", Access: MCPAccessWrite},
			},
		}
		sanitizeMCPPolicy(&p)
		assert.Equal(t, MCPAccessDeny, p.DefaultAccess)
		assert.Len(t, p.Rules, 2)
	})

	t.Run("invalid DefaultAccess is reset to read", func(t *testing.T) {
		p := MCPPolicy{DefaultAccess: "execute"}
		sanitizeMCPPolicy(&p)
		assert.Equal(t, MCPAccessRead, p.DefaultAccess)
	})

	t.Run("empty DefaultAccess is left empty (evaluateMCPAccess treats as read)", func(t *testing.T) {
		p := MCPPolicy{DefaultAccess: ""}
		sanitizeMCPPolicy(&p)
		assert.Equal(t, MCPAccessLevel(""), p.DefaultAccess)
	})

	t.Run("rules with invalid access level are dropped", func(t *testing.T) {
		p := MCPPolicy{
			Rules: []MCPRule{
				{Tag: "ok", Access: MCPAccessRead},
				{Tag: "bad", Access: "superuser"},
				{NoteID: "x.md", Access: MCPAccessWrite},
			},
		}
		sanitizeMCPPolicy(&p)
		require.Len(t, p.Rules, 2)
		assert.Equal(t, "ok", p.Rules[0].Tag)
		assert.Equal(t, "x.md", p.Rules[1].NoteID)
	})

	t.Run("empty rules list is safe", func(t *testing.T) {
		p := MCPPolicy{Enabled: true}
		sanitizeMCPPolicy(&p)
		assert.Empty(t, p.Rules)
	})
}

// ── callMCPTool parameter validation ─────────────────────────────────────────

func TestMCPToolParamValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	s := &Server{Library: lib}
	rawID := json.RawMessage(`1`)

	t.Run("malformed JSON returns -32602", func(t *testing.T) {
		resp := s.callMCPTool(&rawID, "read_note", json.RawMessage(`{bad json`))
		require.NotNil(t, resp)
		require.NotNil(t, resp.Error)
		assert.Equal(t, -32602, resp.Error.Code)
	})

	t.Run("invalid search_in value returns -32602", func(t *testing.T) {
		resp := s.callMCPTool(&rawID, "search_notes", json.RawMessage(`{"query":"x","search_in":"full_text"}`))
		require.NotNil(t, resp)
		require.NotNil(t, resp.Error)
		assert.Equal(t, -32602, resp.Error.Code)
	})

	t.Run("valid search_in 'title' is accepted", func(t *testing.T) {
		resp := s.callMCPTool(&rawID, "search_notes", json.RawMessage(`{"query":"x","search_in":"title"}`))
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error, "valid search_in should not return a protocol error")
	})

	t.Run("unknown tool returns -32601", func(t *testing.T) {
		resp := s.callMCPTool(&rawID, "nonexistent_tool", json.RawMessage(`{}`))
		require.NotNil(t, resp)
		require.NotNil(t, resp.Error)
		assert.Equal(t, -32601, resp.Error.Code)
	})
}

// ── sanitizeMCPPolicy: empty-condition rule rejection ─────────────────────────

func TestSanitizeMCPPolicyEmptyCondition(t *testing.T) {
	t.Run("rule with all empty condition fields is dropped", func(t *testing.T) {
		p := MCPPolicy{
			Enabled: true,
			Rules: []MCPRule{
				{Tag: "ok", Access: MCPAccessDeny},
				{Access: MCPAccessDeny}, // all condition fields empty — no-op
			},
		}
		sanitizeMCPPolicy(&p)
		require.Len(t, p.Rules, 1)
		assert.Equal(t, "ok", p.Rules[0].Tag)
	})

	t.Run("rule with only one non-empty condition field is kept", func(t *testing.T) {
		p := MCPPolicy{
			Rules: []MCPRule{
				{NoteID: "note.md", Access: MCPAccessRead},
			},
		}
		sanitizeMCPPolicy(&p)
		require.Len(t, p.Rules, 1)
	})

	t.Run("multiple no-condition rules are all dropped", func(t *testing.T) {
		p := MCPPolicy{
			Rules: []MCPRule{
				{Access: MCPAccessDeny},
				{Access: MCPAccessWrite},
				{Access: MCPAccessRead},
			},
		}
		sanitizeMCPPolicy(&p)
		assert.Empty(t, p.Rules)
	})
}

// Canonical note filenames for tests: 8-digit date prefix + 16 lowercase alphanumeric chars.
// Plain names like "note.md" are rejected by IsValidName (which guards path-traversal).
const (
	noteA = "20240101aaaaaaaaaaaaaaaa.md" // "note A" / public / visible
	noteB = "20240101bbbbbbbbbbbbbbbb.md" // "note B" / secret / hidden
	noteC = "20240101cccccccccccccccc.md" // "note C" / encrypted
	noteD = "20240101dddddddddddddddd.md" // "note D" / writable
)

// ── loadMCPStructure ──────────────────────────────────────────────────────────

func TestLoadMCPStructure(t *testing.T) {
	newLib := func() *NoteLibrary {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		return lib
	}

	t.Run("missing _structure.json returns empty struct", func(t *testing.T) {
		lib := newLib()
		st := lib.loadMCPStructure()
		assert.Empty(t, st.Order)
		assert.Nil(t, st.NoteTitles)
	})

	t.Run("valid JSON is parsed correctly", func(t *testing.T) {
		lib := newLib()
		raw := `{"order":["` + noteA + `"],"titles":{"` + noteA + `":"Alpha"},"tags":{"` + noteA + `":["tag1"]}}`
		require.NoError(t, os.WriteFile(lib.FullPath("_structure.json"), []byte(raw), 0600))
		st := lib.loadMCPStructure()
		assert.Equal(t, []string{noteA}, st.Order)
		assert.Equal(t, "Alpha", st.NoteTitles[noteA])
		assert.Equal(t, []string{"tag1"}, st.NoteTags[noteA])
	})

	t.Run("ENC1 prefix returns empty struct without error", func(t *testing.T) {
		lib := newLib()
		require.NoError(t, os.WriteFile(lib.FullPath("_structure.json"), []byte("ENC1:somedata"), 0600))
		st := lib.loadMCPStructure()
		assert.Empty(t, st.Order)
		assert.Nil(t, st.NoteTitles)
	})

	t.Run("malformed JSON returns empty struct without panic", func(t *testing.T) {
		lib := newLib()
		require.NoError(t, os.WriteFile(lib.FullPath("_structure.json"), []byte("{bad json"), 0600))
		st := lib.loadMCPStructure()
		assert.Empty(t, st.Order)
	})
}

// ── mcpReadNote access control ────────────────────────────────────────────────

func TestMCPReadNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	newServerWithNote := func(filename, content string) *Server {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		if filename != "" {
			_ = os.WriteFile(lib.FullPath(filename), []byte(content), 0600)
		}
		return NewServer(lib)
	}

	t.Run("deny policy returns note-not-found (hides existence)", func(t *testing.T) {
		s := newServerWithNote(noteA, "top secret content")
		s.Library.mu.Lock()
		s.Library.Config.MCPPolicy = MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
		}
		s.Library.mu.Unlock()
		resp := s.mcpReadNote(&rawID, noteA)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
		assert.NotContains(t, string(body), "top secret")
	})

	t.Run("ENC1 content returns encryption notice instead of ciphertext", func(t *testing.T) {
		s := newServerWithNote(noteC, "ENC1:aabbccddeeff")
		resp := s.mcpReadNote(&rawID, noteC)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "end-to-end encrypted")
		assert.NotContains(t, string(body), "aabbccddeeff")
	})

	t.Run("missing file returns note-not-found", func(t *testing.T) {
		s := newServerWithNote("", "")
		resp := s.mcpReadNote(&rawID, noteA)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("invalid filename returns note-not-found", func(t *testing.T) {
		s := newServerWithNote("", "")
		resp := s.mcpReadNote(&rawID, "../escape.md")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("readable note returns full content", func(t *testing.T) {
		s := newServerWithNote(noteA, "# Hello\nWorld")
		resp := s.mcpReadNote(&rawID, noteA)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "Hello")
	})
}

// ── mcpUpdateNote access control ──────────────────────────────────────────────

func TestMCPUpdateNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	newServerWithNote := func(filename, content string, policy MCPPolicy) *Server {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		if filename != "" {
			_ = os.WriteFile(lib.FullPath(filename), []byte(content), 0600)
		}
		lib.Config.MCPPolicy = policy
		s := NewServer(lib)
		return s
	}

	t.Run("deny policy hides existence of note", func(t *testing.T) {
		s := newServerWithNote(noteD, "old", MCPPolicy{
			Enabled: true, DefaultAccess: MCPAccessDeny,
		})
		resp := s.mcpUpdateNote(&rawID, noteD, "new content")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
		// Verify file was not changed
		data, _ := os.ReadFile(s.Library.FullPath(noteD))
		assert.Equal(t, "old", string(data))
	})

	t.Run("read-only access returns not-found (no write permission)", func(t *testing.T) {
		s := newServerWithNote(noteD, "old", MCPPolicy{
			Enabled: true, DefaultAccess: MCPAccessRead,
		})
		resp := s.mcpUpdateNote(&rawID, noteD, "new content")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
		data, _ := os.ReadFile(s.Library.FullPath(noteD))
		assert.Equal(t, "old", string(data))
	})

	t.Run("nonexistent file returns note-not-found even with write policy", func(t *testing.T) {
		s := newServerWithNote("", "", MCPPolicy{
			Enabled: true, DefaultAccess: MCPAccessWrite,
		})
		resp := s.mcpUpdateNote(&rawID, noteA, "content")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("write access saves note successfully", func(t *testing.T) {
		s := newServerWithNote(noteD, "old", MCPPolicy{
			Enabled: true, DefaultAccess: MCPAccessWrite,
		})
		resp := s.mcpUpdateNote(&rawID, noteD, "updated content")
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "updated")
		data, _ := os.ReadFile(s.Library.FullPath(noteD))
		assert.Equal(t, "updated content", string(data))
	})

	t.Run("invalid filename is rejected", func(t *testing.T) {
		s := newServerWithNote("", "", MCPPolicy{Enabled: true, DefaultAccess: MCPAccessWrite})
		resp := s.mcpUpdateNote(&rawID, "../escape.md", "content")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})
}

// ── mcpListNotes access control ───────────────────────────────────────────────

func TestMCPListNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	newServerWithNotes := func(notes map[string]string, policy MCPPolicy) *Server {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		for name, content := range notes {
			_ = os.WriteFile(lib.FullPath(name), []byte(content), 0600)
		}
		lib.Config.MCPPolicy = policy
		return NewServer(lib)
	}

	t.Run("denied notes are excluded from listing", func(t *testing.T) {
		s := newServerWithNotes(map[string]string{
			noteA: "# Public Note",
			noteB: "# Secret Note",
		}, MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{NoteID: noteA, Access: MCPAccessRead}},
		})
		resp := s.mcpListNotes(&rawID)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA)
		assert.NotContains(t, bodyStr, noteB)
	})

	t.Run("all notes visible when policy disabled", func(t *testing.T) {
		s := newServerWithNotes(map[string]string{
			noteA: "Content A",
			noteB: "Content B",
		}, MCPPolicy{Enabled: false})
		resp := s.mcpListNotes(&rawID)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA)
		assert.Contains(t, bodyStr, noteB)
	})
}

// ── write-tool gating (mcpToolDefs) ──────────────────────────────────────────

func TestMCPWriteToolGating(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newServerWithPolicy := func(policy MCPPolicy) *Server {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MCPPolicy = policy
		return NewServer(lib)
	}

	toolNames := func(s *Server) []string {
		tools := mcpToolDefs(s)
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			if n, ok := tool["name"].(string); ok {
				names = append(names, n)
			}
		}
		return names
	}

	t.Run("update_note advertised when policy disabled (default)", func(t *testing.T) {
		s := newServerWithPolicy(MCPPolicy{Enabled: false})
		names := toolNames(s)
		assert.Contains(t, names, "update_note")
	})

	t.Run("update_note advertised when default access is write", func(t *testing.T) {
		s := newServerWithPolicy(MCPPolicy{Enabled: true, DefaultAccess: MCPAccessWrite})
		names := toolNames(s)
		assert.Contains(t, names, "update_note")
	})

	t.Run("update_note advertised when a write rule exists", func(t *testing.T) {
		s := newServerWithPolicy(MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
			Rules:         []MCPRule{{Tag: "editable", Access: MCPAccessWrite}},
		})
		names := toolNames(s)
		assert.Contains(t, names, "update_note")
	})

	t.Run("update_note hidden when policy enabled with no write rule and default read", func(t *testing.T) {
		s := newServerWithPolicy(MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessRead,
		})
		names := toolNames(s)
		assert.NotContains(t, names, "update_note")
	})

	t.Run("update_note hidden when policy enabled with deny default and no write rule", func(t *testing.T) {
		s := newServerWithPolicy(MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
		})
		names := toolNames(s)
		assert.NotContains(t, names, "update_note")
	})
}

// ── JSON-RPC dispatch (dispatchMCP) ──────────────────────────────────────────

func TestMCPDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newServer := func() *Server {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		return NewServer(lib)
	}
	idRaw := json.RawMessage(`1`)

	t.Run("initialize returns server info", func(t *testing.T) {
		s := newServer()
		params := json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"}}`)
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", ID: &idRaw, Method: "initialize", Params: params})
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "YinMoNote")
	})

	t.Run("ping returns empty result", func(t *testing.T) {
		s := newServer()
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", ID: &idRaw, Method: "ping"})
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("tools/list returns tool definitions", func(t *testing.T) {
		s := newServer()
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", ID: &idRaw, Method: "tools/list"})
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "list_notes")
		assert.Contains(t, string(body), "read_note")
	})

	t.Run("unknown method returns -32601", func(t *testing.T) {
		s := newServer()
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", ID: &idRaw, Method: "unknown/method"})
		require.NotNil(t, resp)
		require.NotNil(t, resp.Error)
		assert.Equal(t, -32601, resp.Error.Code)
	})

	t.Run("notifications/initialized returns nil (no response expected)", func(t *testing.T) {
		s := newServer()
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
		assert.Nil(t, resp, "notifications must not produce a response")
	})

	t.Run("tools/call with valid tool dispatches correctly", func(t *testing.T) {
		s := newServer()
		params := json.RawMessage(`{"name":"list_notes","arguments":{}}`)
		resp := s.dispatchMCP(mcpRequest{JSONRPC: "2.0", ID: &idRaw, Method: "tools/call", Params: params})
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

// ── handleMCPCAFingerprint HTTP handler ───────────────────────────────────────

func TestMCPCAFingerprintHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("no CA cert configured returns 404", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		s.CACert = nil
		r := s.SetupRouter()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/mcp/ca-fingerprint", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ── mcpSearchNotes access control ─────────────────────────────────────────────

func TestMCPSearchNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	t.Run("denied notes are excluded from search results", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Visible Alpha"), 0600)
		_ = os.WriteFile(lib.FullPath(noteB), []byte("# Hidden Alpha"), 0600)
		lib.Config.MCPPolicy = MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{NoteID: noteA, Access: MCPAccessRead}},
		}
		s := NewServer(lib)

		resp := s.mcpSearchNotes(&rawID, "Alpha", "title")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA)
		assert.NotContains(t, bodyStr, noteB)
	})

	t.Run("empty query returns error result", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		resp := s.mcpSearchNotes(&rawID, "", "title")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		// errResult sets isError=true
		assert.True(t, strings.Contains(string(body), "isError") || strings.Contains(string(body), "empty"))
	})

	t.Run("search_in=tag matches notes by tag", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Work Note"), 0600)
		_ = os.WriteFile(lib.FullPath(noteB), []byte("# Personal Note"), 0600)
		// Provide tag metadata via _structure.json
		raw := `{"order":["` + noteA + `","` + noteB + `"],"titles":{"` + noteA + `":"Work Note","` + noteB + `":"Personal Note"},"tags":{"` + noteA + `":["work","project"],"` + noteB + `":["personal"]}}`
		_ = os.WriteFile(lib.FullPath("_structure.json"), []byte(raw), 0600)
		s := NewServer(lib)

		resp := s.mcpSearchNotes(&rawID, "work", "tag")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA, "note with 'work' tag should appear")
		assert.NotContains(t, bodyStr, noteB, "note without 'work' tag should not appear")
	})

	t.Run("search_in=both matches on title or tag", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Alpha Study"), 0600)
		_ = os.WriteFile(lib.FullPath(noteB), []byte("# Unrelated"), 0600)
		raw := `{"order":["` + noteA + `","` + noteB + `"],"titles":{"` + noteA + `":"Alpha Study","` + noteB + `":"Unrelated"},"tags":{"` + noteB + `":["alpha"]}}`
		_ = os.WriteFile(lib.FullPath("_structure.json"), []byte(raw), 0600)
		s := NewServer(lib)

		// "alpha" matches noteA by title AND noteB by tag
		resp := s.mcpSearchNotes(&rawID, "alpha", "both")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA, "title match should appear in 'both' mode")
		assert.Contains(t, bodyStr, noteB, "tag match should appear in 'both' mode")
	})
}

// ── mcpGetStructure access control ────────────────────────────────────────────

func TestMCPGetStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	t.Run("denied notes are excluded from structure", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Public"), 0600)
		_ = os.WriteFile(lib.FullPath(noteB), []byte("# Secret"), 0600)
		// noteB is in the structure but has no NoteTitles entry — this was the bug
		raw := `{"order":["` + noteA + `","` + noteB + `"],"titles":{"` + noteA + `":"Public"}}`
		_ = os.WriteFile(lib.FullPath("_structure.json"), []byte(raw), 0600)
		lib.Config.MCPPolicy = MCPPolicy{
			Enabled:       true,
			DefaultAccess: MCPAccessDeny,
			Rules:         []MCPRule{{NoteID: noteA, Access: MCPAccessRead}},
		}
		s := NewServer(lib)

		resp := s.mcpGetStructure(&rawID)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		body, _ := json.Marshal(resp.Result)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, noteA, "allowed note should be in structure")
		assert.NotContains(t, bodyStr, noteB, "denied note must not appear in structure")
	})

	t.Run("all notes visible when policy disabled", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Note A"), 0600)
		raw := `{"order":["` + noteA + `"],"titles":{"` + noteA + `":"Note A"}}`
		_ = os.WriteFile(lib.FullPath("_structure.json"), []byte(raw), 0600)
		s := NewServer(lib)

		resp := s.mcpGetStructure(&rawID)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), noteA)
	})

	t.Run("encrypted structure returns empty-library message", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath("_structure.json"), []byte("ENC1:encrypted"), 0600)
		s := NewServer(lib)

		resp := s.mcpGetStructure(&rawID)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "empty")
	})
}

// ── mcpGetHistory access control ──────────────────────────────────────────────

func TestMCPGetHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)

	t.Run("deny policy returns note-not-found (hides history existence)", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Secret"), 0600)
		lib.Config.MCPPolicy = MCPPolicy{Enabled: true, DefaultAccess: MCPAccessDeny}
		s := NewServer(lib)

		resp := s.mcpGetHistory(&rawID, noteA, 10)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("invalid filename returns note-not-found", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)

		resp := s.mcpGetHistory(&rawID, "../escape.md", 10)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("readable note with no history returns informational message", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Note"), 0600)
		s := NewServer(lib)

		resp := s.mcpGetHistory(&rawID, noteA, 10)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "No history")
	})
}

// ── mcpReadVersion access control ─────────────────────────────────────────────

func TestMCPReadVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rawID := json.RawMessage(`1`)
	validHash := strings.Repeat("a", 40) // 40-char hex hash

	t.Run("invalid git hash format returns error", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Note"), 0600)
		s := NewServer(lib)

		resp := s.mcpReadVersion(&rawID, noteA, "not-a-hash")
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "invalid git hash")
	})

	t.Run("deny policy returns note-not-found", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Secret"), 0600)
		lib.Config.MCPPolicy = MCPPolicy{Enabled: true, DefaultAccess: MCPAccessDeny}
		s := NewServer(lib)

		resp := s.mcpReadVersion(&rawID, noteA, validHash)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("invalid filename is rejected", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)

		resp := s.mcpReadVersion(&rawID, "../escape.md", validHash)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "not found")
	})

	t.Run("valid hash for non-existent version returns version-not-found", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		_ = os.WriteFile(lib.FullPath(noteA), []byte("# Note"), 0600)
		s := NewServer(lib)

		resp := s.mcpReadVersion(&rawID, noteA, validHash)
		require.NotNil(t, resp)
		body, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(body), "version not found")
	})
}
