package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HendryAvila/Hoofy/internal/changes"
	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- Test helpers for context-check ---

// setupContextCheckProject creates a temp dir with docs/hoofy.json (for findProjectRoot)
// and docs/ artifacts. The context-check tool uses findProjectRoot()
// which walks up looking for docs/hoofy.json, and scans artifacts from docs/.
func setupContextCheckProject(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create docs/ directory with a minimal hoofy.json (for findProjectRoot).
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir docs: %v", err)
	}
	hoofyJSON := `{"name":"test","description":"test project","mode":"guided"}`
	if err := os.WriteFile(filepath.Join(docsDir, "hoofy.json"), []byte(hoofyJSON), 0o644); err != nil {
		t.Fatalf("setup: write hoofy.json: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("setup: getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("setup: chdir: %v", err)
	}

	cleanup := func() { _ = os.Chdir(origDir) }
	return tmpDir, cleanup
}

// writeArtifact writes a file in the docs/ directory.
func writeArtifact(t *testing.T, projectRoot, filename, content string) {
	t.Helper()
	path := filepath.Join(projectRoot, "docs", filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write artifact %s: %v", filename, err)
	}
}

// createCompletedChange creates a completed change record in the filesystem.
func createCompletedChange(t *testing.T, projectRoot string, id string, ct changes.ChangeType, cs changes.ChangeSize, desc string) {
	t.Helper()

	changeDir := filepath.Join(projectRoot, "docs", "changes", id, "adrs")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatalf("create change dir: %v", err)
	}

	record := changes.ChangeRecord{
		ID:           id,
		Type:         ct,
		Size:         cs,
		Description:  desc,
		Stages:       []changes.StageEntry{},
		CurrentStage: changes.StageTasks,
		ADRs:         []string{},
		Status:       changes.StatusCompleted,
		CreatedAt:    "2025-01-01T00:00:00Z",
		UpdatedAt:    "2025-01-01T00:00:00Z",
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal change: %v", err)
	}

	configPath := filepath.Join(projectRoot, "docs", "changes", id, "change.json")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write change.json: %v", err)
	}
}

// contextCheckReq builds a CallToolRequest for sdd_context_check.
func contextCheckReq(args map[string]interface{}) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// newContextCheckMemStore creates a memory.Store for context-check tests.
func newContextCheckMemStore(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.New(memory.Config{
		DataDir:              t.TempDir(),
		MaxObservationLength: 2000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create test memory store: %v", err)
	}
	if err := store.CreateSession("manual-save", "", "/tmp/test"); err != nil {
		t.Fatalf("seed manual-save session: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// --- ContextCheckTool tests ---

func TestContextCheckTool_Definition(t *testing.T) {
	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)
	def := tool.Definition()

	if def.Name != "sdd_context_check" {
		t.Errorf("tool name = %q, want %q", def.Name, "sdd_context_check")
	}

	// Should have change_description (required), project_name (optional), detail_level (optional), and max_tokens (optional).
	props := def.InputSchema.Properties
	if len(props) != 4 {
		t.Errorf("parameter count = %d, want 4", len(props))
	}

	required := def.InputSchema.Required
	if len(required) != 1 || required[0] != "change_description" {
		t.Errorf("required = %v, want [change_description]", required)
	}
}

func TestContextCheckTool_Handle_EmptyDescription(t *testing.T) {
	_, cleanup := setupContextCheckProject(t)
	defer cleanup()

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatal("expected tool error for empty change_description")
	}
	text := getResultText(result)
	if !strings.Contains(text, "change_description") {
		t.Errorf("error should mention 'change_description': %s", text)
	}
}

func TestContextCheckTool_Handle_WithArtifacts(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Write SDD artifacts.
	writeArtifact(t, tmpDir, "charter.md", "# Charter\n\nBuild an authentication system.")
	writeArtifact(t, tmpDir, "requirements.md", "# Requirements\n\n- FR-001: User login\n- FR-002: Password reset")
	writeArtifact(t, tmpDir, "design.md", "# Design\n\nJWT-based auth with refresh tokens.")

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Add OAuth2 login support",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Should contain the "Context Check Report" header.
	if !strings.Contains(text, "# Context Check Report") {
		t.Error("result should contain 'Context Check Report' header")
	}

	// Should list found artifacts.
	if !strings.Contains(text, "charter.md") {
		t.Error("result should list charter.md artifact")
	}
	if !strings.Contains(text, "requirements.md") {
		t.Error("result should list requirements.md artifact")
	}
	if !strings.Contains(text, "design.md") {
		t.Error("result should list design.md artifact")
	}

	// Should contain artifact excerpts since artifacts were found.
	if !strings.Contains(text, "## Artifact Excerpts") {
		t.Error("result should contain Artifact Excerpts section")
	}
	if !strings.Contains(text, "authentication system") {
		t.Error("artifact excerpts should contain charter content")
	}

	// Convention files should be skipped when SDD artifacts exist.
	if !strings.Contains(text, "convention file scan skipped") {
		t.Error("convention files should be skipped when SDD artifacts present")
	}
}

func TestContextCheckTool_Handle_NoArtifacts_ConventionFallback(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Write convention files (NO SDD artifacts).
	claudeMD := "# CLAUDE.md\n\nConventional commits only.\nUse Go 1.25.\nNo CGO."
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	readmeMD := "# README\n\nThis is a Go project."
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeMD), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Add caching layer",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Should indicate no SDD artifacts.
	if !strings.Contains(text, "No SDD artifacts found") {
		t.Error("result should indicate no SDD artifacts")
	}

	// Should scan convention files as fallback.
	if !strings.Contains(text, "Scanning project conventions") {
		t.Error("result should scan conventions when no SDD artifacts")
	}
	if !strings.Contains(text, "CLAUDE.md") {
		t.Error("result should contain CLAUDE.md content")
	}
	if !strings.Contains(text, "Conventional commits") {
		t.Error("result should include CLAUDE.md content text")
	}
	if !strings.Contains(text, "README.md") {
		t.Error("result should contain README.md")
	}

	// Should NOT have artifact excerpts.
	if strings.Contains(text, "## Artifact Excerpts") {
		t.Error("result should NOT contain Artifact Excerpts when no artifacts found")
	}
}

func TestContextCheckTool_Handle_ConventionDir(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Create .cursor/rules/ with a rule file.
	cursorDir := filepath.Join(tmpDir, ".cursor", "rules")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatalf("create .cursor/rules: %v", err)
	}
	ruleContent := "Always use TypeScript strict mode.\nPrefer const over let."
	if err := os.WriteFile(filepath.Join(cursorDir, "typescript.md"), []byte(ruleContent), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Refactor TypeScript modules",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, ".cursor/rules/typescript.md") {
		t.Error("result should contain .cursor/rules/typescript.md")
	}
	if !strings.Contains(text, "TypeScript strict mode") {
		t.Error("result should include rule file content")
	}
}

func TestContextCheckTool_Handle_KeywordMatching(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Create completed changes.
	createCompletedChange(t, tmpDir, "fix-auth-login-bug", changes.TypeFix, changes.SizeSmall,
		"Fix authentication login bug when password is empty")
	createCompletedChange(t, tmpDir, "add-oauth-support", changes.TypeFeature, changes.SizeMedium,
		"Add OAuth2 support for third-party login")
	createCompletedChange(t, tmpDir, "refactor-database-layer", changes.TypeRefactor, changes.SizeLarge,
		"Refactor database connection pooling")

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	// Search for something related to auth/login.
	req := contextCheckReq(map[string]interface{}{
		"change_description": "Update login authentication flow",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Should match changes with "login" and "authentication" keywords.
	if !strings.Contains(text, "fix-auth-login-bug") {
		t.Error("result should match fix-auth-login-bug (shares 'login' and 'authentication' keywords)")
	}
	if !strings.Contains(text, "add-oauth-support") {
		t.Error("result should match add-oauth-support (shares 'login' keyword)")
	}

	// Database refactor should NOT match — no keyword overlap.
	if strings.Contains(text, "refactor-database-layer") {
		t.Error("result should NOT match refactor-database-layer (no keyword overlap with auth/login)")
	}
}

func TestContextCheckTool_Handle_NoMatchingChanges(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Create a change with totally unrelated description.
	createCompletedChange(t, tmpDir, "add-dark-mode", changes.TypeFeature, changes.SizeSmall,
		"Add dark mode toggle to settings page")

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Fix database migration error",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "No completed changes found matching") {
		t.Error("result should indicate no matching changes")
	}
}

func TestContextCheckTool_Handle_NilMemoryStore(t *testing.T) {
	_, cleanup := setupContextCheckProject(t)
	defer cleanup()

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil) // nil memory store

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Some change",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error with nil memory: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error with nil memory: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Memory subsystem not available") {
		t.Error("result should indicate memory not available")
	}
}

func TestContextCheckTool_Handle_WithMemoryStore(t *testing.T) {
	_, cleanup := setupContextCheckProject(t)
	defer cleanup()

	memStore := newContextCheckMemStore(t)

	// Save an explore observation to memory.
	if _, err := memStore.AddObservation(memory.AddObservationParams{
		SessionID: "manual-save",
		Type:      "explore",
		Title:     "Auth system exploration",
		Content:   "Exploring JWT vs session-based authentication patterns",
		Scope:     "project",
	}); err != nil {
		t.Fatalf("save explore observation: %v", err)
	}

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, memStore)

	// Use terms that actually appear in the observation content.
	// FTS5 with sanitizeFTS does implicit AND, so all terms must match.
	req := contextCheckReq(map[string]interface{}{
		"change_description": "JWT authentication patterns",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Should contain explore context from memory.
	if !strings.Contains(text, "Auth system exploration") {
		t.Errorf("result should contain explore observation title from memory, got:\n%s", text)
	}
}

func TestContextCheckTool_Handle_WithMemoryStoreAndProjectFilter(t *testing.T) {
	_, cleanup := setupContextCheckProject(t)
	defer cleanup()

	memStore := newContextCheckMemStore(t)

	// Save observations for different projects.
	projectA := "project-alpha"
	projectB := "project-beta"

	if _, err := memStore.AddObservation(memory.AddObservationParams{
		SessionID: "manual-save",
		Type:      "explore",
		Title:     "Alpha auth design",
		Content:   "Designing authentication for project alpha",
		Scope:     "project",
		Project:   projectA,
	}); err != nil {
		t.Fatalf("save alpha observation: %v", err)
	}
	if _, err := memStore.AddObservation(memory.AddObservationParams{
		SessionID: "manual-save",
		Type:      "explore",
		Title:     "Beta auth design",
		Content:   "Designing authentication for project beta",
		Scope:     "project",
		Project:   projectB,
	}); err != nil {
		t.Fatalf("save beta observation: %v", err)
	}

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, memStore)

	// Filter by project-alpha. Use terms that appear in the observation content.
	req := contextCheckReq(map[string]interface{}{
		"change_description": "authentication design project",
		"project_name":       "project-alpha",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Alpha auth design") {
		t.Error("result should contain project-alpha observation")
	}
	// project-beta observation should NOT appear when filtered.
	if strings.Contains(text, "Beta auth design") {
		t.Error("result should NOT contain project-beta observation when filtering by project-alpha")
	}
}

func TestContextCheckTool_Handle_BusinessRulesArtifact(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// business-rules.md should be scanned as an SDD artifact.
	writeArtifact(t, tmpDir, "business-rules.md",
		"# Business Rules\n\n## Definitions\n\n- **Customer**: A person who completed a purchase\n\n## Constraints\n\n- When Order total > $500, manager approval required")

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Change order approval workflow",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "business-rules.md") {
		t.Error("result should list business-rules.md as found artifact")
	}
	if !strings.Contains(text, "manager approval") {
		t.Error("artifact excerpts should contain business rules content")
	}
}

func TestContextCheckTool_Handle_EmptyProject(t *testing.T) {
	_, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// No artifacts, no convention files, no changes — completely empty.
	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "First change ever",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)
	// Should still produce a valid report.
	if !strings.Contains(text, "# Context Check Report") {
		t.Error("result should contain report header even for empty project")
	}
	if !strings.Contains(text, "No SDD artifacts found") {
		t.Error("result should indicate no SDD artifacts")
	}
	if !strings.Contains(text, "No completed changes found") {
		t.Error("result should indicate no matching changes")
	}
	if !strings.Contains(text, "No convention files found") {
		t.Error("result should indicate no convention files")
	}
}

// --- Detail level tests for context-check ---

func TestContextCheckTool_Handle_SummaryLevel(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	// Write SDD artifacts.
	writeArtifact(t, tmpDir, "charter.md", "# Charter\n\nBuild an authentication system with OAuth2 and JWT.")
	writeArtifact(t, tmpDir, "requirements.md", "# Requirements\n\n- FR-001: User login\n- FR-002: Password reset")

	cs := changes.NewFileStore()
	memStore := newContextCheckMemStore(t)

	// Save an explore observation.
	if _, err := memStore.AddObservation(memory.AddObservationParams{
		SessionID: "manual-save",
		Type:      "explore",
		Title:     "Auth exploration",
		Content:   "Exploring JWT authentication patterns with session-based comparison and detailed analysis",
		Scope:     "project",
	}); err != nil {
		t.Fatalf("save explore observation: %v", err)
	}

	tool := NewContextCheckTool(cs, memStore)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "JWT authentication patterns",
		"detail_level":       "summary",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Summary should show artifact names and sizes but NOT excerpts
	if !strings.Contains(text, "charter.md") {
		t.Error("summary should list artifact names")
	}
	if strings.Contains(text, "## Artifact Excerpts") {
		t.Error("summary should NOT contain Artifact Excerpts section")
	}
	if strings.Contains(text, "authentication system") {
		t.Error("summary should NOT contain artifact content")
	}

	// Memory results should show title but not full content
	if !strings.Contains(text, "Auth exploration") {
		t.Error("summary should show memory observation title")
	}
	if strings.Contains(text, "detailed analysis") {
		t.Error("summary should NOT show memory observation full content")
	}

	// Should have footer hint
	if !strings.Contains(text, "detail_level") {
		t.Error("summary should have footer hint about detail_level")
	}
}

func TestContextCheckTool_Handle_FullLevel(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	artifactContent := strings.Repeat("Full artifact content block. ", 30) // 870+ chars
	writeArtifact(t, tmpDir, "charter.md", "# Charter\n\n"+artifactContent)

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Update proposal",
		"detail_level":       "full",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Full mode should have Artifact Excerpts with complete untruncated content
	if !strings.Contains(text, "## Artifact Excerpts") {
		t.Error("full mode should contain Artifact Excerpts section")
	}
	if !strings.Contains(text, artifactContent) {
		t.Error("full mode should contain complete untruncated artifact content")
	}
	// Should NOT have truncation markers
	if strings.Contains(text, "[...truncated]") {
		t.Error("full mode should NOT have truncation markers")
	}
	// Should NOT have footer hint
	if strings.Contains(text, "💡") {
		t.Error("full mode should NOT have footer hint")
	}
}

func TestContextCheckTool_Handle_StandardLevel(t *testing.T) {
	tmpDir, cleanup := setupContextCheckProject(t)
	defer cleanup()

	artifactContent := strings.Repeat("Standard artifact content block. ", 30) // 960+ chars
	writeArtifact(t, tmpDir, "charter.md", "# Charter\n\n"+artifactContent)

	cs := changes.NewFileStore()
	tool := NewContextCheckTool(cs, nil)

	req := contextCheckReq(map[string]interface{}{
		"change_description": "Update proposal",
		"detail_level":       "standard",
	})

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Standard mode should have Artifact Excerpts but truncated
	if !strings.Contains(text, "## Artifact Excerpts") {
		t.Error("standard mode should contain Artifact Excerpts section")
	}
	if !strings.Contains(text, "Standard artifact content block") {
		t.Error("standard mode should contain beginning of content")
	}
	// Content should be truncated (500 chars max for standard artifacts)
	if strings.Contains(text, artifactContent) {
		t.Error("standard mode should truncate long artifact content")
	}
}

// --- Unit tests for private helpers ---

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Fix authentication login bug",
			expected: []string{"fix", "authentication", "login", "bug"},
		},
		{
			input:    "the and for are but", // all stop words
			expected: nil,
		},
		{
			input:    "Add OAuth2 support for third-party login",
			expected: []string{"add", "oauth2", "support", "third-party", "login"},
		},
		{
			input:    "ab", // too short (< 3 chars)
			expected: nil,
		},
		{
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractKeywords(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("extractKeywords(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.expected, len(tt.expected))
				return
			}
			for i, kw := range got {
				if kw != tt.expected[i] {
					t.Errorf("keyword[%d] = %q, want %q", i, kw, tt.expected[i])
				}
			}
		})
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short content unchanged",
			input:  "Hello world",
			maxLen: 100,
			want:   "Hello world",
		},
		{
			name:   "truncated at line boundary",
			input:  "Line one\nLine two\nLine three\nLine four is very long and exceeds the limit",
			maxLen: 30,
			want:   "Line one\nLine two\nLine three\n[...truncated]",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateContent(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadFirstLines(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with 10 lines.
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, "Line "+string(rune('0'+i)))
	}
	content := strings.Join(lines, "\n")
	path := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Read only first 3 lines.
	got := readFirstLines(path, 3)
	lineCount := strings.Count(got, "\n") + 1
	if lineCount > 3 {
		t.Errorf("readFirstLines(3) returned %d lines, want <= 3", lineCount)
	}

	// Non-existent file returns empty string.
	got = readFirstLines(filepath.Join(tmpDir, "nonexistent.md"), 10)
	if got != "" {
		t.Errorf("readFirstLines(nonexistent) = %q, want empty", got)
	}
}
