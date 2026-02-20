package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/HendryAvila/sdd-hoffy/internal/pipeline"
	"github.com/HendryAvila/sdd-hoffy/internal/templates"
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
		os.Chdir(origDir)
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

// makeCallToolRequest creates a minimal MCP CallToolRequest with string args.
func makeCallToolRequest(args map[string]string) mcp.CallToolRequest {
	params := make(map[string]interface{})
	for k, v := range args {
		params[k] = v
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = params
	return req
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

// --- InitTool ---

func TestInitTool_Handle_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	store := config.NewFileStore()
	tool := NewInitTool(store)

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
	historyDir := filepath.Join(tmpDir, "sdd", "history")
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Error("sdd/history/ directory should exist after init")
	}
}

func TestInitTool_Handle_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	store := config.NewFileStore()
	tool := NewInitTool(store)

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
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	store := config.NewFileStore()
	tool := NewInitTool(store)

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
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	store := config.NewFileStore()
	tool := NewInitTool(store)

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
	tool := NewInitTool(store)

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
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	store := config.NewFileStore()
	tool := NewInitTool(store)

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

// --- ProposeTool ---

func TestProposeTool_Handle_Success(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StagePropose)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewProposeTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Freelancers waste 30+ minutes daily tracking hours across spreadsheets",
		"target_users":      "- **Freelance designers** who need simple time tracking\n- **Small agency owners** who need team visibility",
		"proposed_solution":  "A web app where freelancers log hours per project and see weekly reports",
		"out_of_scope":       "- Will NOT handle invoicing\n- Will NOT support offline mode",
		"success_criteria":   "- Users can log time in under 10 seconds\n- 80% complete onboarding without help",
		"open_questions":     "- Should we support mobile from day one?",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Proposal Created") {
		t.Error("result should contain 'Proposal Created'")
	}
	if !strings.Contains(text, "Freelancers waste") {
		t.Error("result should contain the problem statement content")
	}
	if !strings.Contains(text, "Freelance designers") {
		t.Error("result should contain the target users content")
	}
}

func TestProposeTool_Handle_MissingRequiredFields(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StagePropose)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewProposeTool(store, renderer)

	tests := []struct {
		name   string
		args   map[string]interface{}
		errMsg string
	}{
		{
			name:   "missing problem_statement",
			args:   map[string]interface{}{"target_users": "devs", "proposed_solution": "app", "out_of_scope": "none", "success_criteria": "works"},
			errMsg: "problem_statement",
		},
		{
			name:   "missing target_users",
			args:   map[string]interface{}{"problem_statement": "problem", "proposed_solution": "app", "out_of_scope": "none", "success_criteria": "works"},
			errMsg: "target_users",
		},
		{
			name:   "missing proposed_solution",
			args:   map[string]interface{}{"problem_statement": "problem", "target_users": "devs", "out_of_scope": "none", "success_criteria": "works"},
			errMsg: "proposed_solution",
		},
		{
			name:   "missing out_of_scope",
			args:   map[string]interface{}{"problem_statement": "problem", "target_users": "devs", "proposed_solution": "app", "success_criteria": "works"},
			errMsg: "out_of_scope",
		},
		{
			name:   "missing success_criteria",
			args:   map[string]interface{}{"problem_statement": "problem", "target_users": "devs", "proposed_solution": "app", "out_of_scope": "none"},
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

func TestProposeTool_Handle_WrongStage(t *testing.T) {
	_, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageClarify)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewProposeTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "problem",
		"target_users":      "devs",
		"proposed_solution":  "app",
		"out_of_scope":       "none",
		"success_criteria":   "works",
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

func TestProposeTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StagePropose)
	defer cleanup()

	store := config.NewFileStore()
	renderer, _ := templates.NewRenderer()
	tool := NewProposeTool(store, renderer)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"problem_statement": "Users need a chat app",
		"target_users":      "Remote teams",
		"proposed_solution":  "Real-time messaging platform",
		"out_of_scope":       "Video calls",
		"success_criteria":   "Sub-second message delivery",
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
		t.Errorf("stage should be specify after propose, got: %s", cfg.CurrentStage)
	}
}

// --- SpecifyTool ---

func TestSpecifyTool_Handle_Success(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	// Write a proposal file (required by specify).
	proposalPath := config.StagePath(tmpDir, config.StagePropose)
	writeStageFile(proposalPath, "# Test Proposal\n\nThis is a test proposal with some requirements.")

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

	proposalPath := config.StagePath(tmpDir, config.StagePropose)
	writeStageFile(proposalPath, "# Test Proposal\n\nSome content here.")

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
		t.Error("should return error when proposal is empty")
	}
}

func TestSpecifyTool_Handle_AdvancesPipeline(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	proposalPath := config.StagePath(tmpDir, config.StagePropose)
	writeStageFile(proposalPath, "# Test Proposal\n\nSome content here.")

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
	if cfg.CurrentStage != config.StageClarify {
		t.Errorf("stage should be clarify after specify, got: %s", cfg.CurrentStage)
	}
}

func TestSpecifyTool_Handle_OptionalFieldsDefault(t *testing.T) {
	tmpDir, cleanup := setupTestProjectAtStage(t, config.ModeGuided, config.StageSpecify)
	defer cleanup()

	proposalPath := config.StagePath(tmpDir, config.StagePropose)
	writeStageFile(proposalPath, "# Test Proposal\n\nSome content here.")

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
	writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up")

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
	writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up")

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
	writeStageFile(reqPath, "# Requirements\n\n- FR-001: Users can sign up")

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
	if !strings.Contains(text, "test-project") {
		t.Error("overview should contain project name")
	}
	if !strings.Contains(text, "Pipeline Progress") {
		t.Error("overview should contain pipeline table")
	}
	if !strings.Contains(text, "Propose") || !strings.Contains(text, "Specify") {
		t.Error("overview should list pipeline stages")
	}
}

func TestContextTool_Handle_SpecificStage(t *testing.T) {
	tmpDir, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	// Write a proposal file.
	proposalPath := config.StagePath(tmpDir, config.StagePropose)
	writeStageFile(proposalPath, "# My Proposal\n\nThis is the proposal content.")

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage": "propose",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "My Proposal") {
		t.Error("should return proposal content")
	}
}

func TestContextTool_Handle_EmptyStage(t *testing.T) {
	_, cleanup := setupTestProject(t, config.ModeGuided)
	defer cleanup()

	store := config.NewFileStore()
	tool := NewContextTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"stage": "propose",
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
		{config.StagePropose, "sdd_create_proposal"},
		{config.StageSpecify, "sdd_generate_requirements"},
		{config.StageClarify, "sdd_clarify"},
		{config.StageDesign, "coming in v2"},
		{config.StageTasks, "coming in v2"},
		{config.StageValidate, "coming in v2"},
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
