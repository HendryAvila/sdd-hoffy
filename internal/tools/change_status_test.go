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

// --- ChangeStatusTool tests (TASK-007) ---

func TestChangeStatusTool_Definition(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)
	def := tool.Definition()

	if def.Name != "sdd_change_status" {
		t.Errorf("name = %q, want sdd_change_status", def.Name)
	}
}

func TestChangeStatusTool_Handle_ActiveChange(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "status active test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

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
	if !strings.Contains(text, "Change Status") {
		t.Error("result should contain 'Change Status' header")
	}
	if !strings.Contains(text, "status-active-test") {
		t.Error("result should contain the change ID")
	}
	if !strings.Contains(text, "fix") {
		t.Error("result should show change type")
	}
	if !strings.Contains(text, "small") {
		t.Error("result should show change size")
	}
	if !strings.Contains(text, "active") {
		t.Error("result should show status 'active'")
	}
}

func TestChangeStatusTool_Handle_StageProgressTable(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "progress table")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	// fix/small: describe → tasks → verify
	if !strings.Contains(text, "Stage Progress") {
		t.Error("result should contain 'Stage Progress' section")
	}
	if !strings.Contains(text, "describe") {
		t.Error("result should show 'describe' stage")
	}
	if !strings.Contains(text, "tasks") {
		t.Error("result should show 'tasks' stage")
	}
	if !strings.Contains(text, "verify") {
		t.Error("result should show 'verify' stage")
	}
	// First stage is in_progress → 🔄
	if !strings.Contains(text, "🔄") {
		t.Error("result should show 🔄 marker for in_progress stage")
	}
}

func TestChangeStatusTool_Handle_ByChangeID(t *testing.T) {
	_, cleanup, change := createActiveChange(t, changes.TypeFeature, changes.SizeMedium, "lookup by id")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"change_id": change.ID,
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, change.ID) {
		t.Errorf("result should contain change ID %q", change.ID)
	}
}

func TestChangeStatusTool_Handle_NoActiveChange(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

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

func TestChangeStatusTool_Handle_InvalidChangeID(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"change_id": "nonexistent-change",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for nonexistent change ID")
	}
	text := getResultText(result)
	if !strings.Contains(text, "nonexistent-change") {
		t.Errorf("error should mention the change ID: %s", text)
	}
}

func TestChangeStatusTool_Handle_WithArtifactSizes(t *testing.T) {
	tmpDir, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "artifact sizes")
	defer cleanup()

	// Write a stage artifact so the status tool can show its size.
	changeDir := filepath.Join(tmpDir, "docs", "changes", "artifact-sizes")
	descPath := filepath.Join(changeDir, "describe.md")
	content := "# Description\n\nThis is a test description with enough content to verify byte count."
	if err := os.WriteFile(descPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write describe.md: %v", err)
	}

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	// Should show file size for the existing artifact.
	if !strings.Contains(text, "describe.md") {
		t.Error("result should mention the artifact filename")
	}
	if !strings.Contains(text, "bytes") {
		t.Error("result should show byte count for existing artifact")
	}
}

func TestChangeStatusTool_Handle_WithADRs(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "adr status test")
	defer cleanup()

	// Add ADRs to the change record.
	store := changes.NewFileStore()
	cwd, _ := os.Getwd()
	change, err := store.LoadActive(cwd)
	if err != nil {
		t.Fatalf("LoadActive failed: %v", err)
	}
	change.ADRs = []string{"ADR-001", "ADR-002"}
	if err := store.Save(cwd, change); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "ADRs") {
		t.Error("result should contain 'ADRs' section")
	}
	if !strings.Contains(text, "ADR-001") {
		t.Error("result should list ADR-001")
	}
	if !strings.Contains(text, "ADR-002") {
		t.Error("result should list ADR-002")
	}
}

func TestChangeStatusTool_Handle_NoADRsSection(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "no adrs")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeStatusTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	// With no ADRs, the ADRs section should NOT appear.
	if strings.Contains(text, "## ADRs") {
		t.Error("result should NOT contain ADRs section when there are no ADRs")
	}
}

func TestChangeStatusTool_Handle_CompletedChange(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "completed status")
	defer cleanup()

	// Complete the change through the advance tool.
	store := changes.NewFileStore()
	advanceTool := NewChangeAdvanceTool(store)

	stages := []string{
		"# Describe\n\nContent.",
		"# Tasks\n\nContent.",
		"# Verify\n\nContent.",
	}
	for _, c := range stages {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"content": c}
		if _, err := advanceTool.Handle(context.Background(), req); err != nil {
			t.Fatalf("advance failed: %v", err)
		}
	}

	// Now check status by ID (LoadActive won't find completed changes).
	statusTool := NewChangeStatusTool(store)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"change_id": "completed-status",
	}

	result, err := statusTool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "completed") {
		t.Error("result should show 'completed' status")
	}
	// All stages should show ✅.
	if !strings.Contains(text, "✅") {
		t.Error("result should show ✅ markers for all completed stages")
	}
}
