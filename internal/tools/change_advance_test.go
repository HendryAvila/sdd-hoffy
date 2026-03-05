package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HendryAvila/Hoofy/internal/changes"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- ChangeAdvanceTool tests (TASK-006) ---

func TestChangeAdvanceTool_Definition(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)
	def := tool.Definition()

	if def.Name != "sdd_change_advance" {
		t.Errorf("name = %q, want sdd_change_advance", def.Name)
	}
}

func TestChangeAdvanceTool_Handle_AdvancesStage(t *testing.T) {
	tmpDir, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "fix empty query")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// fix/small flow: describe → context-check → tasks → verify
	// Current stage: describe (first, in_progress)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Description\n\nFix the FTS5 empty query crash.",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Stage Completed: describe") {
		t.Error("result should mention completed stage 'describe'")
	}
	if !strings.Contains(text, "context-check") {
		t.Error("result should show next stage 'context-check'")
	}

	// Verify the file was written.
	descPath := filepath.Join(tmpDir, "docs", "changes", "fix-empty-query", "describe.md")
	data, err := os.ReadFile(descPath)
	if err != nil {
		t.Fatalf("describe.md should exist: %v", err)
	}
	if !strings.Contains(string(data), "FTS5 empty query crash") {
		t.Error("describe.md should contain the provided content")
	}

	// Verify state advanced.
	change, err := store.LoadActive(tmpDir)
	if err != nil {
		t.Fatalf("LoadActive failed: %v", err)
	}
	if change.CurrentStage != changes.StageContextCheck {
		t.Errorf("current stage = %q, want context-check", change.CurrentStage)
	}
}

func TestChangeAdvanceTool_Handle_CompletesChange(t *testing.T) {
	_, cleanup, change := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "complete me")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// fix/small flow: describe → context-check → tasks → verify
	// Advance through all stages.
	stages := []string{
		"# Description\n\nFix something.",
		"# Context Check\n\nNo conflicts found.",
		"# Tasks\n\n- [ ] Task 1",
		"# Verification\n\nAll good.",
	}

	for _, content := range stages {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"content": content,
		}
		result, err := tool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("Handle failed: %v", err)
		}
		if isErrorResult(result) {
			t.Fatalf("expected success, got error: %s", getResultText(result))
		}
	}

	// Last call should show "Change completed!"
	// (We already consumed it, so reload from store)
	// LoadActive returns nil for completed changes, so use Load directly.
	tmpDir, _ := os.Getwd()
	loaded, err := store.Load(tmpDir, change.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", loaded.Status)
	}

	// All stages should be completed.
	for _, s := range loaded.Stages {
		if s.Status != "completed" {
			t.Errorf("stage %q status = %q, want completed", s.Name, s.Status)
		}
	}
}

func TestChangeAdvanceTool_Handle_CompletionResponse(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "completion response test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// fix/small: describe → context-check → tasks → verify
	contents := []string{
		"# Describe\n\nContent.",
		"# Context Check\n\nContent.",
		"# Tasks\n\nContent.",
		"# Verify\n\nContent.",
	}

	var lastResult *mcp.CallToolResult
	for _, c := range contents {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"content": c,
		}
		result, err := tool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("Handle failed: %v", err)
		}
		if isErrorResult(result) {
			t.Fatalf("expected success, got error: %s", getResultText(result))
		}
		lastResult = result
	}

	// The last result should contain the completion message.
	text := getResultText(lastResult)
	if !strings.Contains(text, "Change completed!") {
		t.Error("final result should contain 'Change completed!'")
	}
	if !strings.Contains(text, "completed") {
		t.Error("final result should mention 'completed' status")
	}
}

func TestChangeAdvanceTool_Handle_EmptyContent(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "empty content test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "   ",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for empty/whitespace content")
	}
	text := getResultText(result)
	if !strings.Contains(text, "content") {
		t.Errorf("error should mention 'content': %s", text)
	}
}

func TestChangeAdvanceTool_Handle_NoContent(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "no content test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when content is missing")
	}
}

func TestChangeAdvanceTool_Handle_NoActiveChange(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Some content",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when no active change exists")
	}
	text := getResultText(result)
	if !strings.Contains(text, "No active change") {
		t.Errorf("error should mention 'No active change': %s", text)
	}
}

func TestChangeAdvanceTool_Handle_WritesCorrectFilename(t *testing.T) {
	tmpDir, cleanup, _ := createActiveChange(t, changes.TypeRefactor, changes.SizeSmall, "rename vars")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// refactor/small: scope → context-check → tasks → verify
	// First stage: scope → should write scope.md
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Scope\n\nRename variables for consistency.",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	scopePath := filepath.Join(tmpDir, "docs", "changes", "rename-vars", "scope.md")
	if _, err := os.Stat(scopePath); os.IsNotExist(err) {
		t.Error("scope.md should be created for refactor/small first stage")
	}
}

func TestChangeAdvanceTool_Handle_WithTitle(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "title test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Content\n\nSome content.",
		"title":   "My Stage Title",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "My Stage Title") {
		t.Error("result should include the title when provided")
	}
}

func TestChangeAdvanceTool_Handle_ProgressMarkers(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFeature, changes.SizeMedium, "progress markers")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// feature/medium: charter → context-check → spec → tasks → verify
	// Advance first stage.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Charter\n\nNew feature.",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	// After completing charter, should show ✅ for charter and 🔄 for spec.
	if !strings.Contains(text, "✅") {
		t.Error("should show ✅ marker for completed stage")
	}
	if !strings.Contains(text, "🔄") {
		t.Error("should show 🔄 marker for in_progress stage")
	}
}

func TestChangeAdvanceTool_Handle_MediumFlowFullCycle(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeMedium, "medium flow cycle")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// fix/medium: describe → context-check → spec → tasks → verify
	contents := []string{
		"# Describe\n\nDescription.",
		"# Context Check\n\nNo conflicts.",
		"# Spec\n\nSpecification.",
		"# Tasks\n\n- [ ] Task 1",
		"# Verify\n\nVerification.",
	}

	for i, c := range contents {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"content": c,
		}
		result, err := tool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("stage %d: Handle failed: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("stage %d: expected success, got error: %s", i, getResultText(result))
		}
	}

	// Should be completed now.
	cwd, _ := os.Getwd()
	change, _ := store.Load(cwd, "medium-flow-cycle")
	if change.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", change.Status)
	}
}

func TestChangeAdvanceTool_Handle_LargeFlowFullCycle(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "large flow")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// feature/large: charter → context-check → spec → clarify → design → tasks → verify
	contents := []string{
		"# Charter\n\nCharter content.",
		"# Context Check\n\nNo conflicts.",
		"# Spec\n\nSpecification.",
		"# Clarify\n\nClarifications.",
		"# Design\n\nArchitecture.",
		"# Tasks\n\n- [ ] Task 1",
		"# Verify\n\nVerification.",
	}

	for i, c := range contents {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"content": c,
		}
		result, err := tool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("stage %d: Handle failed: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("stage %d: expected success, got error: %s", i, getResultText(result))
		}
	}

	cwd, _ := os.Getwd()
	change, _ := store.Load(cwd, "large-flow")
	if change.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", change.Status)
	}
	if len(change.Stages) != 7 {
		t.Errorf("stages = %d, want 7", len(change.Stages))
	}
}

func TestChangeAdvanceTool_SetBridge(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	// Should not panic with nil.
	tool.SetBridge(nil)

	// Verify bridge is set.
	if tool.bridge != nil {
		t.Error("bridge should be nil after SetBridge(nil)")
	}
}

func TestChangeAdvanceTool_Handle_BridgeNotification(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "bridge notify test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeAdvanceTool(store)

	var notified bool
	var notifiedStage changes.ChangeStage
	var notifiedContent string
	mock := &mockChangeObserver{
		fn: func(changeID string, stage changes.ChangeStage, content string) {
			notified = true
			notifiedStage = stage
			notifiedContent = content
		},
	}
	tool.SetBridge(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"content": "# Description\n\nBridge test content.",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if !notified {
		t.Error("bridge should have been notified")
	}
	if notifiedStage != changes.StageDescribe {
		t.Errorf("notified stage = %q, want describe", notifiedStage)
	}
	if !strings.Contains(notifiedContent, "Bridge test content") {
		t.Error("notified content should contain the stage content")
	}
}

// mockChangeObserver implements ChangeObserver for testing.
type mockChangeObserver struct {
	fn func(changeID string, stage changes.ChangeStage, content string)
}

func (m *mockChangeObserver) OnChangeStageComplete(changeID string, stage changes.ChangeStage, content string) {
	if m.fn != nil {
		m.fn(changeID, stage, content)
	}
}
