package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- Test helpers ---

// setupTestProject creates a temp dir with an initialized SDD project
// and changes cwd to it. Returns the temp dir and a cleanup function.
func setupTestProject(t *testing.T, mode config.Mode) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	store := config.NewFileStore()
	cfg := config.NewProjectConfig("test-project", "A test project", mode)
	if err := store.Save(tmpDir, cfg); err != nil {
		t.Fatalf("setup: save config: %v", err)
	}

	// Save original working dir.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("setup: getwd: %v", err)
	}

	// Change to temp dir so findProjectRoot() works.
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("setup: chdir: %v", err)
	}

	cleanup := func() {
		_ = os.Chdir(origDir)
	}

	return tmpDir, cleanup
}

// setupTestProjectAtStage creates a project at a specific pipeline stage.
func setupTestProjectAtStage(t *testing.T, mode config.Mode, stage config.Stage) (string, func()) {
	t.Helper()
	tmpDir, cleanup := setupTestProject(t, mode)

	store := config.NewFileStore()
	cfg, err := store.Load(tmpDir)
	if err != nil {
		cleanup()
		t.Fatalf("setup: load config: %v", err)
	}

	// Advance to the desired stage by walking through the pipeline.
	cfg.ClarityScore = 100 // Bypass clarity gate if needed.
	for cfg.CurrentStage != stage {
		if err := pipeline.Advance(cfg); err != nil {
			cleanup()
			t.Fatalf("setup: advance to %s: %v", stage, err)
		}
	}

	if err := store.Save(tmpDir, cfg); err != nil {
		cleanup()
		t.Fatalf("setup: save config at stage %s: %v", stage, err)
	}

	return tmpDir, cleanup
}

// isErrorResult checks if the result is a tool error.
func isErrorResult(result *mcp.CallToolResult) bool {
	return result != nil && result.IsError
}

// getResultText extracts the text content from a CallToolResult.
func getResultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// mustRenderer creates a template renderer or fails the test.
func mustRenderer(t *testing.T) *templates.EmbedRenderer {
	t.Helper()
	r, err := templates.NewRenderer()
	if err != nil {
		t.Fatalf("setup: new renderer: %v", err)
	}
	return r
}

// --- InitTool ---

func TestInitTool_Handle_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "my-app",
		"description": "A cool app",
		"mode":        "guided",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "SDD Project Initialized") {
		t.Errorf("result should contain 'SDD Project Initialized', got: %s", text[:min(100, len(text))])
	}
	if !strings.Contains(text, "my-app") {
		t.Errorf("result should contain project name")
	}

	// Verify files were created.
	if !config.Exists(tmpDir) {
		t.Error("SDD config should exist after init")
	}

	// Verify sdd/history/ directory exists.
	historyDir := filepath.Join(tmpDir, "docs", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Error("docs/history/ directory should exist after init")
	}
}

func TestInitTool_Handle_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"description": "A cool app",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when name is missing")
	}
}

func TestInitTool_Handle_MissingDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name": "my-app",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when description is missing")
	}
}

func TestInitTool_Handle_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "my-app",
		"description": "A cool app",
		"mode":        "invalid",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for invalid mode")
	}
}

func TestInitTool_Handle_AlreadyExists(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "another-app",
		"description": "Another app",
		"mode":        "guided",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when project already exists")
	}
	text := getResultText(result)
	if !strings.Contains(text, "already exists") {
		t.Errorf("error should mention 'already exists': %s", text)
	}

	_ = tmpDir
}

func TestInitTool_Handle_ExpertMode(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "expert-app",
		"description": "An expert app",
		"mode":        "expert",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Expert") {
		t.Errorf("result should mention Expert mode, got: %s", text[:min(200, len(text))])
	}
}

func TestInitTool_Handle_CreatesAgentsFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "fresh-project",
		"description": "A brand new project",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// When neither CLAUDE.md nor AGENTS.md exists, AGENTS.md should be created.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md should exist: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## Hoofy SDD Project") {
		t.Error("AGENTS.md should contain '## Hoofy SDD Project' marker")
	}
	if !strings.Contains(content, "fresh-project") {
		t.Error("AGENTS.md should contain project name")
	}
	if !strings.Contains(content, "docs/") {
		t.Error("AGENTS.md should reference docs/ directory")
	}
}

func TestInitTool_Handle_AppendsToClaudeFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Pre-create CLAUDE.md with existing content.
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	existingContent := "# My Project Rules\n\nExisting instructions here.\n"
	if err := os.WriteFile(claudePath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("create CLAUDE.md: %v", err)
	}

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "claude-project",
		"description": "Uses CLAUDE.md",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Existing content should be preserved.
	if !strings.Contains(content, "Existing instructions here") {
		t.Error("CLAUDE.md should preserve existing content")
	}
	// Hoofy section should be appended.
	if !strings.Contains(content, "## Hoofy SDD Project") {
		t.Error("CLAUDE.md should have Hoofy section appended")
	}
	// AGENTS.md should NOT be created.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
		t.Error("AGENTS.md should NOT be created when CLAUDE.md exists")
	}
}

func TestInitTool_Handle_AppendsToAgentsFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Pre-create AGENTS.md with existing content (no CLAUDE.md).
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	existingContent := "# Agents Guide\n\nExisting agent instructions.\n"
	if err := os.WriteFile(agentsPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("create AGENTS.md: %v", err)
	}

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "agents-project",
		"description": "Uses AGENTS.md",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)

	// Existing content should be preserved.
	if !strings.Contains(content, "Existing agent instructions") {
		t.Error("AGENTS.md should preserve existing content")
	}
	// Hoofy section should be appended.
	if !strings.Contains(content, "## Hoofy SDD Project") {
		t.Error("AGENTS.md should have Hoofy section appended")
	}
}

func TestInitTool_Handle_AgentInstructions_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Pre-create AGENTS.md with a Hoofy section already present.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	existingContent := "# Agents Guide\n\n## Hoofy SDD Project\n\nAlready here.\n"
	if err := os.WriteFile(agentsPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("create AGENTS.md: %v", err)
	}

	store := config.NewFileStore()
	tool := NewInitTool(store, mustRenderer(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":        "idempotent-project",
		"description": "Tests idempotency",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)

	// Should NOT have duplicated the section.
	count := strings.Count(content, "## Hoofy SDD Project")
	if count != 1 {
		t.Errorf("Hoofy section should appear exactly once, found %d times", count)
	}
}

// --- CharterTool ---

func TestCharterTool_Handle_Success(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageCharter)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Freelancers waste 30+ minutes daily tracking hours across spreadsheets",
		"target_users":      "- **Freelance designers** who need simple time tracking\n- **Small agency owners** who need team visibility",
		"proposed_solution": "A web app where freelancers log hours per project and see weekly reports",
		"success_criteria":  "- Users can log time in under 10 seconds\n- 80% complete onboarding without help",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Charter Created") {
		t.Error("result should contain 'Charter Created'")
	}
	if !strings.Contains(text, "Freelancers waste") {
		t.Error("result should contain the problem statement content")
	}
	if !strings.Contains(text, "Freelance designers") {
		t.Error("result should contain the target users content")
	}
}

func TestCharterTool_Handle_MissingRequiredFields(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageCharter)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing problem_statement",
			args:   map[string]interface{}{"target_users": "devs", "proposed_solution": "app", "success_criteria": "works"},
			errMsg: "problem_statement",
		},
		{
			name:   "missing target_users",
			args:   map[string]interface{}{"problem_statement": "problem", "proposed_solution": "app", "success_criteria": "works"},
			errMsg: "target_users",
		},
		{
			name:   "missing proposed_solution",
			args:   map[string]interface{}{"problem_statement": "problem", "target_users": "devs", "success_criteria": "works"},
			errMsg: "proposed_solution",
		},
		{
			name:   "missing success_criteria",
			args:   map[string]interface{}{"problem_statement": "problem", "target_users": "devs", "proposed_solution": "app"},
			errMsg: "success_criteria",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error when required field is missing")
			}
			text := getResultText(result)
			if !strings.Contains(text, tt.errMsg) {
				t.Errorf("error should mention '%s': %s", tt.errMsg, text)
			}
		})
	}
}

func TestCharterTool_Handle_WrongStage(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "problem",
		"target_users":      "devs",
		"proposed_solution": "app",
		"success_criteria":  "works",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when at wrong stage")
	}
	text := getResultText(result)
	if !strings.Contains(text, "wrong pipeline stage") {
		t.Errorf("error should mention wrong stage: %s", text)
	}
}

func TestCharterTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageCharter)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Users need a chat app",
		"target_users":      "Remote teams",
		"proposed_solution": "Real-time messaging platform",
		"success_criteria":  "Sub-second message delivery",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Verify pipeline advanced.
	cfg, err := store.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.CurrentStage != config.StageSpecify {
		t.Errorf("stage should be specify after charter, got: %s", cfg.CurrentStage)
	}
}

// --- SpecifyTool ---

func TestSpecifyTool_Handle_Success(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	// Write a charter file (required by specify).
	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# Test Charter\n\nThis is a test charter with some requirements."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewSpecifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"must_have":      "- **FR-001**: Users can create an account\n- **FR-002**: Users can log time entries",
		"should_have":    "- **FR-005**: Users can export time entries as CSV",
		"non_functional": "- **NFR-001**: Page load time must be under 2 seconds",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Requirements Generated") {
		t.Error("result should contain 'Requirements Generated'")
	}
	if !strings.Contains(text, "FR-001") {
		t.Error("result should contain requirement IDs")
	}
	if !strings.Contains(text, "Users can create an account") {
		t.Error("result should contain the actual requirement content")
	}
}

func TestSpecifyTool_Handle_MissingRequiredFields(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# Test Charter\n\nSome content here."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewSpecifyTool(store, renderer)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing must_have",
			args:   map[string]interface{}{"should_have": "something", "non_functional": "something"},
			errMsg: "must_have",
		},
		{
			name:   "missing should_have",
			args:   map[string]interface{}{"must_have": "something", "non_functional": "something"},
			errMsg: "should_have",
		},
		{
			name:   "missing non_functional",
			args:   map[string]interface{}{"must_have": "something", "should_have": "something"},
			errMsg: "non_functional",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error when required field is missing")
			}
			text := getResultText(result)
			if !strings.Contains(text, tt.errMsg) {
				t.Errorf("error should mention '%s': %s", tt.errMsg, text)
			}
		})
	}
}

func TestSpecifyTool_Handle_EmptyProposal(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewSpecifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"must_have":      "- FR-001: Something",
		"should_have":    "- FR-002: Something",
		"non_functional": "- NFR-001: Something",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when charter is empty")
	}
}

func TestSpecifyTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# Test Charter\n\nSome content here."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewSpecifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"must_have":      "- **FR-001**: Users can sign up",
		"should_have":    "- **FR-003**: Users can export data",
		"non_functional": "- **NFR-001**: Load time < 2s",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	cfg, _ := store.Load(tmpDir)
	if cfg.CurrentStage != config.StageBusinessRules {
		t.Errorf("stage should be business-rules after specify, got: %s", cfg.CurrentStage)
	}
}

func TestSpecifyTool_Handle_OptionalFieldsDefault(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# Test Charter\n\nSome content here."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewSpecifyTool(store, renderer)

	// Only required fields — optional should get defaults.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"must_have":      "- **FR-001**: Users can sign up",
		"should_have":    "- **FR-003**: Users can export data",
		"non_functional": "- **NFR-001**: Load time < 2s",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "None defined for this version") {
		t.Error("optional empty fields should show default text")
	}
}

// --- ClarifyTool ---

func TestClarifyTool_Handle_GenerateQuestions(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	// Write requirements.
	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up"); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewClarifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Clarity Gate Analysis") {
		t.Error("result should contain 'Clarity Gate Analysis'")
	}
	if !strings.Contains(text, "target_users") {
		t.Error("result should contain dimension names")
	}
}

func TestClarifyTool_Handle_ProcessAnswers_GatePassed(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeExpert, config.StageClarify)
	defer cleanup()

	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up"); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewClarifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"answers":          "The target users are developers.\nCore functionality is task management.",
		"dimension_scores": "target_users:80,core_functionality:90,data_model:60,integrations:50,edge_cases:55,security:70,scale_performance:60,scope_boundaries:85",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Clarity Gate PASSED") {
		t.Errorf("expected gate to pass with high scores, got: %s", text[:min(200, len(text))])
	}

	// Verify pipeline advanced.
	cfg, _ := store.Load(tmpDir)
	if cfg.CurrentStage != config.StageDesign {
		t.Errorf("stage should be design after passing clarity gate, got: %s", cfg.CurrentStage)
	}
}

func TestClarifyTool_Handle_ProcessAnswers_GateNotPassed(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up"); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewClarifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"answers":          "Some vague answers",
		"dimension_scores": "target_users:30,core_functionality:40,data_model:20",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "More Clarification Needed") {
		t.Errorf("expected gate to not pass with low scores, got: %s", text[:min(200, len(text))])
	}

	// Pipeline should NOT have advanced.
	cfg, _ := store.Load(tmpDir)
	if cfg.CurrentStage != config.StageClarify {
		t.Errorf("stage should still be clarify, got: %s", cfg.CurrentStage)
	}
}

func TestClarifyTool_Handle_EmptyRequirements(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewClarifyTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when requirements are empty")
	}
}

// --- ContextTool ---

func TestContextTool_Handle_Overview(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	// Default is now "summary" mode — compact stage list without Pipeline Progress table.
	if !strings.Contains(text, "test-project") {
		t.Error("overview should contain project name")
	}
	if !strings.Contains(text, "Charter") || !strings.Contains(text, "Specify") {
		t.Error("overview should list pipeline stages")
	}
}

func TestContextTool_Handle_SpecificStage(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	// Write a charter file.
	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# My Charter\n\nThis is the charter content."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage": "charter",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "My Charter") {
		t.Error("should return charter content")
	}
}

func TestContextTool_Handle_EmptyStage(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage": "charter",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Not yet completed") {
		t.Error("should indicate stage is not yet completed")
	}
}

func TestContextTool_Handle_UnknownStage(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage": "nonexistent",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for unknown stage")
	}
}

func TestContextTool_Handle_SummaryDetailLevel(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"detail_level": "summary",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Summary should contain project name and mode.
	if !strings.Contains(text, "test-project") {
		t.Error("summary should contain project name")
	}
	if !strings.Contains(text, "guided") {
		t.Error("summary should contain mode")
	}

	// Summary should contain stage names with status indicators.
	if !strings.Contains(text, "Initialize") {
		t.Error("summary should list Initialize stage")
	}
	if !strings.Contains(text, "Charter") {
		t.Error("summary should list Charter stage")
	}
	if !strings.Contains(text, "✅") {
		t.Error("summary should have completed indicator for Init")
	}

	// Summary should mark current stage.
	if !strings.Contains(text, "←") {
		t.Error("summary should mark current stage with arrow")
	}

	// Summary should NOT contain verbose sections from standard mode.
	if strings.Contains(text, "Pipeline Progress") {
		t.Error("summary should NOT contain pipeline table header")
	}
	if strings.Contains(text, "Artifacts") {
		t.Error("summary should NOT contain artifacts section")
	}
	if strings.Contains(text, "Next Steps") {
		t.Error("summary should NOT contain next steps section")
	}
	if strings.Contains(text, "Description:") {
		t.Error("summary should NOT contain project description")
	}
}

func TestContextTool_Handle_SummaryAtClarifyShowsScore(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"detail_level": "summary",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Clarity:") {
		t.Error("summary at clarify stage should show clarity score")
	}
}

func TestContextTool_Handle_StandardDetailLevel(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	// Explicitly requesting "standard" should behave exactly like no detail_level.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"detail_level": "standard",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Pipeline Progress") {
		t.Error("standard should contain pipeline table")
	}
	if !strings.Contains(text, "Next Steps") {
		t.Error("standard should contain next steps")
	}
}

func TestContextTool_Handle_FullDetailLevel(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	// Write a charter artifact so there's content to include.
	charterPath := config.StagePath(tmpDir, config.StageCharter)
	charterContent := "# My Charter\n\nThis is a full charter.\n\n## Problem\n\nThe problem statement."
	if err := writeStageFile(charterPath, charterContent); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"detail_level": "full",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Full should include the standard overview sections.
	if !strings.Contains(text, "Pipeline Progress") {
		t.Error("full should contain pipeline table from standard overview")
	}
	if !strings.Contains(text, "Next Steps") {
		t.Error("full should contain next steps from standard overview")
	}

	// Full should include the inline artifact content.
	if !strings.Contains(text, "Charter Content") {
		t.Error("full should contain 'Charter Content' section header")
	}
	if !strings.Contains(text, "My Charter") {
		t.Error("full should include actual charter content")
	}
	if !strings.Contains(text, "The problem statement") {
		t.Error("full should include charter body text")
	}
}

func TestContextTool_Handle_FullDetailLevel_NoArtifacts(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"detail_level": "full",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)

	// Full with no artifacts should just be the standard overview.
	if !strings.Contains(text, "Pipeline Progress") {
		t.Error("full with no artifacts should contain standard overview")
	}
	// Should NOT have any "Content" section headers since no artifacts exist.
	if strings.Contains(text, "Charter Content") {
		t.Error("full with no artifacts should NOT have artifact content sections")
	}
}

func TestContextTool_Handle_DetailLevelIgnoredWithStage(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	// Write a charter artifact.
	charterPath := config.StagePath(tmpDir, config.StageCharter)
	if err := writeStageFile(charterPath, "# Specific Charter\n\nJust this one."); err != nil {
		t.Fatalf("write charter: %v", err)
	}

	store := config.NewFileStore()
	tool := NewContextTool(store)

	// Both stage and detail_level set — stage should take priority.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage":        "charter",
		"detail_level": "summary",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)

	// Should return the stage content, NOT a summary overview.
	if !strings.Contains(text, "Specific Charter") {
		t.Error("when stage is set, detail_level should be ignored — should return stage content")
	}
	if strings.Contains(text, "guided") {
		t.Error("should not return overview when stage is set")
	}
}

func TestContextTool_Handle_DefaultDetailLevel(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	// No detail_level — should default to summary (changed in F3).
	reqWithout := mcp.CallToolRequest{}
	reqWithout.Params.Arguments = map[string]interface{}{}

	resultWithout, err := tool.Handle(context.Background(), reqWithout)
	if err != nil {
		t.Fatalf("Handle without detail_level failed: %v", err)
	}

	textWithout := getResultText(resultWithout)
	// Summary mode shows stage names but NOT the full "Pipeline Progress" table.
	if !strings.Contains(textWithout, "Charter") {
		t.Error("no detail_level should default to summary (with stage names)")
	}
	if strings.Contains(textWithout, "Pipeline Progress") {
		t.Error("default summary mode should NOT contain Pipeline Progress table")
	}
}

// --- parseDimensionScores ---

func TestParseDimensionScores_ValidInput(t *testing.T) {
	dims := pipeline.DefaultDimensions()
	parseDimensionScores("target_users:80,core_functionality:90", dims)

	for _, d := range dims {
		switch d.Name {
		case "target_users":
			if d.Score != 80 {
				t.Errorf("target_users score = %d, want 80", d.Score)
			}
			if !d.Covered {
				t.Error("target_users should be covered (score > 30)")
			}
		case "core_functionality":
			if d.Score != 90 {
				t.Errorf("core_functionality score = %d, want 90", d.Score)
			}
		case "data_model":
			if d.Score != 0 {
				t.Errorf("data_model should be 0 (not provided), got %d", d.Score)
			}
		}
	}
}

func TestParseDimensionScores_ClampsValues(t *testing.T) {
	dims := pipeline.DefaultDimensions()
	parseDimensionScores("target_users:150,core_functionality:-10", dims)

	for _, d := range dims {
		switch d.Name {
		case "target_users":
			if d.Score != 100 {
				t.Errorf("target_users should clamp to 100, got %d", d.Score)
			}
		case "core_functionality":
			if d.Score != 0 {
				t.Errorf("core_functionality should clamp to 0, got %d", d.Score)
			}
		}
	}
}

func TestParseDimensionScores_InvalidFormat(t *testing.T) {
	dims := pipeline.DefaultDimensions()
	parseDimensionScores("garbage input without colons", dims)

	// All should remain at 0.
	for _, d := range dims {
		if d.Score != 0 {
			t.Errorf("dimension %s should be 0 with invalid input, got %d", d.Name, d.Score)
		}
	}
}

func TestParseDimensionScores_EmptyString(t *testing.T) {
	dims := pipeline.DefaultDimensions()
	parseDimensionScores("", dims)

	for _, d := range dims {
		if d.Score != 0 {
			t.Errorf("dimension %s should be 0 with empty input, got %d", d.Name, d.Score)
		}
	}
}

func TestParseDimensionScores_CoveredThreshold(t *testing.T) {
	dims := pipeline.DefaultDimensions()
	parseDimensionScores("target_users:30,core_functionality:31", dims)

	for _, d := range dims {
		switch d.Name {
		case "target_users":
			if d.Covered {
				t.Error("target_users at score 30 should NOT be covered (needs > 30)")
			}
		case "core_functionality":
			if !d.Covered {
				t.Error("core_functionality at score 31 should be covered (> 30)")
			}
		}
	}
}

// --- statusIndicator ---

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"completed", "✅"},
		{"in_progress", "🔄"},
		{"skipped", "⏭️"},
		{"pending", "⬜"},
		{"unknown", "⬜"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusIndicator(tt.status)
			if got != tt.want {
				t.Errorf("statusIndicator(%s) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}

// --- nextStepGuidance ---

func TestNextStepGuidance(t *testing.T) {
	tests := []struct {
		stage    config.Stage
		contains string
	}{
		{config.StagePrinciples, "sdd_create_principles"},
		{config.StageCharter, "sdd_create_charter"},
		{config.StageSpecify, "sdd_generate_requirements"},
		{config.StageClarify, "sdd_clarify"},
		{config.StageDesign, "sdd_create_design"},
		{config.StageTasks, "sdd_create_tasks"},
		{config.StageValidate, "sdd_validate"},
	}

	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			cfg := &config.ProjectConfig{
				CurrentStage: tt.stage,
				Mode:         config.ModeGuided,
			}
			got := nextStepGuidance(cfg)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("nextStepGuidance(%s) = %s, want to contain %s", tt.stage, got, tt.contains)
			}
		})
	}
}

// --- clarityThresholdForMode ---

func TestClarityThresholdForMode(t *testing.T) {
	if got := clarityThresholdForMode(config.ModeGuided); got != 70 {
		t.Errorf("clarityThresholdForMode(guided) = %d, want 70", got)
	}
	if got := clarityThresholdForMode(config.ModeExpert); got != 50 {
		t.Errorf("clarityThresholdForMode(expert) = %d, want 50", got)
	}
}

// --- Helper: min ---

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- DesignTool ---

func TestDesignTool_Handle_Success(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	// Write requirements (required by design).
	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up"); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"architecture_overview": "A modular monolith using Clean Architecture with 3 layers",
		"tech_stack":            "- **Runtime**: Node.js 20 LTS\n- **Database**: PostgreSQL 16",
		"components":            "### AuthModule\n- **Responsibility**: User registration, login\n- **Covers**: FR-001",
		"data_model":            "### User\n| Field | Type |\n|-------|------|\n| id | UUID |",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Technical Design Created") {
		t.Error("result should contain 'Technical Design Created'")
	}
	if !strings.Contains(text, "Clean Architecture") {
		t.Error("result should contain the architecture overview content")
	}
	if !strings.Contains(text, "AuthModule") {
		t.Error("result should contain component content")
	}
}

func TestDesignTool_Handle_MissingRequiredFields(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\nSome content."); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing architecture_overview",
			args:   map[string]interface{}{"tech_stack": "node", "components": "auth", "data_model": "users"},
			errMsg: "architecture_overview",
		},
		{
			name:   "missing tech_stack",
			args:   map[string]interface{}{"architecture_overview": "monolith", "components": "auth", "data_model": "users"},
			errMsg: "tech_stack",
		},
		{
			name:   "missing components",
			args:   map[string]interface{}{"architecture_overview": "monolith", "tech_stack": "node", "data_model": "users"},
			errMsg: "components",
		},
		{
			name:   "missing data_model",
			args:   map[string]interface{}{"architecture_overview": "monolith", "tech_stack": "node", "components": "auth"},
			errMsg: "data_model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error when required field is missing")
			}
			text := getResultText(result)
			if !strings.Contains(text, tt.errMsg) {
				t.Errorf("error should mention '%s': %s", tt.errMsg, text)
			}
		})
	}
}

func TestDesignTool_Handle_WrongStage(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"architecture_overview": "monolith",
		"tech_stack":            "node",
		"components":            "auth",
		"data_model":            "users",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when at wrong stage")
	}
	text := getResultText(result)
	if !strings.Contains(text, "wrong pipeline stage") {
		t.Errorf("error should mention wrong stage: %s", text)
	}
}

func TestDesignTool_Handle_EmptyRequirements(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"architecture_overview": "monolith",
		"tech_stack":            "node",
		"components":            "auth",
		"data_model":            "users",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when requirements are empty")
	}
}

func TestDesignTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\nSome content."); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"architecture_overview": "Microservices with API gateway",
		"tech_stack":            "Go + PostgreSQL",
		"components":            "AuthService, UserService",
		"data_model":            "User table with email, password_hash",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	cfg, _ := store.Load(tmpDir)
	if cfg.CurrentStage != config.StageTasks {
		t.Errorf("stage should be tasks after design, got: %s", cfg.CurrentStage)
	}
}

func TestDesignTool_Handle_OptionalFieldsDefault(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	reqPath := config.StagePath(tmpDir, config.StageSpecify)
	if err := writeStageFile(reqPath, "# Requirements\n\nSome content."); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewDesignTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"architecture_overview": "Monolith",
		"tech_stack":            "Python + Django",
		"components":            "AuthModule",
		"data_model":            "User model",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Not yet defined") {
		t.Error("optional empty fields should show default text")
	}
}

// --- TasksTool ---

func TestTasksTool_Handle_Success(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	// Write design document (required by tasks).
	designPath := config.StagePath(tmpDir, config.StageDesign)
	if err := writeStageFile(designPath, "# Design\n\n## Architecture\nMonolith with Clean Architecture"); err != nil {
		t.Fatalf("write design: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "5",
		"estimated_effort": "3-4 days for a single developer",
		"tasks":            "### TASK-001: Set up project scaffolding\n**Component**: ProjectSetup\n**Covers**: Infrastructure\n**Dependencies**: None",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Implementation Tasks Created") {
		t.Error("result should contain 'Implementation Tasks Created'")
	}
	if !strings.Contains(text, "TASK-001") {
		t.Error("result should contain task IDs")
	}
}

func TestTasksTool_Handle_MissingRequiredFields(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	designPath := config.StagePath(tmpDir, config.StageDesign)
	if err := writeStageFile(designPath, "# Design\n\nSome content."); err != nil {
		t.Fatalf("write design: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing total_tasks",
			args:   map[string]interface{}{"estimated_effort": "3 days", "tasks": "TASK-001"},
			errMsg: "total_tasks",
		},
		{
			name:   "missing estimated_effort",
			args:   map[string]interface{}{"total_tasks": "5", "tasks": "TASK-001"},
			errMsg: "estimated_effort",
		},
		{
			name:   "missing tasks",
			args:   map[string]interface{}{"total_tasks": "5", "estimated_effort": "3 days"},
			errMsg: "tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error when required field is missing")
			}
			text := getResultText(result)
			if !strings.Contains(text, tt.errMsg) {
				t.Errorf("error should mention '%s': %s", tt.errMsg, text)
			}
		})
	}
}

func TestTasksTool_Handle_WithWaveAssignments(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	designPath := config.StagePath(tmpDir, config.StageDesign)
	if err := writeStageFile(designPath, "# Design\n\n## Architecture\nMonolith"); err != nil {
		t.Fatalf("write design: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "3",
		"estimated_effort": "2 days",
		"tasks":            "### TASK-001: Scaffolding\n**Component**: Setup",
		"dependency_graph": "TASK-001 → TASK-002 → TASK-003",
		"wave_assignments": "**Wave 1**:\n- TASK-001\n\n**Wave 2**:\n- TASK-002\n- TASK-003",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Wave section must be present in the rendered output.
	waveChecks := []string{
		"Execution Waves",
		"Wave 1",
		"Wave 2",
		"TASK-001",
		"in parallel",
	}
	for _, check := range waveChecks {
		if !strings.Contains(text, check) {
			t.Errorf("result should contain %q when wave_assignments provided", check)
		}
	}
}

func TestTasksTool_Handle_WithoutWaveAssignments(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	designPath := config.StagePath(tmpDir, config.StageDesign)
	if err := writeStageFile(designPath, "# Design\n\n## Architecture\nMonolith"); err != nil {
		t.Fatalf("write design: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "3",
		"estimated_effort": "2 days",
		"tasks":            "### TASK-001: Scaffolding\n**Component**: Setup",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Wave section must NOT be present when wave_assignments is omitted.
	if strings.Contains(text, "Execution Waves") {
		t.Error("result should NOT contain 'Execution Waves' when wave_assignments is omitted")
	}

	// Other sections must still work (backwards compatibility).
	if !strings.Contains(text, "Implementation Tasks Created") {
		t.Error("result should still contain 'Implementation Tasks Created'")
	}
	if !strings.Contains(text, "TASK-001") {
		t.Error("result should still contain task content")
	}
}

func TestTasksTool_Handle_WrongStage(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageDesign)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "5",
		"estimated_effort": "3 days",
		"tasks":            "TASK-001",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when at wrong stage")
	}
}

func TestTasksTool_Handle_EmptyDesign(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "5",
		"estimated_effort": "3 days",
		"tasks":            "TASK-001",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when design is empty")
	}
}

func TestTasksTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	designPath := config.StagePath(tmpDir, config.StageDesign)
	if err := writeStageFile(designPath, "# Design\n\nSome content."); err != nil {
		t.Fatalf("write design: %v", err)
	}

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewTasksTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"total_tasks":      "3",
		"estimated_effort": "1 week",
		"tasks":            "### TASK-001: Setup\n### TASK-002: Auth\n### TASK-003: Deploy",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	cfg, _ := store.Load(tmpDir)
	if cfg.CurrentStage != config.StageValidate {
		t.Errorf("stage should be validate after tasks, got: %s", cfg.CurrentStage)
	}
}

// --- ValidateTool ---

// setupValidateProject creates a project at validate stage with all artifacts.
func setupValidateProject(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageValidate)

	// Write all required artifacts.
	artifacts := map[config.Stage]string{
		config.StagePrinciples: "# Principles\n\nTest principles.",
		config.StageCharter:    "# Charter\n\nA test charter.",
		config.StageSpecify:    "# Requirements\n\n- FR-001: Users can sign up",
		config.StageClarify:    "# Clarifications\n\nAll clarified.",
		config.StageDesign:     "# Design\n\nMonolith with Clean Architecture.",
		config.StageTasks:      "# Tasks\n\n### TASK-001: Setup project",
	}

	for stage, content := range artifacts {
		path := config.StagePath(tmpDir, stage)
		if err := writeStageFile(path, content); err != nil {
			cleanup()
			t.Fatalf("write %s: %v", stage, err)
		}
	}

	return tmpDir, cleanup
}

func TestValidateTool_Handle_Pass(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "**Covered (1/1)**:\n- FR-001 → TASK-001",
		"component_coverage":    "**Covered**:\n- AuthModule → TASK-001",
		"consistency_issues":    "_None found._",
		"verdict":               "PASS",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "PASS") {
		t.Error("result should contain 'PASS'")
	}
	if !strings.Contains(text, "SDD Pipeline Complete") {
		t.Error("result should contain 'SDD Pipeline Complete' for PASS verdict")
	}
}

func TestValidateTool_Handle_PassWithWarnings(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "**Covered (1/1)**:\n- FR-001 → TASK-001",
		"component_coverage":    "**Covered**:\n- AuthModule → TASK-001",
		"consistency_issues":    "1. Minor: No monitoring tasks defined",
		"verdict":               "PASS_WITH_WARNINGS",
		"recommendations":       "Add monitoring as tech debt for v1.1",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "PASS_WITH_WARNINGS") {
		t.Error("result should contain 'PASS_WITH_WARNINGS'")
	}
	if !strings.Contains(text, "with warnings") {
		t.Error("result should contain warning message")
	}
}

func TestValidateTool_Handle_Fail(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "**Uncovered (3/5)**:\n- FR-002, FR-003, FR-005 have no tasks",
		"component_coverage":    "**Uncovered**:\n- EmailModule has no tasks",
		"consistency_issues":    "1. Critical: Design says PostgreSQL but tasks mention MongoDB",
		"verdict":               "FAIL",
		"recommendations":       "1. Revisit design to fix database choice\n2. Add missing tasks",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "FAIL") {
		t.Error("result should contain 'FAIL'")
	}
	if !strings.Contains(text, "Validation Failed") {
		t.Error("result should contain 'Validation Failed' for FAIL verdict")
	}
}

func TestValidateTool_Handle_MissingRequiredFields(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing requirements_coverage",
			args:   map[string]interface{}{"component_coverage": "ok", "consistency_issues": "none", "verdict": "PASS"},
			errMsg: "requirements_coverage",
		},
		{
			name:   "missing component_coverage",
			args:   map[string]interface{}{"requirements_coverage": "ok", "consistency_issues": "none", "verdict": "PASS"},
			errMsg: "component_coverage",
		},
		{
			name:   "missing consistency_issues",
			args:   map[string]interface{}{"requirements_coverage": "ok", "component_coverage": "ok", "verdict": "PASS"},
			errMsg: "consistency_issues",
		},
		{
			name:   "missing verdict",
			args:   map[string]interface{}{"requirements_coverage": "ok", "component_coverage": "ok", "consistency_issues": "none"},
			errMsg: "verdict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error when required field is missing")
			}
			text := getResultText(result)
			if !strings.Contains(text, tt.errMsg) {
				t.Errorf("error should mention '%s': %s", tt.errMsg, text)
			}
		})
	}
}

func TestValidateTool_Handle_InvalidVerdict(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "all covered",
		"component_coverage":    "all covered",
		"consistency_issues":    "none",
		"verdict":               "MAYBE",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for invalid verdict")
	}
	text := getResultText(result)
	if !strings.Contains(text, "PASS") {
		t.Errorf("error should mention valid options: %s", text)
	}
}

func TestValidateTool_Handle_WrongStage(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageTasks)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "ok",
		"component_coverage":    "ok",
		"consistency_issues":    "none",
		"verdict":               "PASS",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when at wrong stage")
	}
}

func TestValidateTool_Handle_MissingArtifacts(t *testing.T) {
	// Set up at validate stage but DON'T write all artifacts.
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageValidate)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "ok",
		"component_coverage":    "ok",
		"consistency_issues":    "none",
		"verdict":               "PASS",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when artifacts are missing")
	}
}

func TestValidateTool_Handle_CompletesStage(t *testing.T) {
	tmpDir, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "All covered",
		"component_coverage":    "All covered",
		"consistency_issues":    "_None found._",
		"verdict":               "PASS",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Verify the stage is marked completed (not advanced — it's the last stage).
	cfg, _ := store.Load(tmpDir)
	status := cfg.StageStatus[config.StageValidate]
	if status.Status != "completed" {
		t.Errorf("validate stage should be completed, got: %s", status.Status)
	}
}

func TestValidateTool_Handle_VerdictCaseInsensitive(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "All covered",
		"component_coverage":    "All covered",
		"consistency_issues":    "_None found._",
		"verdict":               "pass",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("should accept lowercase verdict, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "PASS") {
		t.Error("result should normalize verdict to uppercase")
	}
}

// --- Bridge tests ---

// spyObserver records calls for testing.
type spyObserver struct {
	calls []bridgeCall
}

type bridgeCall struct {
	projectName string
	stage       config.Stage
	content     string
}

func (s *spyObserver) OnStageComplete(projectName string, stage config.Stage, content string) {
	s.calls = append(s.calls, bridgeCall{projectName, stage, content})
}

func TestNotifyObserver_NilSafe(t *testing.T) {
	// Must not panic with nil observer.
	notifyObserver(nil, "test", config.StageCharter, "content")
}

func TestNotifyObserver_CallsObserver(t *testing.T) {
	spy := &spyObserver{}
	notifyObserver(spy, "my-project", config.StageDesign, "design content")

	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(spy.calls))
	}
	c := spy.calls[0]
	if c.projectName != "my-project" {
		t.Errorf("projectName = %q, want %q", c.projectName, "my-project")
	}
	if c.stage != config.StageDesign {
		t.Errorf("stage = %q, want %q", c.stage, config.StageDesign)
	}
	if c.content != "design content" {
		t.Errorf("content = %q, want %q", c.content, "design content")
	}
}

func TestNormalizeProject(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Project", "my-project"},
		{"  Hello World  ", "hello-world"},
		{"test_project", "test_project"},
		{"abc123", "abc123"},
		{"special!@#chars$%^", "specialchars"},
		{"", ""},
	}
	for _, tc := range tests {
		got := normalizeProject(tc.input)
		if got != tc.want {
			t.Errorf("normalizeProject(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCompactSummary_ShortContent(t *testing.T) {
	content := "Short content"
	result := compactSummary(config.StageCharter, content)
	if !strings.Contains(result, "Short content") {
		t.Errorf("short content should be preserved fully, got: %s", result)
	}
	if !strings.Contains(result, "charter") {
		t.Errorf("should include stage name, got: %s", result)
	}
}

func TestCompactSummary_LongContent(t *testing.T) {
	content := strings.Repeat("x", 1000)
	result := compactSummary(config.StageDesign, content)
	if len(result) > 550 {
		t.Errorf("compact summary too long: %d chars", len(result))
	}
	if !strings.Contains(result, "[...truncated]") {
		t.Errorf("should indicate truncation, got: %s", result)
	}
}

func TestNewMemoryBridge_NilStore(t *testing.T) {
	bridge := NewMemoryBridge(nil)
	if bridge != nil {
		t.Error("NewMemoryBridge(nil) should return nil")
	}
}

func TestMemoryBridge_OnStageComplete(t *testing.T) {
	memCfg := memory.Config{
		DataDir:              t.TempDir(),
		MaxObservationLength: 2000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         15 * time.Minute,
	}
	ms, err := memory.New(memCfg)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer func() { _ = ms.Close() }()

	bridge := NewMemoryBridge(ms)
	bridge.OnStageComplete("Test Project", config.StageCharter, "# Charter\n\nWe're building a thing.")

	// Verify observation was saved by searching.
	results, err := ms.Search("SDD charter", memory.SearchOptions{
		Project: "Test Project",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected bridge to save an observation, got 0 results")
	}
	if !strings.Contains(results[0].Title, "SDD charter") {
		t.Errorf("title = %q, want it to contain 'SDD charter'", results[0].Title)
	}
	if !strings.Contains(results[0].Content, "Charter") {
		t.Errorf("content should contain artifact text, got: %s", results[0].Content)
	}
	wantTopicKey := "sdd/test-project/charter"
	if results[0].TopicKey == nil || *results[0].TopicKey != wantTopicKey {
		got := "<nil>"
		if results[0].TopicKey != nil {
			got = *results[0].TopicKey
		}
		t.Errorf("topic_key = %q, want %q", got, wantTopicKey)
	}
}

func TestMemoryBridge_TopicKeyUpsert(t *testing.T) {
	memCfg := memory.Config{
		DataDir:              t.TempDir(),
		MaxObservationLength: 2000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         0, // Disable dedup window to allow rapid upserts.
	}
	ms, err := memory.New(memCfg)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer func() { _ = ms.Close() }()

	bridge := NewMemoryBridge(ms)

	// Call twice for same stage — should upsert, not duplicate.
	bridge.OnStageComplete("MyApp", config.StageDesign, "Design v1")
	bridge.OnStageComplete("MyApp", config.StageDesign, "Design v2")

	results, err := ms.Search("SDD design", memory.SearchOptions{
		Project: "MyApp",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 observation (upsert), got %d", len(results))
	}
	if !strings.Contains(results[0].Content, "Design v2") {
		t.Errorf("should have latest content, got: %s", results[0].Content)
	}
}

func TestCharterTool_SetBridge(t *testing.T) {
	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)
	spy := &spyObserver{}

	// SetBridge should not panic and should be callable.
	tool.SetBridge(spy)
	tool.SetBridge(nil) // nil should also be safe
}

func TestCharterTool_Handle_NotifiesBridge(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageCharter)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)
	spy := &spyObserver{}
	tool.SetBridge(spy)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Users need help",
		"target_users":      "- Developers",
		"proposed_solution": "Build a tool",
		"success_criteria":  "- It works",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 bridge call, got %d", len(spy.calls))
	}
	if spy.calls[0].stage != config.StageCharter {
		t.Errorf("stage = %q, want charter", spy.calls[0].stage)
	}
	if spy.calls[0].projectName != "test-project" {
		t.Errorf("projectName = %q, want test-project", spy.calls[0].projectName)
	}
}

func TestValidateTool_Handle_NotifiesBridge(t *testing.T) {
	_, cleanup := setupValidateProject(t)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewValidateTool(store)
	spy := &spyObserver{}
	tool.SetBridge(spy)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"requirements_coverage": "All covered",
		"component_coverage":    "All covered",
		"consistency_issues":    "_None found._",
		"verdict":               "PASS",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error: %s", getResultText(result))
	}

	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 bridge call, got %d", len(spy.calls))
	}
	if spy.calls[0].stage != config.StageValidate {
		t.Errorf("stage = %q, want validate", spy.calls[0].stage)
	}
}

func TestCharterTool_Handle_NilBridge_NoError(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageCharter)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewCharterTool(store, renderer)
	// Do NOT set bridge — it should be nil and still work.

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Users need help",
		"target_users":      "- Developers",
		"proposed_solution": "Build a tool",
		"success_criteria":  "- It works",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed with nil bridge: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("unexpected error with nil bridge: %s", getResultText(result))
	}
}
