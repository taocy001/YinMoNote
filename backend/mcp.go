package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 {
			return val
		}
	}
	return fallback
}

// ── Session management ────────────────────────────────────────────────────────

// mcpSession represents one connected MCP client (one SSE stream).
type mcpSession struct {
	id    string
	msgCh chan string    // outbound SSE messages (JSON-RPC responses)
	done  chan struct{}  // closed when the SSE connection drops
}

var (
	mcpSessionsMu sync.Mutex
	mcpSessions   = map[string]*mcpSession{}
	mcpMaxSessions = getEnvInt("MCP_MAX_SESSIONS", 50)
)

func newMCPSession(id string) *mcpSession {
	return &mcpSession{
		id:    id,
		msgCh: make(chan string, 32),
		done:  make(chan struct{}),
	}
}

func storeMCPSession(sess *mcpSession) bool {
	mcpSessionsMu.Lock()
	defer mcpSessionsMu.Unlock()
	if len(mcpSessions) >= mcpMaxSessions {
		return false
	}
	mcpSessions[sess.id] = sess
	return true
}

func removeMCPSession(id string) {
	mcpSessionsMu.Lock()
	delete(mcpSessions, id)
	mcpSessionsMu.Unlock()
}

func getMCPSession(id string) (*mcpSession, bool) {
	mcpSessionsMu.Lock()
	s, ok := mcpSessions[id]
	mcpSessionsMu.Unlock()
	return s, ok
}

// ── Auth middleware ───────────────────────────────────────────────────────────

// mcpAuth is the authentication middleware for all /mcp/* endpoints.
// It requires a Bearer token whose SHA-256 hex matches MCPTokenHash.
// Uses the same progressive-delay helpers as apiAuth to slow brute force.
func (s *Server) mcpAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.Library.mu.Lock()
		tokenHash := s.Library.Config.MCPTokenHash
		s.Library.mu.Unlock()

		if tokenHash == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable,
				gin.H{"error": "mcp_not_configured"})
			return
		}

		ip := c.ClientIP()
		applyAuthDelay(ip)

		auth := c.GetHeader("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") || token == "" {
			recordAuthFailure(ip)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		h := sha256.Sum256([]byte(token))
		gotHash := hex.EncodeToString(h[:])
		if subtle.ConstantTimeCompare([]byte(gotHash), []byte(tokenHash)) != 1 {
			recordAuthFailure(ip)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		clearAuthFailures(ip)
		c.Next()
	}
}

// ── SSE transport ─────────────────────────────────────────────────────────────

// handleMCPSSE opens a persistent Server-Sent Events stream for one MCP client.
// On connect it emits an "endpoint" event telling the client where to POST messages.
// Subsequent tool responses are pushed as "message" events through the same stream.
func (s *Server) handleMCPSSE(c *gin.Context) {
	sessID := randomString(24)
	sess := newMCPSession(sessID)
	if !storeMCPSession(sess) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "too_many_connections"})
		return
	}
	defer func() {
		removeMCPSession(sessID)
		close(sess.done)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // disable Nginx buffering

	// Tell the client where to POST JSON-RPC messages.
	fmt.Fprintf(c.Writer, "event: endpoint\ndata: /mcp/messages?sessionId=%s\n\n", sessID)
	c.Writer.Flush()

	// Heartbeat ticker — keeps the SSE connection alive through idle periods and
	// lets us detect dead clients before they time out at the TCP layer.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-sess.msgCh:
			fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", msg)
			c.Writer.Flush()
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": ping\n\n") // SSE comment keeps connection alive
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return // client disconnected
		}
	}
}

// handleMCPMessage receives a JSON-RPC 2.0 request from the client,
// dispatches it to the appropriate handler, and pushes the response
// back through the client's SSE stream.
func (s *Server) handleMCPMessage(c *gin.Context) {
	sessID := c.Query("sessionId")
	sess, ok := getMCPSession(sessID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found"})
		return
	}

	var req mcpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	resp := s.dispatchMCP(req)

	// Notifications (id == nil) don't require a response.
	if resp == nil {
		c.Status(http.StatusAccepted)
		return
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	select {
	case sess.msgCh <- string(data):
	case <-sess.done:
	default: // channel full — drop and let client retry
	}
	c.Status(http.StatusAccepted)
}

// ── JSON-RPC types ────────────────────────────────────────────────────────────

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *mcpError        `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func mcpOK(id *json.RawMessage, result interface{}) *mcpResponse {
	return &mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
}
func mcpErr(id *json.RawMessage, code int, msg string) *mcpResponse {
	return &mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: msg}}
}

// ── Dispatcher ────────────────────────────────────────────────────────────────

func (s *Server) dispatchMCP(req mcpRequest) *mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpOK(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "YinMoNote", "version": "1.0"},
		})

	case "notifications/initialized":
		return nil // notification — no response

	case "ping":
		return mcpOK(req.ID, map[string]interface{}{})

	case "tools/list":
		return mcpOK(req.ID, map[string]interface{}{"tools": mcpToolDefs(s)})

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return mcpErr(req.ID, -32602, "invalid params")
		}
		return s.callMCPTool(req.ID, p.Name, p.Arguments)

	default:
		return mcpErr(req.ID, -32601, "method not found: "+req.Method)
	}
}

// ── Tool definitions ──────────────────────────────────────────────────────────

func mcpToolDefs(s *Server) []map[string]interface{} {
	s.Library.mu.Lock()
	policy := s.Library.Config.MCPPolicy
	s.Library.mu.Unlock()

	tools := []map[string]interface{}{
		{
			"name":        "list_notes",
			"description": "List all notes accessible to AI, with titles and tags.",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}},
		},
		{
			"name":        "read_note",
			"description": "Read the full content of a note by its filename.",
			"inputSchema": map[string]interface{}{
				"type":     "object",
				"required": []string{"filename"},
				"properties": map[string]interface{}{
					"filename": map[string]interface{}{"type": "string", "description": "Note filename, e.g. 20240101abc123456789.md"},
				},
			},
		},
		{
			"name":        "get_structure",
			"description": "Get the hierarchical folder/note structure of the library.",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}},
		},
		{
			"name":        "search_notes",
			"description": "Search notes by title or tag. Returns matching accessible notes.",
			"inputSchema": map[string]interface{}{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]interface{}{
					"query":     map[string]interface{}{"type": "string"},
					"search_in": map[string]interface{}{"type": "string", "enum": []string{"title", "tag", "both"}, "description": "Default: both"},
				},
			},
		},
		{
			"name":        "get_note_history",
			"description": "Get the git commit history for a specific note.",
			"inputSchema": map[string]interface{}{
				"type":     "object",
				"required": []string{"filename"},
				"properties": map[string]interface{}{
					"filename": map[string]interface{}{"type": "string"},
					"limit":    map[string]interface{}{"type": "integer", "description": "Max commits to return (default 20, max 50)"},
				},
			},
		},
		{
			"name":        "read_note_version",
			"description": "Read the content of a note at a specific git commit.",
			"inputSchema": map[string]interface{}{
				"type":     "object",
				"required": []string{"filename", "hash"},
				"properties": map[string]interface{}{
					"filename": map[string]interface{}{"type": "string"},
					"hash":     map[string]interface{}{"type": "string", "description": "40-character git commit SHA"},
				},
			},
		},
	}

	// Only advertise write tools when the policy could allow writes.
	if !policy.Enabled || policy.DefaultAccess == MCPAccessWrite || hasWriteRule(policy) {
		tools = append(tools,
			map[string]interface{}{
				"name":        "update_note",
				"description": "Overwrite the content of an existing note. Only succeeds when the note has write access.",
				"inputSchema": map[string]interface{}{
					"type":     "object",
					"required": []string{"filename", "content"},
					"properties": map[string]interface{}{
						"filename": map[string]interface{}{"type": "string"},
						"content":  map[string]interface{}{"type": "string", "description": "New full Markdown content"},
					},
				},
			},
		)
	}

	return tools
}

func hasWriteRule(p MCPPolicy) bool {
	for _, r := range p.Rules {
		if r.Access == MCPAccessWrite {
			return true
		}
	}
	return false
}

// ── Tool execution ────────────────────────────────────────────────────────────

func (s *Server) callMCPTool(id *json.RawMessage, name string, args json.RawMessage) *mcpResponse {
	switch name {
	case "list_notes":
		return s.mcpListNotes(id)
	case "read_note":
		var a struct{ Filename string `json:"filename"` }
		if err := json.Unmarshal(args, &a); err != nil {
			return mcpErr(id, -32602, "invalid params")
		}
		return s.mcpReadNote(id, a.Filename)
	case "get_structure":
		return s.mcpGetStructure(id)
	case "search_notes":
		var a struct {
			Query    string `json:"query"`
			SearchIn string `json:"search_in"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return mcpErr(id, -32602, "invalid params")
		}
		switch a.SearchIn {
		case "title", "tag", "both", "":
			if a.SearchIn == "" {
				a.SearchIn = "both"
			}
		default:
			return mcpErr(id, -32602, "invalid search_in: must be 'title', 'tag', or 'both'")
		}
		return s.mcpSearchNotes(id, a.Query, a.SearchIn)
	case "get_note_history":
		var a struct {
			Filename string `json:"filename"`
			Limit    int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return mcpErr(id, -32602, "invalid params")
		}
		if a.Limit <= 0 || a.Limit > 50 {
			a.Limit = 20
		}
		return s.mcpGetHistory(id, a.Filename, a.Limit)
	case "read_note_version":
		var a struct {
			Filename string `json:"filename"`
			Hash     string `json:"hash"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return mcpErr(id, -32602, "invalid params")
		}
		return s.mcpReadVersion(id, a.Filename, a.Hash)
	case "update_note":
		var a struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return mcpErr(id, -32602, "invalid params")
		}
		return s.mcpUpdateNote(id, a.Filename, a.Content)
	default:
		return mcpErr(id, -32601, "unknown tool: "+name)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func textResult(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{{"type": "text", "text": text}},
	}
}

func errResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"content":  []map[string]interface{}{{"type": "text", "text": msg}},
		"isError":  true,
	}
}

// mcpNoteAccess returns (title, tags, access) for a given note filename using
// the current policy and structure. The file's title is extracted from disk
// (falls back to the structure map when available).
func (s *Server) mcpNoteAccess(filename string, st mcpStructure) (title string, tags []string, access MCPAccessLevel) {
	s.Library.mu.Lock()
	policy := s.Library.Config.MCPPolicy
	s.Library.mu.Unlock()

	title = extractNoteTitle(s.Library.FullPath(filename))
	if title == "" {
		title = st.NoteTitles[filename]
	}
	tags = st.NoteTags[filename]
	if tags == nil {
		tags = []string{}
	}
	access = evaluateMCPAccess(filename, title, tags, policy, st)
	return
}

// ── Tool handlers ─────────────────────────────────────────────────────────────

func (s *Server) mcpListNotes(id *json.RawMessage) *mcpResponse {
	notes, err := s.Library.ListNotes()
	if err != nil {
		return mcpOK(id, errResult("Failed to list notes: "+err.Error()))
	}
	st := s.Library.loadMCPStructure()

	var sb strings.Builder
	visible := 0
	for _, n := range notes {
		title, tags, access := s.mcpNoteAccess(n.Name, st)
		if access == MCPAccessDeny {
			continue
		}
		visible++
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		perm := "read"
		if access == MCPAccessWrite {
			perm = "read+write"
		}
		modTime := time.UnixMilli(n.ModTime).Format("2006-01-02 15:04")
		sb.WriteString(fmt.Sprintf("- %s | %s%s | %s | %s\n",
			n.Name, title, tagStr, modTime, perm))
	}
	header := fmt.Sprintf("Notes (%d visible):\n", visible)
	return mcpOK(id, textResult(header+sb.String()))
}

func (s *Server) mcpReadNote(id *json.RawMessage, filename string) *mcpResponse {
	if !s.Library.IsValidName(filename) || filename == "_structure.json" {
		return mcpOK(id, errResult("note not found"))
	}
	st := s.Library.loadMCPStructure()
	_, _, access := s.mcpNoteAccess(filename, st)
	if access == MCPAccessDeny {
		return mcpOK(id, errResult("note not found")) // hide existence of denied notes
	}

	data, err := os.ReadFile(s.Library.FullPath(filename))
	if err != nil {
		if os.IsNotExist(err) {
			return mcpOK(id, errResult("note not found"))
		}
		return mcpOK(id, errResult("read failed"))
	}
	content := string(data)
	if strings.HasPrefix(content, "ENC1:") {
		return mcpOK(id, textResult("[This note is end-to-end encrypted and cannot be read by the MCP server. "+
			"Switch to Keyless mode or disable Client Encryption to make notes accessible.]"))
	}
	return mcpOK(id, textResult(content))
}

func (s *Server) mcpGetStructure(id *json.RawMessage) *mcpResponse {
	st := s.Library.loadMCPStructure()

	s.Library.mu.Lock()
	policy := s.Library.Config.MCPPolicy
	s.Library.mu.Unlock()

	// Build a set of all denied note IDs so we can prune the tree.
	// We enumerate every note that appears in the structure (Order + all ChildOrder
	// entries) rather than just those with frontend-supplied NoteTitles metadata.
	// A note without a NoteTitles entry would otherwise bypass the denied check
	// entirely and leak its presence through the rendered tree.
	denied := map[string]bool{}
	if policy.Enabled {
		allStructureIDs := map[string]bool{}
		for _, id := range st.Order {
			allStructureIDs[id] = true
		}
		for _, children := range st.ChildOrder {
			for _, child := range children {
				allStructureIDs[child] = true
			}
		}
		for noteID := range allStructureIDs {
			title := extractNoteTitle(s.Library.FullPath(noteID))
			if title == "" {
				title = st.NoteTitles[noteID]
			}
			tags := st.NoteTags[noteID]
			if tags == nil {
				tags = []string{}
			}
			if evaluateMCPAccess(noteID, title, tags, policy, st) == MCPAccessDeny {
				denied[noteID] = true
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("Note Hierarchy:\n")

	var renderNode func(id string, depth int)
	renderNode = func(nodeID string, depth int) {
		if denied[nodeID] {
			return
		}
		indent := strings.Repeat("  ", depth)
		title := st.NoteTitles[nodeID]
		if title == "" {
			title = extractNoteTitle(s.Library.FullPath(nodeID))
		}
		if title == "" {
			title = nodeID
		}
		sb.WriteString(fmt.Sprintf("%s- %s (%s)\n", indent, title, nodeID))
		for _, child := range st.ChildOrder[nodeID] {
			renderNode(child, depth+1)
		}
	}

	for _, rootID := range st.Order {
		renderNode(rootID, 0)
	}

	if sb.Len() == len("Note Hierarchy:\n") {
		sb.WriteString("(empty library or structure is encrypted)\n")
	}
	return mcpOK(id, textResult(sb.String()))
}

func (s *Server) mcpSearchNotes(id *json.RawMessage, query, searchIn string) *mcpResponse {
	if query == "" {
		return mcpOK(id, errResult("query must not be empty"))
	}
	notes, err := s.Library.ListNotes()
	if err != nil {
		return mcpOK(id, errResult("list failed"))
	}
	st := s.Library.loadMCPStructure()
	q := strings.ToLower(query)

	var sb strings.Builder
	count := 0
	for _, n := range notes {
		title, tags, access := s.mcpNoteAccess(n.Name, st)
		if access == MCPAccessDeny {
			continue
		}
		matchTitle := strings.Contains(strings.ToLower(title), q)
		matchTag := false
		for _, t := range tags {
			if strings.Contains(strings.ToLower(t), q) {
				matchTag = true
				break
			}
		}
		hit := (searchIn == "title" && matchTitle) ||
			(searchIn == "tag" && matchTag) ||
			(searchIn == "both" && (matchTitle || matchTag))
		if !hit {
			continue
		}
		count++
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		sb.WriteString(fmt.Sprintf("- %s | %s%s\n", n.Name, title, tagStr))
	}
	header := fmt.Sprintf("Search results for %q (%d matches):\n", query, count)
	return mcpOK(id, textResult(header+sb.String()))
}

func (s *Server) mcpGetHistory(id *json.RawMessage, filename string, limit int) *mcpResponse {
	if !s.Library.IsValidName(filename) || filename == "_structure.json" {
		return mcpOK(id, errResult("note not found"))
	}
	st := s.Library.loadMCPStructure()
	_, _, access := s.mcpNoteAccess(filename, st)
	if access == MCPAccessDeny {
		return mcpOK(id, errResult("note not found"))
	}

	history := s.Library.GetHistory(filename, limit)
	if len(history) == 0 {
		return mcpOK(id, textResult("No history found for "+filename))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("History for %s (%d commits):\n", filename, len(history)))
	for _, c := range history {
		sb.WriteString(fmt.Sprintf("- %s | %s | %s\n",
			c.Hash[:8], c.Date.Format("2006-01-02 15:04"), c.Message))
	}
	return mcpOK(id, textResult(sb.String()))
}

func (s *Server) mcpReadVersion(id *json.RawMessage, filename, hash string) *mcpResponse {
	if !s.Library.IsValidName(filename) || filename == "_structure.json" {
		return mcpOK(id, errResult("note not found"))
	}
	if !validGitHashRegex.MatchString(hash) {
		return mcpOK(id, errResult("invalid git hash"))
	}
	st := s.Library.loadMCPStructure()
	_, _, access := s.mcpNoteAccess(filename, st)
	if access == MCPAccessDeny {
		return mcpOK(id, errResult("note not found"))
	}

	content, found := s.Library.GetContentAtHash(filename, hash)
	if !found {
		return mcpOK(id, errResult("version not found"))
	}
	if strings.HasPrefix(content, "ENC1:") {
		return mcpOK(id, textResult("[encrypted version]"))
	}
	return mcpOK(id, textResult(fmt.Sprintf("Version %s of %s:\n\n%s", hash[:8], filename, content)))
}

func (s *Server) mcpUpdateNote(id *json.RawMessage, filename, content string) *mcpResponse {
	if !s.Library.IsValidName(filename) || filename == "_structure.json" {
		return mcpOK(id, errResult("note not found"))
	}
	// Note must already exist — MCP cannot create new notes through this tool.
	if _, err := os.Stat(s.Library.FullPath(filename)); os.IsNotExist(err) {
		return mcpOK(id, errResult("note not found"))
	}
	st := s.Library.loadMCPStructure()
	_, _, access := s.mcpNoteAccess(filename, st)
	if access != MCPAccessWrite {
		return mcpOK(id, errResult("note not found")) // hide existence; write not granted
	}
	if err := s.Library.CheckNoteQuota(filename, int64(len(content))); err != nil {
		return mcpOK(id, errResult("quota exceeded"))
	}
	if err := s.Library.SaveNote(filename, content); err != nil {
		return mcpOK(id, errResult("save failed"))
	}
	return mcpOK(id, textResult("Note updated successfully."))
}

// ── Token management API ──────────────────────────────────────────────────────

// handleMCPSetToken generates a new MCP bearer token server-side, stores its
// SHA-256 hash, and returns the raw token to the caller exactly once.
// The raw token is never stored — only the hash persists on disk.
func (s *Server) handleMCPSetToken(c *gin.Context) {
	// 32 bytes of CSPRNG → 48-char base36 string (via randomString, charset [a-z0-9])
	rawToken := randomString(48)
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	if err := s.persistMCPTokenHash(hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": rawToken})
}

// handleMCPRevokeToken removes the MCP token hash, disabling MCP access.
func (s *Server) handleMCPRevokeToken(c *gin.Context) {
	if err := s.persistMCPTokenHash(""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) persistMCPTokenHash(hash string) error {
	s.Library.mu.Lock()
	defer s.Library.mu.Unlock()
	oldHash := s.Library.Config.MCPTokenHash
	s.Library.Config.MCPTokenHash = hash
	data, err := json.MarshalIndent(s.Library.Config, "", "  ")
	if err != nil {
		s.Library.Config.MCPTokenHash = oldHash
		return err
	}
	if err := atomicWriteFile(s.Library.ConfigPath, data, 0600); err != nil {
		s.Library.Config.MCPTokenHash = oldHash
		return err
	}
	return nil
}

// ── CA fingerprint API ────────────────────────────────────────────────────────

// handleMCPCAFingerprint returns the SHA-256 fingerprint of the local CA cert.
// Only active when TLS_SELF=1 (s.CACert non-nil). Unauthenticated so the
// Settings UI can fetch it before the MCP token is configured.
func (s *Server) handleMCPCAFingerprint(c *gin.Context) {
	if s.CACert == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_self_signed"})
		return
	}
	cert, err := parseCert(s.CACert)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "parse_failed"})
		return
	}
	fp := sha256.Sum256(cert.Raw)
	// Format as colon-separated uppercase hex pairs (standard cert fingerprint notation).
	fpBytes := fp[:]
	pairs := make([]string, len(fpBytes))
	for i, b := range fpBytes {
		pairs[i] = fmt.Sprintf("%02X", b)
	}
	c.JSON(http.StatusOK, gin.H{
		"fingerprint": strings.Join(pairs, ":"),
		"not_after":   cert.NotAfter.Format(time.RFC3339),
	})
}
