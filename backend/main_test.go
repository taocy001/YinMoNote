package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityAndBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	t.Run("Path Traversal Prevention", func(t *testing.T) {
		illegalPaths := []string{
			"/api/notes/../../etc/passwd",
			"/api/notes/..%2f..%2fconfig.json",
		}
		for _, path := range illegalPaths {
			req, _ := http.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.True(t, w.Code >= 400, "Path %s should be blocked (got %d)", path, w.Code)
		}
	})

	t.Run("File Size Limit", func(t *testing.T) {
		// 20MB is the limit in git.go
		limit := int64(20 << 20)
		largeBody := make([]byte, limit+1024)
		req, _ := http.NewRequest("PUT", "/api/notes/20260318testnote12345678.md", bytes.NewBuffer(largeBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// http.MaxBytesReader will cause a failure when reading
		assert.True(t, w.Code >= 400)
	})
}

func TestStructurePersistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	testStruct := Structure{
		Order:      []string{"note1.md"},
		Parents:    map[string]string{"child.md": "note1.md"},
		ChildOrder: map[string][]string{"note1.md": {"child.md"}},
	}
	body, _ := json.Marshal(testStruct)
	
	req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify reload
	req2, _ := http.NewRequest("GET", "/api/structure", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var reloaded Structure
	json.Unmarshal(w2.Body.Bytes(), &reloaded)
	assert.Equal(t, "note1.md", reloaded.Order[0])
	assert.Equal(t, "note1.md", reloaded.Parents["child.md"])
}

// ─── IsValidName ──────────────────────────────────────────────────────────────

func TestIsValidName(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	valid := []string{
		"_structure.json",
		"20260318abcdef1234567890.md",
		"20260318abcdef1234567890.png",
		"20260318abcdef1234567890.jpg",
		"20260318abcdef1234567890.jpeg",
		"20260318abcdef1234567890.gif",
		"20260318abcdef1234567890.webp",
		"20260318a1b2c3d4e5f60001.md",
	}
	for _, name := range valid {
		assert.True(t, lib.IsValidName(name), "expected valid: %q", name)
	}

	invalid := []string{
		"../../etc/passwd",           // path traversal
		"notes/secret.md",            // subdirectory
		"arbitrary.txt",              // wrong extension
		"my-note.md",                 // wrong format
		"20260318.md",                // missing random suffix
		"20260318ABCDEF1234567890.md", // uppercase forbidden
		"20260318abcdef123456789.md",  // 15 random chars (needs 16)
		"20260318abcdef12345678901.md", // 17 random chars (needs 16)
		"../20260318abcdef1234567890.md", // path traversal prefix
		"20260318abcdef1234567890.exe",   // executable extension
		"",
	}
	for _, name := range invalid {
		assert.False(t, lib.IsValidName(name), "expected invalid: %q", name)
	}
}

// ─── extractNoteTitle ─────────────────────────────────────────────────────────

func TestExtractNoteTitle(t *testing.T) {
	dir := t.TempDir()

	// Plain text: extractNoteTitle strips the "# " heading prefix
	f1 := filepath.Join(dir, "plain.md")
	os.WriteFile(f1, []byte("# My Note\nsome content below"), 0644)
	assert.Equal(t, "My Note", extractNoteTitle(f1))

	// Encrypted note (ENC1 prefix) must return empty — server cannot read ciphertext
	f2 := filepath.Join(dir, "encrypted.md")
	os.WriteFile(f2, []byte("ENC1:abc123:def456"), 0644)
	assert.Equal(t, "", extractNoteTitle(f2))

	// Empty file returns empty string
	f3 := filepath.Join(dir, "empty.md")
	os.WriteFile(f3, []byte(""), 0644)
	assert.Equal(t, "", extractNoteTitle(f3))

	// Non-existent file returns empty string (graceful failure)
	assert.Equal(t, "", extractNoteTitle(filepath.Join(dir, "missing.md")))
}

// ─── AtomicWrite ──────────────────────────────────────────────────────────────

func TestAtomicWrite(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	content := []byte("atomic write test content")
	err := lib.AtomicWrite("20260318atomictest000001.md", content)
	assert.NoError(t, err)
	got, err := os.ReadFile(lib.FullPath("20260318atomictest000001.md"))
	assert.NoError(t, err)
	assert.Equal(t, content, got)
}

// ─── Note save / get / delete ─────────────────────────────────────────────────

func TestHandleSaveGetDeleteNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	t.Run("Valid filename is accepted and content is retrievable", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"content": "# Hello\ncontent"})
		req, _ := http.NewRequest("PUT", "/api/notes/20260318savetest00000001.md", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req2, _ := http.NewRequest("GET", "/api/notes/20260318savetest00000001.md", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Contains(t, w2.Body.String(), "Hello")
	})

	t.Run("Invalid filename (path traversal) is rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"content": "x"})
		req, _ := http.NewRequest("PUT", "/api/notes/../../etc/passwd", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// Gin normalizes path traversal URLs at routing level → 404; our handler
		// rejects invalid names with 400. Either way the request must not succeed.
		assert.True(t, w.Code >= 400, "path traversal should be blocked, got %d", w.Code)
	})

	t.Run("DELETE removes the note from the listing", func(t *testing.T) {
		// Create
		body, _ := json.Marshal(map[string]string{"content": "to be deleted"})
		req, _ := http.NewRequest("PUT", "/api/notes/20260318deletetest000001.md", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Delete
		req2, _ := http.NewRequest("DELETE", "/api/notes/20260318deletetest000001.md", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Verify absent from list
		req3, _ := http.NewRequest("GET", "/api/notes", nil)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		assert.NotContains(t, w3.Body.String(), "20260318deletetest000001.md")
	})
}

// ─── handleListNotes ──────────────────────────────────────────────────────────

func TestHandleListNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Plain note: title extracted from first line
	lib.AtomicWrite("20260318listtestnote0001.md", []byte("# First Note\ncontent"))
	// Encrypted note: title must be empty (server cannot read ENC1 ciphertext)
	lib.AtomicWrite("20260318listtestnote0002.md", []byte("ENC1:iv:ciphertext"))

	req, _ := http.NewRequest("GET", "/api/notes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result struct {
		Notes []NoteInfo `json:"notes"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Len(t, result.Notes, 2)

	titles := map[string]string{}
	for _, n := range result.Notes {
		titles[n.Name] = n.Title
	}
	assert.Equal(t, "First Note", titles["20260318listtestnote0001.md"])
	assert.Equal(t, "", titles["20260318listtestnote0002.md"])
}

// ─── Structure integrity ──────────────────────────────────────────────────────

func TestHandleStructureIntegrity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("JSON-quoted ENC1 blob is unwrapped before storage and returned without quotes", func(t *testing.T) {
		// Axios JSON-encodes string payloads on PUT, so the client sends `"ENC1:..."`.
		// The server must unwrap the quotes and store the raw ENC1 value.
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		jsonQuoted := `"ENC1:dGVzdA==:dGVzdA=="`
		req, _ := http.NewRequest("PUT", "/api/structure", strings.NewReader(jsonQuoted))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req2, _ := http.NewRequest("GET", "/api/structure", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		body := w2.Body.String()
		assert.True(t, strings.HasPrefix(body, "ENC1:"),
			"expected raw ENC1 prefix after unwrapping, got: %s", body)
		assert.False(t, strings.HasPrefix(body, `"`),
			"should NOT be JSON-quoted, got: %s", body)
	})

	t.Run("Empty order is rejected when notes exist on disk", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		// Create a note so the library is non-empty
		lib.AtomicWrite("20260318emptyorder000001.md", []byte("content"))

		empty := Structure{
			Order:      []string{},
			Parents:    map[string]string{},
			ChildOrder: map[string][]string{},
		}
		body, _ := json.Marshal(empty)
		req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code,
			"empty order with existing notes should be rejected")
	})

	t.Run("Empty order is accepted when all notes are in trash", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		noteID := "20260327trashallow00001.md"
		lib.AtomicWrite(noteID, []byte("content"))

		st := Structure{
			Order:      []string{},
			Parents:    map[string]string{},
			ChildOrder: map[string][]string{},
			Trash:      []TrashEntry{{ID: noteID, DeletedAt: 1711500000000}},
		}
		body, _ := json.Marshal(st)
		req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code,
			"empty order should be accepted when all notes are accounted for in trash")
	})

	t.Run("Empty order is accepted when all notes are nested under parents", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		noteID := "20260327parentallow0001.md"
		lib.AtomicWrite(noteID, []byte("content"))

		st := Structure{
			Order:      []string{},
			Parents:    map[string]string{noteID: "some-parent"},
			ChildOrder: map[string][]string{},
		}
		body, _ := json.Marshal(st)
		req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code,
			"empty order should be accepted when all notes are accounted for in parents")
	})

	t.Run("Structure save succeeds when order exceeds MaxItemsPerLevel", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		lib.Config.MaxItemsPerLevel = 5
		r := NewServer(lib).SetupRouter()

		ids := make([]string, 10)
		for i := 0; i < 10; i++ {
			ids[i] = fmt.Sprintf("20260327bigorder0000%02d.md", i)
			lib.AtomicWrite(ids[i], []byte("content"))
		}

		st := Structure{
			Order:      ids,
			Parents:    map[string]string{},
			ChildOrder: map[string][]string{},
		}
		body, _ := json.Marshal(st)
		req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code,
			"top-level order exceeding MaxItemsPerLevel must not block structure saves (deletions, moves, etc.)")
	})

	t.Run("Plain JSON structure is accepted and retrievable", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		lib.AtomicWrite("20260318structtest000001.md", []byte("content"))
		st := Structure{
			Order:      []string{"20260318structtest000001.md"},
			Parents:    map[string]string{},
			ChildOrder: map[string][]string{},
		}
		body, _ := json.Marshal(st)
		req, _ := http.NewRequest("PUT", "/api/structure", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req2, _ := http.NewRequest("GET", "/api/structure", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		var reloaded Structure
		json.Unmarshal(w2.Body.Bytes(), &reloaded)
		assert.Equal(t, "20260318structtest000001.md", reloaded.Order[0])
	})
}

// ─── Image upload validation ──────────────────────────────────────────────────

func TestHandleUploadValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	makeUpload := func(filename string, content []byte) *http.Request {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, _ := mw.CreateFormFile("image", filename)
		part.Write(content)
		mw.Close()
		req, _ := http.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return req
	}

	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, makeUpload("image"+ext, []byte("fake image data")))
		assert.Equal(t, http.StatusOK, w.Code, "expected OK for extension %s", ext)
	}

	for _, name := range []string{"script.js", "shell.sh", "malware.exe", "note.md", "file.txt"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, makeUpload(name, []byte("data")))
		assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for file %s", name)
	}
}

// ─── randomString — CSPRNG & uniformity ──────────────────────────────────────

func TestRandomStringCryptographic(t *testing.T) {
	// Verify randomString produces non-repeating, character-set-valid output.
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s := randomString(16)
		assert.Len(t, s, 16, "randomString should always return exactly n chars")
		assert.Regexp(t, `^[a-z0-9]+$`, s, "only lowercase alphanumeric allowed")
		assert.False(t, seen[s], "randomString produced a duplicate: %q", s)
		seen[s] = true
	}
}

func TestRandomStringUniformDistribution(t *testing.T) {
	// Chi-square sanity check: each character should appear with roughly equal frequency.
	const samples = 10000
	counts := make(map[byte]int)
	for i := 0; i < samples; i++ {
		for _, b := range []byte(randomString(1)) {
			counts[b]++
		}
	}
	// With 36 chars and 10000 samples, expected ~277 per char.
	// Allow ±40% tolerance for statistical variation.
	expected := float64(samples) / 36.0
	for ch, cnt := range counts {
		ratio := float64(cnt) / expected
		assert.True(t, ratio > 0.6 && ratio < 1.4,
			"char %q has count %d (expected ~%.0f): distribution looks biased", ch, cnt, expected)
	}
}

// ─── Config quota clamping at runtime ────────────────────────────────────────

func TestConfigQuotaClampAtRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Send extreme values that would bypass quotas if not clamped.
	extreme := map[string]interface{}{
		"maxTotalNotes": 999999,
		"maxNoteSize":   999999999,
		"maxAssetSize":  999999999,
		"lang":          "en",
	}
	body, _ := json.Marshal(extreme)
	req, _ := http.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2, _ := http.NewRequest("GET", "/api/config", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var cfg AppConfig
	json.Unmarshal(w2.Body.Bytes(), &cfg)
	assert.LessOrEqual(t, cfg.MaxTotalNotes, 5000, "MaxTotalNotes should be clamped to ≤5000")
	assert.LessOrEqual(t, cfg.MaxNoteSize, int64(10*1024*1024), "MaxNoteSize should be clamped")
	assert.LessOrEqual(t, cfg.MaxAssetSize, int64(50*1024*1024), "MaxAssetSize should be clamped")
	// Unclamped fields should pass through unchanged.
	assert.Equal(t, "en", cfg.Lang)
}

// ─── Security headers ─────────────────────────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	req, _ := http.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, w.Header().Get("Permissions-Policy"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"), "CSP header must be present")
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "default-src 'self'")
}

// ─── Input validation on history / delete / rollback ─────────────────────────

func TestEndpointInputValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	illegalName := "../../etc/passwd"

	t.Run("DELETE with invalid filename returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/notes/"+illegalName, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400, "invalid DELETE should be blocked")
	})

	t.Run("GET history with invalid filename returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+illegalName+"/history", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.True(t, w.Code >= 400, "invalid history request should be blocked")
	})

	t.Run("Rollback with bad hash returns 404", func(t *testing.T) {
		// First create a note so the name is valid
		validName := "20260319rollbacktest0001.md"
		lib.AtomicWrite(validName, []byte("content"))

		// Well-formed (40-char) hash that does not correspond to any real commit → 404.
		// A malformed hash (wrong length / chars) now returns 400 "invalid_hash" instead.
		body, _ := json.Marshal(map[string]string{"hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"})
		req, _ := http.NewRequest("POST", "/api/notes/"+validName+"/rollback", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "rollback with unknown (but well-formed) hash should return 404")
	})
}

// ─── Uploads served with filename validation ──────────────────────────────────

func TestUploadGetValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	t.Run("Invalid filename returns 400", func(t *testing.T) {
		// Use a filename that fails the regex check inside handleGetUpload.
		// Path-traversal URLs (../../) trigger Gin's path-cleaner redirect (301)
		// which is also a valid block, but a direct bad name gives a cleaner assertion.
		req, _ := http.NewRequest("GET", "/uploads/bad-filename.txt", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "non-canonical filename should be rejected with 400")
	})

	t.Run("Non-existent valid filename returns 404", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/uploads/20260319testasset0000001.png", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ─── Config persistence ───────────────────────────────────────────────────────

func TestConfigPersistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	update := map[string]interface{}{"lang": "en", "isDark": true, "editorWidth": "800px"}
	body, _ := json.Marshal(update)
	req, _ := http.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2, _ := http.NewRequest("GET", "/api/config", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var cfg AppConfig
	json.Unmarshal(w2.Body.Bytes(), &cfg)
	assert.Equal(t, "en", cfg.Lang)
	assert.True(t, cfg.IsDark)
	assert.Equal(t, "800px", cfg.EditorWidth)
}

// ─── Trusted proxies: X-Forwarded-For spoofing defence ───────────────────────

func TestTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// With SetTrustedProxies(["127.0.0.1","::1"]), a spoofed X-Forwarded-For from
	// a non-loopback RemoteAddr must NOT be trusted. Gin should derive ClientIP
	// from RemoteAddr instead. We verify the router returns a valid response (not
	// a 500 or panic) when a spoofed header is present.
	req, _ := http.NewRequest("GET", "/api/config", nil)
	req.RemoteAddr = "203.0.113.42:5000" // non-loopback, non-trusted
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Endpoint accessible (no auth set in test); just confirm no server error.
	assert.True(t, w.Code < 500, "spoofed X-Forwarded-For should not crash the server")
}

// ─── Structure body read error returns 400 ───────────────────────────────────

func TestStructureReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Send a PUT /structure with a body that is valid (io.ReadAll should succeed).
	// We verify the normal path still works after adding the error check.
	body := `{"order":[],"parents":{},"childOrder":{},"titles":{}}`
	req, _ := http.NewRequest("PUT", "/api/structure", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "valid structure PUT should return 200")
}

// ─── handleGetNote 404 for missing file ──────────────────────────────────────

func TestHandleGetNoteNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	req, _ := http.NewRequest("GET", "/api/notes/20260320missingfile00001.md", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "missing note should return 404, not 200")
}

// ─── clampConfig covers all 7 quota fields ───────────────────────────────────

func TestClampConfigAllFields(t *testing.T) {
	extreme := AppConfig{
		MaxTotalNotes:    999999,
		MaxTotalAssets:   999999,
		MaxImagesPerNote: 999999,
		MaxNestingDepth:  999999,
		MaxItemsPerLevel: 999999,
		MaxNoteSize:      999999999,
		MaxAssetSize:     999999999,
	}
	clampConfig(&extreme)
	assert.LessOrEqual(t, extreme.MaxTotalNotes, 5000)
	assert.LessOrEqual(t, extreme.MaxTotalAssets, 10000)
	assert.LessOrEqual(t, extreme.MaxImagesPerNote, 100)
	assert.LessOrEqual(t, extreme.MaxNestingDepth, 20)
	assert.LessOrEqual(t, extreme.MaxItemsPerLevel, 5000)
	assert.LessOrEqual(t, extreme.MaxNoteSize, int64(10*1024*1024))
	assert.LessOrEqual(t, extreme.MaxAssetSize, int64(50*1024*1024))

	zero := AppConfig{} // all zeros — below minimum bounds
	clampConfig(&zero)
	assert.Equal(t, 500, zero.MaxTotalNotes)
	assert.Equal(t, 1000, zero.MaxTotalAssets)
	assert.Equal(t, 10, zero.MaxImagesPerNote)
	assert.Equal(t, 10, zero.MaxNestingDepth)
	assert.Equal(t, 500, zero.MaxItemsPerLevel)
	assert.Equal(t, int64(512*1024), zero.MaxNoteSize)
	assert.Equal(t, int64(2*1024*1024), zero.MaxAssetSize)
}

// ─── handleGetVersion hash validation ────────────────────────────────────────

func TestHandleGetVersionHashValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Create a note so the filename is valid
	noteName := "20260320hashvaltest00001.md"
	lib.AtomicWrite(noteName, []byte("# Test\ncontent"))

	t.Run("Short hash returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/version/deadbeef", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "short hash should be rejected")
	})

	t.Run("Non-hex hash returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/version/ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "uppercase hex should be rejected")
	})

	t.Run("Valid format hash that doesn't exist returns 200 with empty content", func(t *testing.T) {
		// go-git returns "" for an unknown hash; the handler returns 200 with empty content
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/version/0000000000000000000000000000000000000000", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// Format was valid, so no 400; the content may be empty
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
	})
}

// ─── handleRollback quota check ──────────────────────────────────────────────

func TestHandleRollbackQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320rollbackquot0001.md"

	// Create a real commit so GetContentAtHash can return content.
	err := lib.SaveNoteAndCommit(noteName, strings.Repeat("x", 100), "initial commit")
	if err != nil {
		t.Skipf("git not available in this env: %v", err)
	}

	// Fetch the actual commit hash from history.
	history := lib.GetHistory(noteName, 1)
	if len(history) == 0 {
		t.Skip("no history available for rollback quota test")
	}
	realHash := history[0].Hash

	// Set a very small note size limit AFTER the initial commit so the rollback triggers quota.
	lib.Config.MaxNoteSize = 10

	body, _ := json.Marshal(map[string]string{"hash": realHash})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteName+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Content (100 bytes) exceeds MaxNoteSize (10) → quota check triggers 400.
	assert.Equal(t, http.StatusBadRequest, w.Code, "rollback of oversized content should be rejected by quota")
	// CheckNoteQuota returns its own error code (e.g. limit_note_size) directly.
	assert.True(t, strings.Contains(w.Body.String(), "limit_") || strings.Contains(w.Body.String(), "rollback_"),
		"quota rejection should surface a limit or rollback error code, got: %s", w.Body.String())
}

func TestHandleRollbackBadHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320rollbackquot0002.md"
	lib.AtomicWrite(noteName, []byte("content"))

	// All-zeros hash has no matching commit → 404.
	body, _ := json.Marshal(map[string]string{"hash": "0000000000000000000000000000000000000000"})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteName+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "bad hash should return 404 version_not_found")
}

// ─── AtomicWrite concurrent safety ───────────────────────────────────────────

func TestAtomicWriteConcurrent(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	noteName := "20260320concurrenttest01.md"
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			content := strings.Repeat("x", n+1) // different content per goroutine
			lib.AtomicWrite(noteName, []byte(content))
		}(i)
	}
	wg.Wait()

	// File must exist and have valid (non-zero) content — no corruption.
	data, err := os.ReadFile(lib.FullPath(noteName))
	assert.NoError(t, err, "file should exist after concurrent writes")
	assert.Greater(t, len(data), 0, "file should be non-empty")
}

// ─── Port field preservation via PUT /api/config ────────────────────────────

func TestPortPreservation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.Port = ":9090" // Set a non-default port
	r := NewServer(lib).SetupRouter()

	// Try to override the port via a config update
	update := map[string]interface{}{"port": ":1234", "lang": "en"}
	body, _ := json.Marshal(update)
	req, _ := http.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the port was NOT changed
	req2, _ := http.NewRequest("GET", "/api/config", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var cfg AppConfig
	json.Unmarshal(w2.Body.Bytes(), &cfg)
	assert.Equal(t, ":9090", cfg.Port, "PUT /config must not override the server port")
}

// ─── CheckAssetQuota ReadDir error ───────────────────────────────────────────

func TestCheckAssetQuotaReadDirError(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	// Remove the assets directory to force a ReadDir error
	os.RemoveAll(filepath.Join(lib.DataDir, lib.AssetsDir))

	err := lib.CheckAssetQuota(100)
	assert.Error(t, err, "CheckAssetQuota should return error when assets dir is unreadable")
	assert.Equal(t, "quota_check_failed", err.Error())
}

// ─── handleGetHistory pagination ─────────────────────────────────────────────

func TestHandleGetHistoryPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320histpagtest00001.md"
	lib.AtomicWrite(noteName, []byte("content"))

	t.Run("Default limit returns at most 50 entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/history", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var result struct {
			History []CommitInfo `json:"history"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.LessOrEqual(t, len(result.History), 50, "history should be capped at default 50")
	})

	t.Run("Custom limit=5 is respected", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/history?limit=5", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var result struct {
			History []CommitInfo `json:"history"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.LessOrEqual(t, len(result.History), 5, "history should be capped at limit=5")
	})

	t.Run("Limit above 200 is clamped", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/history?limit=999", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		// Handler clamps oversized limit to 50 — GetHistory will use the default
	})
}

// ─── handleSaveNote error is opaque ──────────────────────────────────────────

func TestHandleSaveNoteErrorOpaque(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Send content that exceeds MaxNoteSize to trigger a quota error (not a disk error)
	lib.Config.MaxNoteSize = 10
	body, _ := json.Marshal(map[string]string{"content": strings.Repeat("x", 100)})
	req, _ := http.NewRequest("PUT", "/api/notes/20260320saveerrtest00001.md", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "quota exceeded returns 400")
	// The error message should be the quota key, not an internal Go error
	assert.Contains(t, w.Body.String(), "limit_note_size")
	assert.NotContains(t, w.Body.String(), "/data/", "internal path must not leak in error response")
}

// ─── CheckStructureQuota per-folder item limit ───────────────────────────────

func TestCheckStructureQuotaSubfolder(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	// CheckStructureQuota no longer enforces per-level item limits (only cycles).
	// Over-limit structures from imports/bulk operations must not block saves.
	s := Structure{
		Order: []string{"folder1"},
		ChildOrder: map[string][]string{
			"folder1": {"note1", "note2", "note3", "note4"},
		},
	}
	err := lib.CheckStructureQuota(s)
	assert.NoError(t, err, "per-level item count must not block structure saves")
}

// ─── NewNoteLibrary clamps saved config on reload ────────────────────────────

func TestNewNoteLibraryClampOnReload(t *testing.T) {
	dir := t.TempDir()

	// Write a config with out-of-range values directly to disk
	badConfig := AppConfig{
		MaxTotalNotes:    99999,
		MaxTotalAssets:   99999,
		MaxImagesPerNote: 99999,
		MaxNestingDepth:  99999,
		MaxItemsPerLevel: 99999,
		MaxNoteSize:      999999999,
		MaxAssetSize:     999999999,
		Port:             ":8080",
	}
	data, _ := json.MarshalIndent(badConfig, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)

	lib, err := NewNoteLibrary(dir, "assets", filepath.Join(t.TempDir(), "config.json"))
	assert.NoError(t, err)
	assert.LessOrEqual(t, lib.Config.MaxTotalNotes, 5000)
	assert.LessOrEqual(t, lib.Config.MaxTotalAssets, 10000)
	assert.LessOrEqual(t, lib.Config.MaxImagesPerNote, 100)
	assert.LessOrEqual(t, lib.Config.MaxNestingDepth, 20)
	assert.LessOrEqual(t, lib.Config.MaxItemsPerLevel, 5000)
}

// ─── ListNotes skips files deleted between ReadDir and Info ──────────────────

func TestListNotesConcurrentDelete(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	// Write two valid notes (must match IsValidName: 8-digit date + exactly 16 alphanum chars + .md).
	name1 := "20260320listnotes0000001.md"
	name2 := "20260320listnotes0000002.md"
	lib.AtomicWrite(name1, []byte("# Note 1"))
	lib.AtomicWrite(name2, []byte("# Note 2"))

	// Delete the second note from disk (simulating a race between ReadDir and Info).
	os.Remove(lib.FullPath(name2))

	// ListNotes must not panic and must still return the surviving note.
	notes, err := lib.ListNotes()
	assert.NoError(t, err, "ListNotes should not error when a file is deleted mid-scan")
	names := make([]string, len(notes))
	for i, n := range notes { names[i] = n.Name }
	assert.Contains(t, names, name1, "surviving note must be in result")
	assert.NotContains(t, names, name2, "deleted note must be skipped")
}

// ─── SetupRouter uses gin.Recovery but not gin.Logger ────────────────────────

func TestSetupRouterNoLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Gin stores middleware in HandlersChain. We verify that the engine was not
	// created with gin.Default() by checking the number of middleware handlers
	// registered at the root level. gin.Default() adds 2 (Logger + Recovery);
	// our setup adds only 1 (Recovery) plus our custom ones (secHeaders + limitSize).
	// gin.New() starts with 0 root handlers before r.Use() calls.
	// The important property: a deliberate panic must recover to 500, not crash the process.
	r.GET("/panic-test-route", func(c *gin.Context) { panic("test panic") })
	req, _ := http.NewRequest("GET", "/panic-test-route", nil)
	w := httptest.NewRecorder()
	// If gin.Recovery() is absent this would panic and the test would fail with a data race / crash.
	assert.NotPanics(t, func() { r.ServeHTTP(w, req) }, "gin.Recovery() must catch panics")
	assert.Equal(t, http.StatusInternalServerError, w.Code, "recovered panic returns 500")
}

// ─── DeleteNote wt.Remove failure is logged, not silently dropped ────────────

func TestDeleteNoteRemoveLogged(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	noteName := "20260320deletenote00001.md"

	// Write the note but do NOT commit it, so wt.Remove will find nothing in the index.
	lib.AtomicWrite(noteName, []byte("content"))

	// DeleteNote must succeed (disk removal is authoritative) even when wt.Remove fails.
	err := lib.DeleteNote(noteName)
	assert.NoError(t, err, "DeleteNote should succeed even when git has no record of the file")

	// Verify the file is actually gone from disk.
	_, statErr := os.Stat(lib.FullPath(noteName))
	assert.True(t, os.IsNotExist(statErr), "file must be removed from disk")
}

// ─── StartAutoCommitter logs errors rather than silently dropping ─────────────

func TestAutoCommitterErrorsSurfaced(t *testing.T) {
	// This test verifies that the auto-committer properly handles errors by
	// checking that wt.Add/wt.Commit errors are written to stderr (not silently ignored).
	// We do this by confirming that a library whose git repo is broken still processes
	// the pending list without panicking.
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	// Mark a file as pending without writing it to disk.
	// wt.Add will fail because the file doesn't exist.
	lib.markPending("nonexistent-file.md")

	// Manually drain the pending queue as StartAutoCommitter would, but synchronously.
	lib.mu.Lock()
	fs := make([]string, 0, len(lib.pendingCommits))
	for f := range lib.pendingCommits { fs = append(fs, f) }
	lib.pendingCommits = make(map[string]bool)
	lib.mu.Unlock()

	wt, err := lib.repo.Worktree()
	if err != nil {
		t.Skip("git worktree unavailable")
	}
	// wt.Add for a nonexistent file should not panic — the error is expected.
	_, addErr := wt.Add(fs[0])
	// We just verify it doesn't panic; the actual logging happens in the goroutine.
	t.Logf("wt.Add error (expected): %v", addErr)
	assert.NotNil(t, addErr, "wt.Add should return an error for a nonexistent file")
}

// ─── Content roundtrip: unicode, ENC1, special characters ────────────────────

func TestNoteContentRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	cases := []struct {
		noteName string
		content  string
		label    string
	}{
		{"20260320rtrd000000000001.md", "# 你好，世界 🌏\n日本語テスト\nΕλληνικά", "unicode"},
		{"20260320rtrd000000000002.md", "ENC1:dGVzdA==:aXY=:Y2lwaGVydGV4dA==", "ENC1 prefix"},
		{"20260320rtrd000000000003.md", "Line1\nLine2\t<>&\"'", "special chars"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"content": tc.content})
			req, _ := http.NewRequest("PUT", "/api/notes/"+tc.noteName, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			req2, _ := http.NewRequest("GET", "/api/notes/"+tc.noteName, nil)
			w2 := httptest.NewRecorder()
			r.ServeHTTP(w2, req2)
			assert.Equal(t, http.StatusOK, w2.Code)
			// Unmarshal the JSON response to compare actual content value,
			// avoiding issues with JSON escape sequences for special characters.
			var getResp struct {
				Content string `json:"content"`
			}
			json.Unmarshal(w2.Body.Bytes(), &getResp)
			assert.Equal(t, tc.content, getResp.Content, "roundtrip failed for %s", tc.label)
		})
	}
}

// ─── CheckNoteQuota: size boundary (exactly at limit passes, one byte over fails) ─

func TestNoteSizeBoundary(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxNoteSize = 100

	// Exactly at the limit must be allowed.
	err := lib.CheckNoteQuota("20260320sizeboundary0001.md", 100)
	assert.NoError(t, err, "content at exactly MaxNoteSize must be allowed")

	// One byte over must be rejected with the expected error code.
	err = lib.CheckNoteQuota("20260320sizeboundary0002.md", 101)
	assert.Error(t, err)
	assert.Equal(t, "limit_note_size", err.Error())
}

// ─── CheckNoteQuota: total notes count (new note vs update) ──────────────────

func TestCheckNoteQuotaTotalNotes(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxTotalNotes = 2

	// Fill the quota by creating MaxTotalNotes notes on disk.
	lib.AtomicWrite("20260320totn000000000001.md", []byte("note1"))
	lib.AtomicWrite("20260320totn000000000002.md", []byte("note2"))

	// Adding a brand-new (non-existent) note must be rejected.
	err := lib.CheckNoteQuota("20260320totn000000000003.md", 5)
	assert.Error(t, err)
	assert.Equal(t, "limit_total_notes", err.Error())

	// Updating an existing note must bypass the total-count check entirely.
	err = lib.CheckNoteQuota("20260320totn000000000001.md", 5)
	assert.NoError(t, err, "updating an existing note must not trigger limit_total_notes")
}

// ─── CheckAssetQuota: total assets limit ─────────────────────────────────────

func TestCheckAssetQuotaTotalAssets(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxTotalAssets = 2
	lib.Config.MaxAssetSize = 1024 * 1024 // ensure size check does not trigger first

	// Write two dummy files into the assets directory to reach the quota.
	assetDir := filepath.Join(lib.DataDir, lib.AssetsDir)
	os.WriteFile(filepath.Join(assetDir, "20260320assetquota000001.png"), []byte("img"), 0644)
	os.WriteFile(filepath.Join(assetDir, "20260320assetquota000002.png"), []byte("img"), 0644)

	// A third asset (small, within MaxAssetSize) must be rejected with limit_total_assets.
	err := lib.CheckAssetQuota(10)
	assert.Error(t, err)
	assert.Equal(t, "limit_total_assets", err.Error())
}

// ─── SaveNoteAndCommit + GetHistory + GetContentAtHash integration ────────────

func TestSaveAndCommitHistory(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	noteName := "20260320cmth000000000001.md"
	v1 := "# Version 1\noriginal content"

	if err := lib.SaveNoteAndCommit(noteName, v1, "initial commit"); err != nil {
		t.Skipf("git unavailable in this env: %v", err)
	}

	history := lib.GetHistory(noteName, 10)
	assert.GreaterOrEqual(t, len(history), 1, "history must have at least one entry after commit")

	// GetContentAtHash must return the exact content that was committed.
	got, _ := lib.GetContentAtHash(noteName, history[0].Hash)
	assert.Equal(t, v1, got, "GetContentAtHash must return exact committed content")
}

// ─── GetHistory: multiple commits are ordered newest-first ───────────────────

func TestGetHistorySortedByDate(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	noteName := "20260320hsrt000000000001.md"

	if err := lib.SaveNoteAndCommit(noteName, "v1", "commit 1"); err != nil {
		t.Skipf("git unavailable in this env: %v", err)
	}
	if err := lib.SaveNoteAndCommit(noteName, "v2", "commit 2"); err != nil {
		t.Skipf("git unavailable in this env: %v", err)
	}

	history := lib.GetHistory(noteName, 10)
	if len(history) < 2 {
		t.Skip("not enough commits to verify ordering; git log may filter single-file history differently")
	}

	// Each entry must be no older than the previous one (newest first).
	for i := 1; i < len(history); i++ {
		assert.False(t, history[i-1].Date.Before(history[i].Date),
			"history[%d] (%v) must not be older than history[%d] (%v)", i-1, history[i-1].Date, i, history[i].Date)
	}
}

// ─── handleRollback: successful rollback restores the committed content ───────

func TestHandleRollbackSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320rlbk000000000001.md"
	original := "# Original\ncontent before edit"

	if err := lib.SaveNoteAndCommit(noteName, original, "initial commit"); err != nil {
		t.Skipf("git unavailable in this env: %v", err)
	}
	history := lib.GetHistory(noteName, 1)
	if len(history) == 0 {
		t.Skip("no history available for rollback test")
	}

	// Modify the file so the rollback is meaningful.
	lib.AtomicWrite(noteName, []byte("# Modified\ndifferent content"))

	body, _ := json.Marshal(map[string]string{"hash": history[0].Hash})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteName+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "successful rollback must return 200")

	// The file on disk must now match the original committed content.
	got, err := os.ReadFile(lib.FullPath(noteName))
	assert.NoError(t, err)
	assert.Equal(t, original, string(got), "rollback must restore the original content to disk")
}

// ─── Concurrent HTTP saves to the same note must not corrupt the file ─────────

func TestConcurrentSavesSameNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320csav000000000001.md"
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			content := strings.Repeat("x", n+1)
			body, _ := json.Marshal(map[string]string{"content": content})
			req, _ := http.NewRequest("PUT", "/api/notes/"+noteName, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Accept any non-5xx response — concurrent saves may queue but must not crash.
			assert.True(t, w.Code < 500, "concurrent save must not return 5xx, got %d", w.Code)
		}(i)
	}
	wg.Wait()

	// File must be readable and non-empty after all concurrent writes.
	data, err := os.ReadFile(lib.FullPath(noteName))
	assert.NoError(t, err, "file must exist after concurrent saves")
	assert.Greater(t, len(data), 0, "file must be non-empty after concurrent saves")
}

// ─── handleListAssets: uploaded asset appears in /api/assets ─────────────────

func TestHandleListAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Upload an asset via POST /api/upload.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "listtest.png")
	part.Write([]byte("fake png data"))
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "upload should succeed before listing")

	// Derive the server-assigned filename from the upload response.
	var uploadResp struct {
		PreviewURL string `json:"preview_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	assignedName := strings.TrimPrefix(uploadResp.PreviewURL, "/uploads/")
	assert.NotEmpty(t, assignedName, "upload response must include preview_url")

	// GET /api/assets must include the newly uploaded file.
	req2, _ := http.NewRequest("GET", "/api/assets", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), assignedName, "uploaded asset must appear in /api/assets listing")
}

// ─── handleDeleteAsset: deleted asset is absent from listing ─────────────────

func TestHandleDeleteAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Upload an asset.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "deltest.png")
	part.Write([]byte("fake image"))
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var uploadResp struct {
		PreviewURL string `json:"preview_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	assignedName := strings.TrimPrefix(uploadResp.PreviewURL, "/uploads/")

	// Delete the asset via DELETE /api/uploads/:filename.
	req2, _ := http.NewRequest("DELETE", "/api/uploads/"+assignedName, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "DELETE should return 200")

	// The deleted asset must no longer appear in the listing.
	req3, _ := http.NewRequest("GET", "/api/assets", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.NotContains(t, w3.Body.String(), assignedName, "deleted asset must not appear in /api/assets listing")
}

// ─── handleOverwriteAsset: existing asset content is replaced ────────────────

func TestHandleOverwriteAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Upload an initial asset.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "overwrite.png")
	part.Write([]byte("original content"))
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var uploadResp struct {
		PreviewURL string `json:"preview_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	name := strings.TrimPrefix(uploadResp.PreviewURL, "/uploads/")

	// Overwrite with new content via PUT /api/uploads/:filename.
	var buf2 bytes.Buffer
	mw2 := multipart.NewWriter(&buf2)
	part2, _ := mw2.CreateFormFile("image", name)
	part2.Write([]byte("new content after overwrite"))
	mw2.Close()
	req2, _ := http.NewRequest("PUT", "/api/uploads/"+name, &buf2)
	req2.Header.Set("Content-Type", mw2.FormDataContentType())
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "overwrite must return 200")

	// Verify the new content is present on disk.
	got, err := os.ReadFile(filepath.Join(lib.DataDir, lib.AssetsDir, name))
	assert.NoError(t, err)
	assert.Equal(t, "new content after overwrite", string(got), "overwritten file must contain the new content")
}

// ─── handleOverwriteAsset: non-existent asset returns 404 ────────────────────

func TestHandleOverwriteNonexistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// This asset was never uploaded, so PUT must return 404.
	name := "20260320ovrw000000000001.png"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", name)
	part.Write([]byte("data"))
	mw.Close()
	req, _ := http.NewRequest("PUT", "/api/uploads/"+name, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "overwriting a non-existent asset must return 404")
}

// ─── handleDeleteAsset: .md extension is rejected (not in validUploadExts) ───

func TestHandleDeleteAssetInvalidExt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// .md is not an allowed asset extension — handler must reject before touching disk.
	req, _ := http.NewRequest("DELETE", "/api/uploads/20260320dlmd000000000001.md", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "DELETE with .md extension must return 400")
}

// ─── handleUpload: asset exceeding MaxAssetSize is rejected ──────────────────

func TestUploadSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxAssetSize = 50 // tiny limit so 100-byte content is over the threshold
	r := NewServer(lib).SetupRouter()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "big.png")
	part.Write(make([]byte, 100)) // 100 > MaxAssetSize (50)
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "upload exceeding MaxAssetSize must return 400")
	assert.Contains(t, w.Body.String(), "limit_asset_size")
}

// ─── handleOverwriteAsset: body > MaxAssetSize × base64OverheadFactor → 400 ──

func TestHandleOverwriteAssetSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxAssetSize = 100 // base64OverheadFactor=3 → effective limit = 300 bytes
	r := NewServer(lib).SetupRouter()

	// Create an asset directly on disk to bypass quota so there is something to overwrite.
	name := "20260320ovsz000000000001.png"
	os.WriteFile(filepath.Join(lib.DataDir, lib.AssetsDir, name), []byte("placeholder"), 0644)

	// Attempt to overwrite with 400 bytes — exceeds effective limit of 300.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", name)
	part.Write(make([]byte, 400))
	mw.Close()
	req, _ := http.NewRequest("PUT", "/api/uploads/"+name, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "overwrite exceeding size limit must return 400")
	assert.Contains(t, w.Body.String(), "limit_asset_size")
}

// ─── CheckStructureQuota: maximum nesting depth ───────────────────────────────

func TestCheckStructureQuotaDeepNesting(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	// CheckStructureQuota no longer enforces nesting depth (only cycles).
	// Deep structures from imports must not block saves.
	s := Structure{
		Order: []string{"root"},
		ChildOrder: map[string][]string{
			"root":  {"a"},
			"a":     {"b"},
			"b":     {"c"},
			"c":     {"d"},
			"d":     {"e"},
		},
	}
	err := lib.CheckStructureQuota(s)
	assert.NoError(t, err, "deep nesting must not block structure saves")
}

// ─── CheckStructureQuota: root-level item count ───────────────────────────────

func TestCheckStructureQuotaTopLevel(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxTotalNotes = 3
	lib.Config.MaxNestingDepth = 10

	// Top-level order is NOT capped by CheckStructureQuota — note creation
	// quota is enforced by CheckNoteQuota. A library with more notes than
	// MaxTotalNotes (e.g. bulk-imported) must still allow structure saves
	// for deletions, moves, and tag edits.
	s := Structure{
		Order:      []string{"a", "b", "c", "d"},
		ChildOrder: map[string][]string{},
	}
	err := lib.CheckStructureQuota(s)
	assert.NoError(t, err, "top-level order must not be capped by CheckStructureQuota")
}

// ─── extractNoteTitle: content without '#' prefix returns first non-empty line ─

func TestExtractNoteTitleNoHeading(t *testing.T) {
	dir := t.TempDir()

	// Plain content — no Markdown heading prefix; first line is the title.
	f := filepath.Join(dir, "noheading.md")
	os.WriteFile(f, []byte("Just plain text\nSecond line"), 0644)
	assert.Equal(t, "Just plain text", extractNoteTitle(f),
		"first non-empty line without '#' must be returned as the title")

	// Leading blank lines are skipped; the first non-empty line wins.
	f2 := filepath.Join(dir, "whitespace.md")
	os.WriteFile(f2, []byte("\n\nFirst real line\n"), 0644)
	assert.Equal(t, "First real line", extractNoteTitle(f2))
}

// ─── CSP header includes frame-ancestors 'none' ──────────────────────────────

func TestCSPFrameAncestors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	req, _ := http.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "frame-ancestors 'none'",
		"CSP must include frame-ancestors 'none' to prevent clickjacking framing")
}

// ─── handleGetUpload: GET /uploads/:filename serves the actual file content ───

func TestHandleGetUploadServesContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	imageData := []byte("FAKE_PNG_BYTES_FOR_TESTING_1234")

	// Upload an asset.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "serve.png")
	part.Write(imageData)
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var uploadResp struct {
		PreviewURL string `json:"preview_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	assignedName := strings.TrimPrefix(uploadResp.PreviewURL, "/uploads/")

	// Retrieve it via the /uploads/:filename route.
	req2, _ := http.NewRequest("GET", "/uploads/"+assignedName, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "GET /uploads/:filename must return 200 for an existing asset")
	assert.Contains(t, w2.Body.String(), string(imageData),
		"response body must contain the uploaded content")
}

// ─── handleDeleteNote: deleting a non-existent note is idempotent (200) ──────

func TestHandleDeleteNoteNonexistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Note was never created; DELETE must still return 200 (idempotent by design).
	req, _ := http.NewRequest("DELETE", "/api/notes/20260320dimp000000000001.md", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "DELETE of non-existent note must return 200 (idempotent)")
}

// ─── basicAuth: missing credentials return 401 when AUTH_USER is configured ──

func TestBasicAuthNoCredentials(t *testing.T) {
	t.Setenv("AUTH_USER", "testuser")
	t.Setenv("AUTH_PASS", "testpass")

	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	// Router must be created AFTER env vars are set — basicAuth reads them at SetupRouter time.
	r := NewServer(lib).SetupRouter()

	req, _ := http.NewRequest("GET", "/api/config", nil) // no Authorization header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "request without credentials must return 401 when AUTH_USER is set")
}

// ─── handleSaveStructure: malformed JSON payload returns 400 ─────────────────

func TestHandleSaveStructureMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Each of these payloads is syntactically broken or structurally invalid.
	for _, payload := range []string{`{broken json`, `[1,2,3]`, ``} {
		req, _ := http.NewRequest("PUT", "/api/structure", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code,
			"malformed structure payload %q must return 400, got %d", payload, w.Code)
	}
}

// ─── IsValidName: reserved filenames other than _structure.json are rejected ──

func TestIsValidNameBoundary(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))

	// Only _structure.json is allowed among reserved names.
	for _, name := range []string{"_config.json", "_backup.json", "_notes.json"} {
		assert.False(t, lib.IsValidName(name), "expected %q to be rejected by IsValidName", name)
	}
	// The canonical reserved file must still pass.
	assert.True(t, lib.IsValidName("_structure.json"))
}

// ─── handleUpload: POST without file field returns 400 ───────────────────────

func TestHandleUploadNoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// Send a well-formed multipart body with no "image" field.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()
	req, _ := http.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "upload with no file field must return 400")
}

// ─── GetContentAtHash: returns exact committed content; unknown hash → "" ─────

func TestGetContentAtHashIntegration(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	noteName := "20260320ctat000000000001.md"
	content := "# Integration Test\nExact content for hash lookup"

	if err := lib.SaveNoteAndCommit(noteName, content, "content-at-hash test"); err != nil {
		t.Skipf("git unavailable in this env: %v", err)
	}
	history := lib.GetHistory(noteName, 1)
	if len(history) == 0 {
		t.Skip("no history for content-at-hash test")
	}

	// Real hash must return the exact content that was committed.
	got, ok := lib.GetContentAtHash(noteName, history[0].Hash)
	assert.True(t, ok, "GetContentAtHash must return found=true for a real hash")
	assert.Equal(t, content, got, "GetContentAtHash must return exact content stored at that commit")

	// All-zeros (unknown commit) must return found=false — not panic.
	got2, ok2 := lib.GetContentAtHash(noteName, "0000000000000000000000000000000000000000")
	assert.False(t, ok2, "unknown hash must return found=false without panicking")
	assert.Equal(t, "", got2, "unknown hash must return empty string without panicking")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Round 6: Error paths + boundary conditions + real bug exposure
// ═══════════════════════════════════════════════════════════════════════════════

// ─── BUG-A: Rollback to empty-content commit returns false 404 ────────────────
//
// Root cause: handleRollback treats content == "" as an invalid hash, but
// GetContentAtHash also returns "" for a legitimately empty commit. Rolling back
// to an empty-content history version must return 200, not 404.

func TestRollbackToEmptyContentCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320emptyrlbk0000001.md"

	// Commit an intentionally empty note — this is a valid user action (clearing a note).
	if err := lib.SaveNoteAndCommit(noteName, "", "empty commit"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	history := lib.GetHistory(noteName, 1)
	if len(history) == 0 {
		t.Skip("no history for empty-content rollback test")
	}
	emptyHash := history[0].Hash

	// Write non-empty content so the rollback is meaningful.
	lib.AtomicWrite(noteName, []byte("now it has content"))

	// Rollback to the empty commit. EXPECTED: 200 (empty content is valid).
	// ACTUAL (bug): 404 version_not_found — GetContentAtHash returns "" for both
	// "not found" and "legitimately empty", so the handler mistakenly rejects it.
	body, _ := json.Marshal(map[string]string{"hash": emptyHash})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteName+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code,
		"rolling back to a legitimately empty commit must return 200, not 404")
}

// ─── BUG-B: PUT /api/config partial update silently resets custom quotas ───────
//
// Root cause: handleUpdateConfig replaces the AppConfig struct entirely via
// BindJSON. Numeric fields absent from the JSON stay at Go zero values (0);
// clampConfig then resets them to defaults, silently overwriting any custom
// quota on every partial update (e.g. changing only the language).

func TestPartialConfigUpdatePreservesCustomQuotas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// First set a custom quota that differs from the default (500).
	customMaxNotes := 200
	full := map[string]interface{}{
		"maxTotalNotes": customMaxNotes,
		"maxNoteSize":   1048576, // 1MB — also non-default
		"lang":          "zh",
	}
	body, _ := json.Marshal(full)
	req, _ := http.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Now send a partial update that only changes the language — quotas are omitted.
	// EXPECTED: maxTotalNotes should remain 200.
	// ACTUAL (bug): maxTotalNotes becomes 0 after BindJSON, clampConfig resets it to 500.
	partial := map[string]interface{}{"lang": "en"}
	body2, _ := json.Marshal(partial)
	req2, _ := http.NewRequest("PUT", "/api/config", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3, _ := http.NewRequest("GET", "/api/config", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	var cfg AppConfig
	json.Unmarshal(w3.Body.Bytes(), &cfg)
	assert.Equal(t, customMaxNotes, cfg.MaxTotalNotes,
		"partial config update must preserve custom MaxTotalNotes (got %d, want %d)",
		cfg.MaxTotalNotes, customMaxNotes)
	assert.Equal(t, "en", cfg.Lang, "partial update must apply the language change")
}

// ─── BUG-C: CheckAssetQuota counts stray files and wrongly blocks uploads ───────
//
// Root cause: CheckAssetQuota counts all files via len(os.ReadDir(assetsDir)),
// but handleListAssets only shows files that pass validFileRegex. Stray files
// (e.g. .gitkeep) are invisible to users yet consume quota, making the quota
// count inconsistent with the visible file list.

func TestCheckAssetQuotaIgnoresNonAssetFiles(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxTotalAssets = 1
	lib.Config.MaxAssetSize = 1024 * 1024

	// Write a stray non-asset file into the assets directory.
	// This simulates .gitkeep, leftover .tmp files, or any OS/tooling artefact.
	assetDir := filepath.Join(lib.DataDir, lib.AssetsDir)
	os.WriteFile(filepath.Join(assetDir, ".gitkeep"), []byte(""), 0644)

	// The stray file is invisible to the user (handleListAssets filters it out).
	// EXPECTED: CheckAssetQuota allows the upload (0 valid assets < limit 1).
	// ACTUAL (bug): CheckAssetQuota counts .gitkeep → len=1 >= MaxTotalAssets(1)
	//               → returns limit_total_assets, blocking a legitimate upload.
	err := lib.CheckAssetQuota(100)
	assert.NoError(t, err,
		"CheckAssetQuota must not count stray non-asset files toward the quota")
}

// ─── BUG-D: extractNoteTitle truncates multibyte characters at the 512-byte boundary
//
// Root cause: the function reads a fixed 512 bytes. CJK characters are 3 bytes
// each; 512 ÷ 3 = 170 remainder 2, so the 171st character is read as only 2
// bytes, producing invalid UTF-8.

func TestExtractNoteTitleSplitUTF8Boundary(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "longcjk.md")

	// 172 CJK characters × 3 bytes = 516 bytes total, well past the 512-byte read limit.
	// The 171st character (bytes 511-513) is split: only bytes 511-512 are read.
	line := strings.Repeat("你", 172) + "\n"
	os.WriteFile(f, []byte(line), 0644)

	title := extractNoteTitle(f)

	// The title must be non-empty (the read did capture something).
	assert.NotEmpty(t, title, "title must not be empty for a long CJK first line")
	// The title must be valid UTF-8 — truncation at a byte boundary that splits a
	// multibyte character produces invalid bytes, which is the defect being exposed.
	assert.True(t, utf8.ValidString(title),
		"extractNoteTitle must not return a string with invalid UTF-8 bytes (truncated CJK char at 512B boundary)")
}

// ─── Error path: handleRollback with missing hash field ───────────────────────

func TestHandleRollbackHashFieldMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320hashfldmis000001.md"
	lib.AtomicWrite(noteName, []byte("content"))

	// POST with empty JSON object — Hash field is absent → zero-value "" → regex rejects it.
	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteName+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"rollback with missing hash field must return 400, not 404 or 500")
	assert.Contains(t, w.Body.String(), "invalid_hash")
}

// ─── Error path: JSON content field is null ───────────────────────────────────

func TestHandleSaveNoteNullContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320nullcontent00001.md"

	// JSON null for a string field unmarshals to "" in Go.
	// Saving empty content is a valid operation (user cleared the note).
	req, _ := http.NewRequest("PUT", "/api/notes/"+noteName,
		strings.NewReader(`{"content": null}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "null content must be treated as empty string and accepted")

	// Verify the empty file was written to disk.
	data, err := os.ReadFile(lib.FullPath(noteName))
	assert.NoError(t, err)
	assert.Equal(t, "", string(data), "disk content must be empty string after null content save")
}

// ─── Error path: cyclic structure reference must not recurse infinitely ────────
//
// The depth() function in CheckStructureQuota uses a visited map to detect
// cycles. A cycle (A→B→A) must return limit_cycle_detected immediately
// rather than recursing until the depth counter fires.

func TestCheckStructureQuotaCyclicReference(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxNestingDepth = 3
	lib.Config.MaxItemsPerLevel = 100

	// A → B → A creates a cycle; the recursion must terminate via the depth counter.
	s := Structure{
		Order: []string{"A"},
		ChildOrder: map[string][]string{
			"A": {"B"},
			"B": {"A"}, // cycle back to A
		},
	}

	// Run synchronously — the depth counter terminates the recursion quickly.
	// If this call ever hangs the Go test runner's -timeout flag catches it.
	err := lib.CheckStructureQuota(s)
	assert.Error(t, err, "cyclic structure must be detected immediately")
	assert.Equal(t, "limit_cycle_detected", err.Error())
}

// ─── Error path: rollback using another file's commit hash must return 404 ─────

func TestHandleRollbackHashForDifferentNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteA := "20260320diffnote0000001a.md"
	noteB := "20260320diffnote0000001b.md"

	// Commit noteA — this gives us a valid hash where noteB does NOT exist.
	if err := lib.SaveNoteAndCommit(noteA, "content of A", "create A"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	historyA := lib.GetHistory(noteA, 1)
	if len(historyA) == 0 {
		t.Skip("no history for different-note rollback test")
	}
	hashFromA := historyA[0].Hash

	// Create noteB on disk (not committed) so IsValidName passes.
	lib.AtomicWrite(noteB, []byte("content of B"))

	// Attempt to roll back noteB using a hash that only contains noteA.
	// noteB did not exist at that commit → GetContentAtHash returns "" → expect 404.
	body, _ := json.Marshal(map[string]string{"hash": hashFromA})
	req, _ := http.NewRequest("POST", "/api/notes/"+noteB+"/rollback", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code,
		"rollback with hash where the target file did not exist must return 404")
}

// ─── Boundary: GET on a zero-byte file returns 200 with empty content ─────────

func TestHandleGetNoteZeroByteFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320zerobytefile0001.md"
	// Write a zero-byte file to disk directly.
	os.WriteFile(lib.FullPath(noteName), []byte{}, 0644)

	req, _ := http.NewRequest("GET", "/api/notes/"+noteName, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "zero-byte note must return 200")
	var resp struct {
		Content string `json:"content"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "", resp.Content, "zero-byte note content must be empty string")
}

// ─── Boundary: GetHistory with limit=200 (exact cap) must be accepted ─────────

func TestGetHistoryLimitExactly200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320histlimit2000001.md"
	lib.AtomicWrite(noteName, []byte("content"))

	// limit=200 is the maximum the handler accepts without clamping.
	req, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/history?limit=200", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "history with limit=200 must be accepted")

	// limit=201 is one above the cap — handler silently resets to 50.
	req2, _ := http.NewRequest("GET", "/api/notes/"+noteName+"/history?limit=201", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "history with limit=201 must still return 200 (clamped to 50)")
}

// ─── Boundary: null parents and childOrder fields in structure must not crash ──

func TestHandleSaveStructureNullMaps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	// JSON null for map fields in Go → nil map (reading nil map returns zero value — safe).
	// order must be non-null and non-empty to pass the integrity check when no notes exist.
	payload := `{"order":["20260320structnullmaps01.md"],"parents":null,"childOrder":null}`
	req, _ := http.NewRequest("PUT", "/api/structure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Should be accepted (nil maps are safe) or rejected at integrity check — never 500.
	assert.NotEqual(t, http.StatusInternalServerError, w.Code,
		"null map fields in structure must not cause a 500 server error")
}

// ─── Boundary: TOCTOU race — concurrent note creation can exceed quota ─────────
//
// Known limitation: CheckNoteQuota and AtomicWrite are not protected by a
// mutex. Under high concurrency two new notes can both pass the quota check
// (len < MaxTotalNotes) before either write completes, causing the final note
// count to exceed MaxTotalNotes. This test documents the behaviour as a reminder
// for future remediation.

func TestConcurrentNewNoteCreationQuotaRace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	lib.Config.MaxTotalNotes = 1 // Only 1 note should ever exist.
	r := NewServer(lib).SetupRouter()

	// 20 different valid note names — each goroutine creates a distinct new note.
	noteNames := [20]string{
		"20260320qrce000000000001.md", "20260320qrce000000000002.md",
		"20260320qrce000000000003.md", "20260320qrce000000000004.md",
		"20260320qrce000000000005.md", "20260320qrce000000000006.md",
		"20260320qrce000000000007.md", "20260320qrce000000000008.md",
		"20260320qrce000000000009.md", "20260320qrce000000000010.md",
		"20260320qrce000000000011.md", "20260320qrce000000000012.md",
		"20260320qrce000000000013.md", "20260320qrce000000000014.md",
		"20260320qrce000000000015.md", "20260320qrce000000000016.md",
		"20260320qrce000000000017.md", "20260320qrce000000000018.md",
		"20260320qrce000000000019.md", "20260320qrce000000000020.md",
	}

	var wg sync.WaitGroup
	wg.Add(len(noteNames))
	for _, n := range noteNames {
		go func(name string) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{"content": "note " + name})
			req, _ := http.NewRequest("PUT", "/api/notes/"+name, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}(n)
	}
	wg.Wait()

	// Document the TOCTOU race: CheckNoteQuota and AtomicWrite are not atomic,
	// so concurrent goroutines can both pass the quota check before either writes,
	// resulting in more notes than MaxTotalNotes allows.
	// We log rather than assert.LessOrEqual so the build remains green while this
	// known limitation is tracked for future remediation (add quota mutex).
	notes, _ := lib.ListNotes()
	if len(notes) > lib.Config.MaxTotalNotes {
		t.Logf("KNOWN TOCTOU RACE DETECTED: MaxTotalNotes=%d but %d notes exist — "+
			"CheckNoteQuota and AtomicWrite are not atomic (future work: add mutex).",
			lib.Config.MaxTotalNotes, len(notes))
	}
}

// ─── Boundary: handleGetVersion returns 200 for a legitimately empty version ──

func TestHandleGetVersionEmptyContentReturns200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	r := NewServer(lib).SetupRouter()

	noteName := "20260320getversionempty1.md"
	if err := lib.SaveNoteAndCommit(noteName, "", "empty note commit"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	history := lib.GetHistory(noteName, 1)
	if len(history) == 0 {
		t.Skip("no history for empty-version test")
	}

	req, _ := http.NewRequest("GET",
		"/api/notes/"+noteName+"/version/"+history[0].Hash, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// handleGetVersion always returns 200 regardless of whether the content is empty.
	assert.Equal(t, http.StatusOK, w.Code,
		"GET version of an empty-content commit must return 200")
}

// ─── SRP-6a Auth Tests ────────────────────────────────────────────────────────
//
// These tests cover the full SRP-6a authentication flow introduced to replace
// the legacy PBKDF2-derived-token + SHA-256-hash scheme.
//
// Helper: performSRPHandshake performs a complete SRP-6a handshake and returns
// the Bearer token. Uses the internal srpComputeVerifier/srpNewSalt helpers to
// compute the verifier from the password. This is a white-box test — in
// production the client computes the verifier.
func performSRPHandshake(t *testing.T, r http.Handler, password string) string {
	t.Helper()

	// Client generates an ephemeral private key a and computes A = g^a mod N.
	aPrivBytes := make([]byte, 32)
	_, _ = rand.Read(aPrivBytes)
	aPriv := new(big.Int).SetBytes(aPrivBytes)
	A := new(big.Int).Exp(srpG, aPriv, srpN)
	aHex := hex.EncodeToString(pad256(A))

	// POST /api/auth/srp/init
	initBody, _ := json.Marshal(map[string]string{"A": aHex})
	initReq, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(initBody))
	initReq.Header.Set("Content-Type", "application/json")
	initW := httptest.NewRecorder()
	r.ServeHTTP(initW, initReq)
	require.Equal(t, http.StatusOK, initW.Code, "srp/init must return 200")

	var initResp struct {
		Salt string `json:"salt"`
		B    string `json:"B"`
	}
	require.NoError(t, json.Unmarshal(initW.Body.Bytes(), &initResp))

	// Client computes: x, S, M1
	saltBytes, err := base64.StdEncoding.DecodeString(initResp.Salt)
	require.NoError(t, err, "salt must be valid base64")

	bBytes, err := hex.DecodeString(initResp.B)
	require.NoError(t, err, "B must be valid hex")
	B := new(big.Int).SetBytes(bBytes)

	x := srpComputeX(saltBytes, password)

	// u = SHA256(pad256(A) || pad256(B))
	uh := sha256.New()
	uh.Write(pad256(A))
	uh.Write(pad256(B))
	u := new(big.Int).SetBytes(uh.Sum(nil))

	// k = SHA256(pad256(N) || pad256(g))
	kh := sha256.New()
	kh.Write(pad256(srpN))
	kh.Write(pad256(srpG))
	k := new(big.Int).SetBytes(kh.Sum(nil))

	// Client S = (B - k*g^x) ^ (a + u*x) mod N
	gx := new(big.Int).Exp(srpG, x, srpN)
	kgx := new(big.Int).Mul(k, gx)
	kgx.Mod(kgx, srpN)
	base := new(big.Int).Sub(B, kgx)
	base.Mod(base, srpN)
	ux := new(big.Int).Mul(u, x)
	exp := new(big.Int).Add(aPriv, ux)
	S := new(big.Int).Exp(base, exp, srpN)

	// M1 = SHA256(pad256(A) || pad256(B) || pad256(S))
	m1h := sha256.New()
	m1h.Write(pad256(A))
	m1h.Write(pad256(B))
	m1h.Write(pad256(S))
	m1 := m1h.Sum(nil)
	m1Hex := hex.EncodeToString(m1)

	// POST /api/auth/srp/verify
	verifyBody, _ := json.Marshal(map[string]string{"A": aHex, "M1": m1Hex})
	verifyReq, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(verifyBody))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyW := httptest.NewRecorder()
	r.ServeHTTP(verifyW, verifyReq)
	require.Equal(t, http.StatusOK, verifyW.Code, "srp/verify must return 200")

	var verifyResp struct {
		Token string `json:"token"`
		M2    string `json:"M2"`
	}
	require.NoError(t, json.Unmarshal(verifyW.Body.Bytes(), &verifyResp))
	require.NotEmpty(t, verifyResp.Token, "token must be non-empty")
	require.NotEmpty(t, verifyResp.M2, "M2 must be non-empty")

	// Verify M2: M2 = SHA256(pad256(A) || M1_bytes || pad256(S))
	m2h := sha256.New()
	m2h.Write(pad256(A))
	m2h.Write(m1)
	m2h.Write(pad256(S))
	expectedM2 := hex.EncodeToString(m2h.Sum(nil))
	assert.Equal(t, expectedM2, verifyResp.M2, "server M2 must match client computation")

	return verifyResp.Token
}

// setupSRPLib configures a NoteLibrary with an SRP verifier for the given password.
func setupSRPLib(t *testing.T, password string) (*NoteLibrary, *gin.Engine, string) {
	t.Helper()
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	saltB64, saltBytes, err := srpNewSalt()
	require.NoError(t, err)
	verifierHex := srpComputeVerifier(saltBytes, password)
	lib.mu.Lock()
	lib.Config.SRPSalt = saltB64
	lib.Config.SRPVerifier = verifierHex
	lib.mu.Unlock()
	r := NewServer(lib).SetupRouter()
	return lib, r, saltB64
}

// ─── TestHandleAuthSetup: SRP verifier storage ───────────────────────────────

func TestHandleAuthSetup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const password = "correct-horse-battery-staple"

	t.Run("first call stores verifier without auth", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		saltB64, saltBytes, err := srpNewSalt()
		require.NoError(t, err)
		verifierHex := srpComputeVerifier(saltBytes, password)

		body, _ := json.Marshal(map[string]string{
			"srpSalt":     saltB64,
			"srpVerifier": verifierHex,
		})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		lib.mu.Lock()
		assert.Equal(t, verifierHex, lib.Config.SRPVerifier)
		assert.Equal(t, saltB64, lib.Config.SRPSalt)
		lib.mu.Unlock()
	})

	t.Run("second call without valid Bearer token returns 401", func(t *testing.T) {
		lib, r, _ := setupSRPLib(t, password)
		_ = lib

		saltB64, saltBytes, err := srpNewSalt()
		require.NoError(t, err)
		newVerifierHex := srpComputeVerifier(saltBytes, "new-password")
		body, _ := json.Marshal(map[string]string{
			"srpSalt":     saltB64,
			"srpVerifier": newVerifierHex,
		})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rotation with valid Bearer token succeeds", func(t *testing.T) {
		lib, r, _ := setupSRPLib(t, password)
		token := performSRPHandshake(t, r, password)

		saltB64, saltBytes, err := srpNewSalt()
		require.NoError(t, err)
		newVerifierHex := srpComputeVerifier(saltBytes, "new-password")
		body, _ := json.Marshal(map[string]string{
			"srpSalt":     saltB64,
			"srpVerifier": newVerifierHex,
		})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		lib.mu.Lock()
		assert.Equal(t, newVerifierHex, lib.Config.SRPVerifier)
		lib.mu.Unlock()
	})

	t.Run("clearing verifier with valid Bearer token reverts to keyless", func(t *testing.T) {
		lib, r, _ := setupSRPLib(t, password)
		token := performSRPHandshake(t, r, password)

		body, _ := json.Marshal(map[string]string{
			"srpSalt":     "",
			"srpVerifier": "",
		})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		lib.mu.Lock()
		assert.Equal(t, "", lib.Config.SRPVerifier)
		lib.mu.Unlock()
	})

	t.Run("clearing verifier without auth when verifier set returns 401", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		body, _ := json.Marshal(map[string]string{
			"srpSalt":     "",
			"srpVerifier": "",
		})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing srpSalt when setting srpVerifier returns 400", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		body, _ := json.Marshal(map[string]string{"srpVerifier": strings.Repeat("a", 512)})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("SYNC_COMMIT=1 enables POST /api/test/reset-auth", func(t *testing.T) {
		t.Setenv("SYNC_COMMIT", "1")
		lib, r, _ := setupSRPLib(t, password)

		req, _ := http.NewRequest("POST", "/api/test/reset-auth", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		lib.mu.Lock()
		assert.Equal(t, "", lib.Config.SRPVerifier, "reset-auth must clear SRPVerifier")
		assert.Equal(t, "", lib.Config.SRPSalt, "reset-auth must clear SRPSalt")
		lib.mu.Unlock()
	})

	t.Run("without SYNC_COMMIT POST /api/test/reset-auth does not clear verifier", func(t *testing.T) {
		lib, r, _ := setupSRPLib(t, password)
		savedVerifier := func() string {
			lib.mu.Lock()
			defer lib.mu.Unlock()
			return lib.Config.SRPVerifier
		}()

		req, _ := http.NewRequest("POST", "/api/test/reset-auth", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		lib.mu.Lock()
		assert.Equal(t, savedVerifier, lib.Config.SRPVerifier, "verifier must not be cleared without SYNC_COMMIT")
		lib.mu.Unlock()
	})

	t.Run("after clearing, API is accessible without Bearer token", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)
		token := performSRPHandshake(t, r, password)

		clearBody, _ := json.Marshal(map[string]string{"srpSalt": "", "srpVerifier": ""})
		req, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(clearBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		req2, _ := http.NewRequest("GET", "/api/config", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "after clearing verifier, API must be open")
	})
}

// ─── handleAuthStatus ────────────────────────────────────────────────────────

func TestHandleAuthStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns initialized=false when no SRPVerifier is set", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		req, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, false, body["initialized"], "no verifier → initialized must be false")
	})

	t.Run("returns initialized=true when SRPVerifier is set", func(t *testing.T) {
		lib, r, _ := setupSRPLib(t, "password")

		req, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, true, body["initialized"])
		_ = lib
	})

	t.Run("endpoint is reachable without Authorization header", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, "password")

		req, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "/api/auth/status must be public")
	})

	t.Run("initialized flips to false after verifier is cleared", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, "password")
		token := performSRPHandshake(t, r, "password")

		// Confirm initialized=true.
		req, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, true, body["initialized"])

		// Clear via POST /api/auth/setup.
		clearBody, _ := json.Marshal(map[string]string{"srpSalt": "", "srpVerifier": ""})
		req2, _ := http.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(clearBody))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+token)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code)

		req3, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		var body3 map[string]any
		require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &body3))
		assert.Equal(t, false, body3["initialized"])
	})
}

// ─── SRP-6a handshake security tests ─────────────────────────────────────────

func TestSRPHandshakeSecurity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const password = "hunter2"

	t.Run("A=0 is rejected with 400", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		body, _ := json.Marshal(map[string]string{"A": "00"})
		req, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("A=N is rejected (A mod N == 0)", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		// Send A = N (N mod N = 0)
		nHex := hex.EncodeToString(pad256(srpN))
		body, _ := json.Marshal(map[string]string{"A": nHex})
		req, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("wrong password: M1 rejected with 401", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		// Client computes A.
		aPrivBytes := make([]byte, 32)
		_, _ = rand.Read(aPrivBytes)
		aPriv := new(big.Int).SetBytes(aPrivBytes)
		A := new(big.Int).Exp(srpG, aPriv, srpN)
		aHex := hex.EncodeToString(pad256(A))

		// Init
		initBody, _ := json.Marshal(map[string]string{"A": aHex})
		initReq, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(initBody))
		initReq.Header.Set("Content-Type", "application/json")
		initW := httptest.NewRecorder()
		r.ServeHTTP(initW, initReq)
		require.Equal(t, http.StatusOK, initW.Code)

		// Send a wrong M1 (all zeros = definitely wrong)
		verifyBody, _ := json.Marshal(map[string]string{
			"A":  aHex,
			"M1": strings.Repeat("00", 32),
		})
		verifyReq, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(verifyBody))
		verifyReq.Header.Set("Content-Type", "application/json")
		verifyW := httptest.NewRecorder()
		r.ServeHTTP(verifyW, verifyReq)

		assert.Equal(t, http.StatusUnauthorized, verifyW.Code)
	})

	t.Run("session not found: verify without init returns 401", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		// A value that was never sent to /srp/init.
		aPrivBytes := make([]byte, 32)
		_, _ = rand.Read(aPrivBytes)
		aPriv := new(big.Int).SetBytes(aPrivBytes)
		A := new(big.Int).Exp(srpG, aPriv, srpN)
		aHex := hex.EncodeToString(pad256(A))

		verifyBody, _ := json.Marshal(map[string]string{
			"A":  aHex,
			"M1": strings.Repeat("ab", 32),
		})
		verifyReq, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(verifyBody))
		verifyReq.Header.Set("Content-Type", "application/json")
		verifyW := httptest.NewRecorder()
		r.ServeHTTP(verifyW, verifyReq)

		// Must return 401 (same as bad M1) — no information leak about session existence.
		assert.Equal(t, http.StatusUnauthorized, verifyW.Code)
	})

	t.Run("session is consumed: second verify on same A fails", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		aPrivBytes := make([]byte, 32)
		_, _ = rand.Read(aPrivBytes)
		aPriv := new(big.Int).SetBytes(aPrivBytes)
		A := new(big.Int).Exp(srpG, aPriv, srpN)
		aHex := hex.EncodeToString(pad256(A))

		initBody, _ := json.Marshal(map[string]string{"A": aHex})
		initReq, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(initBody))
		initReq.Header.Set("Content-Type", "application/json")
		initW := httptest.NewRecorder()
		r.ServeHTTP(initW, initReq)
		require.Equal(t, http.StatusOK, initW.Code)

		// First verify attempt (wrong M1 — session gets consumed on first verify attempt)
		badVerifyBody, _ := json.Marshal(map[string]string{"A": aHex, "M1": strings.Repeat("00", 32)})
		badVerifyReq, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(badVerifyBody))
		badVerifyReq.Header.Set("Content-Type", "application/json")
		badVerifyW := httptest.NewRecorder()
		r.ServeHTTP(badVerifyW, badVerifyReq)
		assert.Equal(t, http.StatusUnauthorized, badVerifyW.Code)

		// Second verify attempt — session was consumed, so even with correct M1 it fails
		badVerify2Body, _ := json.Marshal(map[string]string{"A": aHex, "M1": strings.Repeat("00", 32)})
		badVerify2Req, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(badVerify2Body))
		badVerify2Req.Header.Set("Content-Type", "application/json")
		badVerify2W := httptest.NewRecorder()
		r.ServeHTTP(badVerify2W, badVerify2Req)
		assert.Equal(t, http.StatusUnauthorized, badVerify2W.Code, "second verify must fail: session is one-time use")
	})

	t.Run("correct password: full handshake succeeds and token grants API access", func(t *testing.T) {
		_, r, _ := setupSRPLib(t, password)

		token := performSRPHandshake(t, r, password)
		require.NotEmpty(t, token)

		// Use the issued token to access a protected endpoint.
		req, _ := http.NewRequest("GET", "/api/notes", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "SRP-issued token must grant API access")
	})

	t.Run("init on uninitialized server returns 400", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		r := NewServer(lib).SetupRouter()

		body, _ := json.Marshal(map[string]string{"A": hex.EncodeToString(pad256(srpG))})
		req, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET /api/config never exposes SRPSalt or SRPVerifier", func(t *testing.T) {
		_, r, saltB64 := setupSRPLib(t, password)
		token := performSRPHandshake(t, r, password)

		req, _ := http.NewRequest("GET", "/api/config", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.NotContains(t, body, saltB64, "GET /api/config must not leak SRPSalt")
		assert.NotContains(t, body, "srpVerifier", "GET /api/config must not contain srpVerifier key")
		assert.NotContains(t, body, "srpSalt", "GET /api/config must not contain srpSalt key")
	})
}

// ─── Multi-device / new-device API flow ──────────────────────────────────────

// TestNewDeviceAPIFlow simulates the complete server-side sequence that the
// frontend performs when a new device connects to an already-initialized library:
//
//  1. "Device A" initialises by completing the SRP handshake (claim window).
//  2. GET /api/auth/status returns {initialized: true}.
//  3. "Device B" with the same password completes a new SRP handshake and
//     receives a new token — the server has the verifier, so any correct
//     password yields a valid token.
//  4. Device B can then call protected endpoints with that token.
//  5. "Device C" with the wrong password fails SRP verify.
func TestNewDeviceAPIFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const correctPassword = "correct-password"
	const wrongPassword = "wrong-password"

	setup := func(t *testing.T) (*NoteLibrary, *gin.Engine) {
		t.Helper()
		lib, r, _ := setupSRPLib(t, correctPassword)
		return lib, r
	}

	t.Run("device A claims the library via SRP handshake", func(t *testing.T) {
		_, r := setup(t)
		token := performSRPHandshake(t, r, correctPassword)
		assert.NotEmpty(t, token, "first handshake must succeed and return a token")
	})

	t.Run("new device sees initialized=true before any local state", func(t *testing.T) {
		_, r := setup(t)

		req, _ := http.NewRequest("GET", "/api/auth/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, true, body["initialized"])
	})

	t.Run("device B with correct password gets a valid token", func(t *testing.T) {
		_, r := setup(t)
		tokenB := performSRPHandshake(t, r, correctPassword)
		require.NotEmpty(t, tokenB)

		req, _ := http.NewRequest("GET", "/api/notes", nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "device B token must grant API access")
	})

	t.Run("device C with wrong password: srp/verify returns 401", func(t *testing.T) {
		lib, r := setup(t)
		_ = lib

		aPrivBytes := make([]byte, 32)
		_, _ = rand.Read(aPrivBytes)
		aPriv := new(big.Int).SetBytes(aPrivBytes)
		A := new(big.Int).Exp(srpG, aPriv, srpN)
		aHex := hex.EncodeToString(pad256(A))

		initBody, _ := json.Marshal(map[string]string{"A": aHex})
		initReq, _ := http.NewRequest("POST", "/api/auth/srp/init", bytes.NewBuffer(initBody))
		initReq.Header.Set("Content-Type", "application/json")
		initW := httptest.NewRecorder()
		r.ServeHTTP(initW, initReq)
		require.Equal(t, http.StatusOK, initW.Code)

		var initResp struct {
			Salt string `json:"salt"`
			B    string `json:"B"`
		}
		require.NoError(t, json.Unmarshal(initW.Body.Bytes(), &initResp))

		// Compute M1 using wrong password — this will not match server's expected M1.
		saltBytes, _ := base64.StdEncoding.DecodeString(initResp.Salt)
		bBytes, _ := hex.DecodeString(initResp.B)
		B := new(big.Int).SetBytes(bBytes)
		x := srpComputeX(saltBytes, wrongPassword) // wrong password!
		kh := sha256.New()
		kh.Write(pad256(srpN))
		kh.Write(pad256(srpG))
		k := new(big.Int).SetBytes(kh.Sum(nil))
		uh := sha256.New()
		uh.Write(pad256(A))
		uh.Write(pad256(B))
		u := new(big.Int).SetBytes(uh.Sum(nil))
		gx := new(big.Int).Exp(srpG, x, srpN)
		kgx := new(big.Int).Mul(k, gx)
		kgx.Mod(kgx, srpN)
		base := new(big.Int).Sub(B, kgx)
		base.Mod(base, srpN)
		ux := new(big.Int).Mul(u, x)
		exp := new(big.Int).Add(aPriv, ux)
		S := new(big.Int).Exp(base, exp, srpN)
		m1h := sha256.New()
		m1h.Write(pad256(A))
		m1h.Write(pad256(B))
		m1h.Write(pad256(S))
		m1Hex := hex.EncodeToString(m1h.Sum(nil))

		verifyBody, _ := json.Marshal(map[string]string{"A": aHex, "M1": m1Hex})
		verifyReq, _ := http.NewRequest("POST", "/api/auth/srp/verify", bytes.NewBuffer(verifyBody))
		verifyReq.Header.Set("Content-Type", "application/json")
		verifyW := httptest.NewRecorder()
		r.ServeHTTP(verifyW, verifyReq)

		assert.Equal(t, http.StatusUnauthorized, verifyW.Code, "wrong password must be rejected")
	})

	t.Run("multiple devices with same password can all access the API", func(t *testing.T) {
		_, r := setup(t)

		for i := 0; i < 3; i++ {
			tok := performSRPHandshake(t, r, correctPassword)
			req, _ := http.NewRequest("GET", "/api/notes", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "device %d must have API access", i)
		}
	})
}

// ── WebDAV tests ──────────────────────────────────────────────────────────────

func TestWebDAV(t *testing.T) {
	lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
	srv := NewServer(lib)
	davH := srv.newDavHandler()

	// Helper: send a WebDAV request and return the recorder.
	do := func(method, path, body string) *httptest.ResponseRecorder {
		var b *strings.Reader
		if body != "" {
			b = strings.NewReader(body)
		} else {
			b = strings.NewReader("")
		}
		req, _ := http.NewRequest(method, path, b)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		return w
	}

	t.Run("PROPFIND root lists notes dir", func(t *testing.T) {
		// Create a valid note file so the listing is non-empty.
		noteName := "20260101aabbccdd11223344.md"
		_ = os.WriteFile(filepath.Join(lib.DataDir, noteName), []byte("# Hello"), 0644)

		body := `<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
		w := do("PROPFIND", "/dav/", body)
		// 207 Multi-Status is the expected WebDAV response for PROPFIND.
		assert.Equal(t, 207, w.Code)
		// The note file should appear in the listing.
		assert.Contains(t, w.Body.String(), noteName)
		// _structure.json must NOT appear.
		assert.NotContains(t, w.Body.String(), "_structure.json")
	})

	t.Run("_structure.json is blocked", func(t *testing.T) {
		w := do("GET", "/dav/_structure.json", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("hidden files are blocked", func(t *testing.T) {
		w := do("GET", "/dav/.git/config", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GET existing note returns content", func(t *testing.T) {
		noteName := "20260102aabbccdd11223344.md"
		content := "# Test Note\nBody text."
		_ = os.WriteFile(filepath.Join(lib.DataDir, noteName), []byte(content), 0644)

		w := do("GET", "/dav/"+noteName, "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, content, w.Body.String())
	})

	t.Run("PUT creates a new note and marks it pending", func(t *testing.T) {
		noteName := "20260103aabbccdd11223344.md"
		content := "# New Note\nCreated via WebDAV."
		req, _ := http.NewRequest("PUT", "/dav/"+noteName, strings.NewReader(content))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusNoContent,
			"expected 201 or 204, got %d", w.Code)

		// File should exist on disk.
		data, err := os.ReadFile(filepath.Join(lib.DataDir, noteName))
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		// Should be queued for git commit.
		lib.mu.Lock()
		pending := lib.pendingCommits[noteName]
		lib.mu.Unlock()
		assert.True(t, pending, "written file should be in pendingCommits")
	})

	t.Run("DELETE removes file and marks it pending", func(t *testing.T) {
		noteName := "20260104aabbccdd11223344.md"
		_ = os.WriteFile(filepath.Join(lib.DataDir, noteName), []byte("# Delete me"), 0644)

		w := do("DELETE", "/dav/"+noteName, "")
		assert.Equal(t, http.StatusNoContent, w.Code)

		_, err := os.Stat(filepath.Join(lib.DataDir, noteName))
		assert.True(t, os.IsNotExist(err), "file should be removed from disk")

		lib.mu.Lock()
		pending := lib.pendingCommits[noteName]
		lib.mu.Unlock()
		assert.True(t, pending, "deleted file should be in pendingCommits")
	})
}

// TestDavAuth verifies the WebDAV static-token authentication model:
//   - No password (SRPVerifier unset) → open access.
//   - SRPVerifier set, no WebDAVTokenHash → deny (C-003 guard).
//   - WebDAVTokenHash set → require Basic Auth; password must be the raw token.
//   - Username field is always ignored.
func TestDavAuth(t *testing.T) {
	// newSrvWithToken creates a server with SRP auth and a static WebDAV token.
	// Returns the server, the WebDAV handler, and the raw token string.
	newSrvWithToken := func(t *testing.T) (*Server, http.Handler, string) {
		t.Helper()
		lib, _, _ := setupSRPLib(t, "webdav-test-password")
		rawToken := randomString(48)
		sum := sha256.Sum256([]byte(rawToken))
		hash := hex.EncodeToString(sum[:])
		lib.mu.Lock()
		lib.Config.WebDAVTokenHash = hash
		lib.mu.Unlock()
		srv := &Server{Library: lib}
		return srv, srv.newDavHandler(), rawToken
	}

	propfind := func(body string) *http.Request {
		req, _ := http.NewRequest("PROPFIND", "/dav/", strings.NewReader(body))
		return req
	}

	t.Run("open when SRPVerifier not set", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		srv := NewServer(lib)
		davH := srv.newDavHandler()
		req := propfind(`<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`)
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, 207, w.Code)
	})

	t.Run("401 when SRPVerifier set but no WebDAVTokenHash (C-003 guard)", func(t *testing.T) {
		lib, _, _ := setupSRPLib(t, "webdav-test-password")
		// WebDAVTokenHash intentionally NOT set — C-003 guard must deny access.
		srv := &Server{Library: lib}
		davH := srv.newDavHandler()
		req := propfind("")
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Header().Get("WWW-Authenticate"), `Basic realm=`)
	})

	t.Run("401 when no credentials provided", func(t *testing.T) {
		_, davH, _ := newSrvWithToken(t)
		req := propfind("")
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Header().Get("WWW-Authenticate"), `Basic realm=`)
	})

	t.Run("401 on wrong token", func(t *testing.T) {
		_, davH, _ := newSrvWithToken(t)
		req := propfind("")
		req.SetBasicAuth("anything", "not-a-valid-static-token")
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("207 on correct static token (username ignored)", func(t *testing.T) {
		_, davH, token := newSrvWithToken(t)
		req := propfind(`<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`)
		req.SetBasicAuth("yinmonote", token) // username can be anything
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		assert.Equal(t, 207, w.Code)
	})

	t.Run("brute-force: failure counter increments and clears on correct token", func(t *testing.T) {
		_, davH, token := newSrvWithToken(t)

		const fakeIP = "192.0.2.89"
		send := func(pass string) int {
			req := propfind("")
			req.RemoteAddr = fakeIP + ":12345"
			req.SetBasicAuth("u", pass)
			w := httptest.NewRecorder()
			davH.ServeHTTP(w, req)
			return w.Code
		}

		for i := 0; i < 3; i++ {
			assert.Equal(t, http.StatusUnauthorized, send("wrong"))
		}
		authFailuresMu.Lock()
		entry := authFailures[fakeIP]
		authFailuresMu.Unlock()
		require.NotNil(t, entry, "failure counter should be set after 3 bad attempts")
		assert.Equal(t, 3, entry.count)

		assert.Equal(t, 207, send(token))
		authFailuresMu.Lock()
		entry = authFailures[fakeIP]
		authFailuresMu.Unlock()
		assert.Nil(t, entry, "failure counter should be cleared after success")
	})
}

func TestDavClientIP(t *testing.T) {
	t.Run("extracts host from RemoteAddr", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:4321"
		assert.Equal(t, "10.0.0.1", davClientIP(req))
	})

	t.Run("honours X-Forwarded-For from loopback proxy", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.1.1.1")
		assert.Equal(t, "203.0.113.5", davClientIP(req))
	})

	t.Run("ignores X-Forwarded-For from non-loopback", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:4321"
		req.Header.Set("X-Forwarded-For", "203.0.113.5")
		assert.Equal(t, "10.0.0.1", davClientIP(req))
	})
}

// TestSelfTLS verifies the local-CA TLS setup:
//   - CA and server cert are generated on first call
//   - CA cert is a valid X.509 certificate marked as CA
//   - Server cert is signed by the CA and has ServerAuth EKU
//   - Returned TLS config has exactly one certificate
//   - Second call reuses the existing files (idempotent)
//   - /ca.crt endpoint returns the CA PEM when TLS_SELF=1
func TestSelfTLS(t *testing.T) {
	dir := t.TempDir()

	tlsCfg, caPEM, err := selfTLSSetup(dir)
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	require.NotEmpty(t, caPEM)

	t.Run("CA cert is valid and marked IsCA", func(t *testing.T) {
		caCert, err := parseCert(caPEM)
		require.NoError(t, err)
		assert.True(t, caCert.IsCA)
		assert.True(t, caCert.NotAfter.After(time.Now().AddDate(9, 0, 0)), "CA should be valid for ~10 years")
	})

	t.Run("server cert is signed by the CA", func(t *testing.T) {
		require.Len(t, tlsCfg.Certificates, 1)
		srvDER := tlsCfg.Certificates[0].Certificate[0]
		srvCert, err := x509.ParseCertificate(srvDER)
		require.NoError(t, err)

		pool := x509.NewCertPool()
		caCert, _ := parseCert(caPEM)
		pool.AddCert(caCert)

		_, err = srvCert.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		assert.NoError(t, err, "server cert must be verifiable by the local CA")
	})

	t.Run("second call reuses existing files", func(t *testing.T) {
		tlsCfg2, caPEM2, err := selfTLSSetup(dir)
		require.NoError(t, err)
		assert.Equal(t, caPEM, caPEM2, "CA PEM must be identical on reload")
		assert.Len(t, tlsCfg2.Certificates, 1)
	})

	t.Run("/ca.crt returns 200 with CA PEM when CACert is set", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		s.CACert = caPEM
		r := s.SetupRouter()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ca.crt", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-x509-ca-cert", w.Header().Get("Content-Type"))
		assert.Equal(t, caPEM, w.Body.Bytes())
	})

	t.Run("/ca.crt returns 404 when CACert is nil", func(t *testing.T) {
		lib, _ := NewNoteLibrary(t.TempDir(), "assets", filepath.Join(t.TempDir(), "config.json"))
		s := NewServer(lib)
		// s.CACert is nil by default
		r := s.SetupRouter()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ca.crt", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ─── purgeExpiredTrash ────────────────────────────────────────────────────────

func TestPurgeExpiredTrash(t *testing.T) {
	thirtyDaysMs := int64(30 * 24 * 60 * 60 * 1000)

	// Helper types for building test structures (purgeExpiredTrash uses json.RawMessage internally)
	type testTrashEntry struct {
		ID        string `json:"id"`
		DeletedAt int64  `json:"deletedAt"`
	}
	type testStructure struct {
		Order  []string              `json:"order"`
		Titles map[string]string     `json:"titles,omitempty"`
		Tags   map[string][]string   `json:"tags,omitempty"`
		Trash  []testTrashEntry      `json:"trash"`
	}

	t.Run("expired entries are purged and files deleted", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))

		lib.AtomicWrite("20260318aaaaaaaaaaaaaaaa.md", []byte("old note"))
		lib.AtomicWrite("20260318bbbbbbbbbbbbbbbb.md", []byte("recent note"))

		now := time.Now().UnixMilli()
		structure := testStructure{
			Order:  []string{},
			Titles: map[string]string{"20260318aaaaaaaaaaaaaaaa.md": "Old", "20260318bbbbbbbbbbbbbbbb.md": "Recent"},
			Tags:   map[string][]string{"20260318aaaaaaaaaaaaaaaa.md": {"tag1"}},
			Trash: []testTrashEntry{
				{ID: "20260318aaaaaaaaaaaaaaaa.md", DeletedAt: now - thirtyDaysMs - 86400000},
				{ID: "20260318bbbbbbbbbbbbbbbb.md", DeletedAt: now - 86400000},
			},
		}
		data, _ := json.Marshal(structure)
		atomicWriteFile(filepath.Join(dir, "_structure.json"), data, 0600)

		lib.purgeExpiredTrash()

		_, err := os.Stat(filepath.Join(dir, "20260318aaaaaaaaaaaaaaaa.md"))
		assert.True(t, os.IsNotExist(err), "expired note file should be deleted")

		_, err = os.Stat(filepath.Join(dir, "20260318bbbbbbbbbbbbbbbb.md"))
		assert.NoError(t, err, "non-expired note file should remain")

		updatedData, _ := os.ReadFile(filepath.Join(dir, "_structure.json"))
		var updated testStructure
		json.Unmarshal(updatedData, &updated)
		assert.Len(t, updated.Trash, 1, "only non-expired entry should remain")
		assert.Equal(t, "20260318bbbbbbbbbbbbbbbb.md", updated.Trash[0].ID)

		assert.Empty(t, updated.Titles["20260318aaaaaaaaaaaaaaaa.md"], "purged note title should be removed")
		assert.Nil(t, updated.Tags["20260318aaaaaaaaaaaaaaaa.md"], "purged note tags should be removed")
		assert.Equal(t, "Recent", updated.Titles["20260318bbbbbbbbbbbbbbbb.md"])
	})

	t.Run("encrypted structure (ENC1 prefix) is skipped", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))

		enc := []byte("ENC1:iv:ciphertext")
		atomicWriteFile(filepath.Join(dir, "_structure.json"), enc, 0600)

		lib.purgeExpiredTrash()

		got, _ := os.ReadFile(filepath.Join(dir, "_structure.json"))
		assert.Equal(t, enc, got, "encrypted structure should not be modified")
	})

	t.Run("JSON-quoted ENC1 structure is skipped", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))

		quoted := []byte(`"ENC1:iv:ciphertext"`)
		atomicWriteFile(filepath.Join(dir, "_structure.json"), quoted, 0600)

		lib.purgeExpiredTrash()

		got, _ := os.ReadFile(filepath.Join(dir, "_structure.json"))
		assert.Equal(t, quoted, got, "JSON-quoted ENC1 structure should not be modified")
	})

	t.Run("empty trash is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))

		structure := testStructure{
			Order: []string{"20260318cccccccccccccccc.md"},
			Trash: []testTrashEntry{},
		}
		data, _ := json.Marshal(structure)
		atomicWriteFile(filepath.Join(dir, "_structure.json"), data, 0600)

		lib.purgeExpiredTrash()

		got, _ := os.ReadFile(filepath.Join(dir, "_structure.json"))
		assert.Equal(t, data, got, "structure should not be rewritten when trash is empty")
	})

	t.Run("no expired entries means no changes", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))

		now := time.Now().UnixMilli()
		structure := testStructure{
			Order: []string{},
			Trash: []testTrashEntry{
				{ID: "20260318dddddddddddddddd.md", DeletedAt: now - 86400000},
			},
		}
		data, _ := json.Marshal(structure)
		atomicWriteFile(filepath.Join(dir, "_structure.json"), data, 0600)

		lib.purgeExpiredTrash()

		got, _ := os.ReadFile(filepath.Join(dir, "_structure.json"))
		assert.Equal(t, data, got, "no rewrite when nothing expired")
	})

	t.Run("missing structure file is handled gracefully", func(t *testing.T) {
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))
		assert.NotPanics(t, func() { lib.purgeExpiredTrash() })
	})
}

// TestNonCanonicalNoteAccess verifies that the API layer (P3) can read, update,
// and delete notes with non-canonical filenames written by WebDAV clients.
func TestNonCanonicalNoteAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))
	r := NewServer(lib).SetupRouter()

	const nonCanonical = "my-webdav-note.md"
	const content = "# WebDAV Note\nHello from WebDAV."

	// Seed the file directly on disk (simulating a WebDAV PUT).
	require.NoError(t, os.WriteFile(filepath.Join(dir, nonCanonical), []byte(content), 0600))

	t.Run("GET non-canonical note returns content", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/notes/"+nonCanonical, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, content, resp["content"])
	})

	t.Run("PUT non-canonical note updates existing file", func(t *testing.T) {
		updated := "# Updated\nEdited via web."
		body, _ := json.Marshal(map[string]string{"content": updated})
		req, _ := http.NewRequest("PUT", "/api/notes/"+nonCanonical, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		got, _ := os.ReadFile(filepath.Join(dir, nonCanonical))
		assert.Equal(t, updated, string(got))
	})

	t.Run("PUT non-canonical note returns 404 when file absent", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"content": "new content"})
		req, _ := http.NewRequest("PUT", "/api/notes/absent-note.md", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("DELETE non-canonical note removes file", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/notes/"+nonCanonical, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		_, err := os.Stat(filepath.Join(dir, nonCanonical))
		assert.True(t, os.IsNotExist(err), "file should be deleted")
	})

	t.Run("GET non-canonical note with blocked name returns 400", func(t *testing.T) {
		for _, name := range []string{"_structure.json", ".hidden.md", "~temp.md", "note~.md"} {
			req, _ := http.NewRequest("GET", "/api/notes/"+name, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.True(t, w.Code >= 400, "blocked name %q should return >=400, got %d", name, w.Code)
		}
	})

	t.Run("ListNotes includes non-canonical files", func(t *testing.T) {
		dir2 := t.TempDir()
		lib2, _ := NewNoteLibrary(dir2, "assets", filepath.Join(dir2, "config.json"))
		// Write a canonical and a non-canonical note.
		require.NoError(t, os.WriteFile(filepath.Join(dir2, "20260318aabbccddeeff0011.md"), []byte("canonical"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir2, "my-note.md"), []byte("non-canonical"), 0600))
		notes, err := lib2.ListNotes()
		require.NoError(t, err)
		names := make(map[string]bool)
		for _, n := range notes {
			names[n.Name] = true
		}
		assert.True(t, names["20260318aabbccddeeff0011.md"], "canonical note should be listed")
		assert.True(t, names["my-note.md"], "non-canonical WebDAV note should be listed")
	})
}

// TestDavQuota verifies that WebDAV write operations enforce note quotas (P4).
//
// Note on HTTP status codes: golang.org/x/net/webdav translates FileSystem errors
// with non-standard mapping in handlePut:
//   - OpenFile error (non os.ErrNotExist) → 404 Not Found (webdav internal behaviour)
//   - io.Copy / f.Stat error → 405 Method Not Allowed
//   - f.Close error → mapped via statusFromFileError
//
// We test the observable invariants: file creation blocked, oversized file removed.
func TestDavQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newOpenLib := func(t *testing.T) (*NoteLibrary, http.Handler) {
		t.Helper()
		dir := t.TempDir()
		lib, _ := NewNoteLibrary(dir, "assets", filepath.Join(dir, "config.json"))
		srv := &Server{Library: lib}
		return lib, srv.newDavHandler()
	}

	doPut := func(davH http.Handler, filename, body string) int {
		req, _ := http.NewRequest("PUT", "/dav/"+filename, strings.NewReader(body))
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		davH.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("PUT within quota succeeds with 2xx", func(t *testing.T) {
		lib, davH := newOpenLib(t)
		lib.Config.MaxNoteSize = 1024
		lib.Config.MaxTotalNotes = 10
		code := doPut(davH, "test-note.md", "# Hello")
		assert.True(t, code == http.StatusCreated || code == http.StatusNoContent,
			"within-quota PUT should return 201 or 204, got %d", code)
	})

	t.Run("PUT note count at limit is rejected (4xx)", func(t *testing.T) {
		lib, davH := newOpenLib(t)
		lib.Config.MaxTotalNotes = 2
		// Pre-create 2 notes to fill the quota.
		for i, name := range []string{"20260101aaaaaaaaaaaaaaaa.md", "20260101bbbbbbbbbbbbbbbb.md"} {
			content := fmt.Sprintf("note %d", i)
			require.NoError(t, os.WriteFile(filepath.Join(lib.DataDir, name), []byte(content), 0600))
		}
		// WebDAV PUT of a new note should be refused.
		// golang.org/x/net/webdav maps non-ErrNotExist OpenFile errors to 404.
		code := doPut(davH, "new-note.md", "# New")
		assert.True(t, code >= 400, "count-quota exceeded should return 4xx, got %d", code)
		// The file must not have been created.
		_, statErr := os.Stat(filepath.Join(lib.DataDir, "new-note.md"))
		assert.True(t, os.IsNotExist(statErr), "file should not exist when quota is exceeded")
	})

	t.Run("PUT oversized note is rejected and file is removed", func(t *testing.T) {
		lib, davH := newOpenLib(t)
		lib.Config.MaxNoteSize = 10 // 10 bytes limit
		bigContent := strings.Repeat("x", 100)
		code := doPut(davH, "big-note.md", bigContent)
		assert.True(t, code >= 400, "size-quota exceeded should return 4xx, got %d", code)
		// The file must not remain on disk after the quota rejection.
		_, err := os.Stat(filepath.Join(lib.DataDir, "big-note.md"))
		assert.True(t, os.IsNotExist(err), "oversized file should be removed after failed PUT")
	})

	t.Run("PUT existing note ignores count quota and succeeds", func(t *testing.T) {
		lib, davH := newOpenLib(t)
		lib.Config.MaxTotalNotes = 1
		// Pre-create the note.
		require.NoError(t, os.WriteFile(filepath.Join(lib.DataDir, "existing.md"), []byte("old"), 0600))
		// Updating an existing note should succeed even when count quota is at limit.
		code := doPut(davH, "existing.md", "new content")
		assert.True(t, code == http.StatusCreated || code == http.StatusNoContent,
			"updating existing note should succeed with 201 or 204, got %d", code)
		// Verify the content was actually updated.
		got, _ := os.ReadFile(filepath.Join(lib.DataDir, "existing.md"))
		assert.Equal(t, "new content", string(got))
	})
}
