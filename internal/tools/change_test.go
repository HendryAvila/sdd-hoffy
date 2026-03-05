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

// --- Test helpers for change tools ---

// setupChangeProject creates a temp dir with a docs/ directory and
// changes cwd to it. Returns the temp dir and a cleanup function.
// Does NOT require hoofy.json — change pipeline is independent.
func setupChangeProject(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create minimal docs/ directory.
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs"), 0o755); err != nil {
		t.Fatalf("setup: mkdir docs: %v", err)
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

// setupChangeProjectWithArtifacts creates a change project with a dummy
// SDD artifact so that medium/large changes pass the artifact guard.
func setupChangeProjectWithArtifacts(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, cleanup := setupChangeProject(t)
	addDummyArtifact(t, tmpDir)
	return tmpDir, cleanup
}

// addDummyArtifact creates a minimal requirements.md in the docs/ directory
// so that CheckSDDArtifacts returns true (artifact guard passes).
func addDummyArtifact(t *testing.T, projectRoot string) {
	t.Helper()
	path := filepath.Join(projectRoot, "docs", "requirements.md")
	if err := os.WriteFile(path, []byte("# Requirements\n\nDummy artifact for testing.\n"), 0o644); err != nil {
		t.Fatalf("setup: write dummy artifact: %v", err)
	}
}

// createActiveChange sets up a change project with an active change.
// Adds a dummy artifact so the artifact guard passes for any size.
func createActiveChange(t *testing.T, ct changes.ChangeType, cs changes.ChangeSize, desc string) (string, func(), *changes.ChangeRecord) {
	t.Helper()
	tmpDir, cleanup := setupChangeProject(t)
	addDummyArtifact(t, tmpDir)

	store := changes.NewFileStore()
	flow, err := changes.StageFlow(ct, cs)
	if err != nil {
		cleanup()
		t.Fatalf("setup: stage flow: %v", err)
	}

	stageEntries := make([]changes.StageEntry, len(flow))
	for i, stage := range flow {
		status := "pending"
		startedAt := ""
		if i == 0 {
			status = "in_progress"
			startedAt = "2025-01-01T00:00:00Z"
		}
		stageEntries[i] = changes.StageEntry{
			Name:      stage,
			Status:    status,
			StartedAt: startedAt,
		}
	}

	change := &changes.ChangeRecord{
		ID:           changes.Slugify(desc),
		Type:         ct,
		Size:         cs,
		Description:  desc,
		Stages:       stageEntries,
		CurrentStage: flow[0],
		ADRs:         []string{},
		Status:       changes.StatusActive,
		CreatedAt:    "2025-01-01T00:00:00Z",
		UpdatedAt:    "2025-01-01T00:00:00Z",
	}

	if err := store.Create(tmpDir, change); err != nil {
		cleanup()
		t.Fatalf("setup: create change: %v", err)
	}

	return tmpDir, cleanup, change
}

// --- ChangeTool tests (TASK-005) ---

func TestChangeTool_Handle_Success(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Fix FTS5 empty query crash",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Change Created") {
		t.Error("result should contain 'Change Created'")
	}
	if !strings.Contains(text, "fix-fts5-empty-query-crash") {
		t.Error("result should contain the generated slug ID")
	}
	if !strings.Contains(text, "describe") {
		t.Error("result should show the first stage (describe for fix/small)")
	}
	if !strings.Contains(text, "4 stages") {
		t.Error("result should show stage count (4 for fix/small)")
	}
}

func TestChangeTool_Handle_CreatesFiles(t *testing.T) {
	tmpDir, cleanup := setupChangeProjectWithArtifacts(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "large",
		"description": "Add user auth",
	}

	_, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Verify directory and change.json created.
	changeDir := filepath.Join(tmpDir, "docs", "changes", "add-user-auth")
	if _, err := os.Stat(changeDir); os.IsNotExist(err) {
		t.Error("change directory should be created")
	}

	configPath := filepath.Join(changeDir, "change.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("change.json should be created")
	}

	// Verify the record loads correctly.
	change, err := store.Load(tmpDir, "add-user-auth")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if change.Type != changes.TypeFeature {
		t.Errorf("type = %q, want feature", change.Type)
	}
	if change.Size != changes.SizeLarge {
		t.Errorf("size = %q, want large", change.Size)
	}
	if change.Status != changes.StatusActive {
		t.Errorf("status = %q, want active", change.Status)
	}
	if len(change.Stages) != 7 {
		t.Errorf("stages count = %d, want 7 for feature/large", len(change.Stages))
	}
}

func TestChangeTool_Handle_InvalidType(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "bugfix",
		"size":        "small",
		"description": "something",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for invalid type")
	}
	text := getResultText(result)
	if !strings.Contains(text, "invalid change type") {
		t.Errorf("error should mention invalid type: %s", text)
	}
}

func TestChangeTool_Handle_InvalidSize(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "huge",
		"description": "something",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for invalid size")
	}
}

func TestChangeTool_Handle_EmptyDescription(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "  ",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for empty description")
	}
}

func TestChangeTool_Handle_ActiveChangeBlocks(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "existing fix")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "medium",
		"description": "new feature",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when active change exists")
	}
	text := getResultText(result)
	if !strings.Contains(text, "active change already exists") {
		t.Errorf("error should mention active change: %s", text)
	}
}

func TestChangeTool_Handle_AllTypeSizeCombinations(t *testing.T) {
	types := []changes.ChangeType{changes.TypeFeature, changes.TypeFix, changes.TypeRefactor, changes.TypeEnhancement}
	sizes := []changes.ChangeSize{changes.SizeSmall, changes.SizeMedium, changes.SizeLarge}

	for _, ct := range types {
		for _, cs := range sizes {
			t.Run(string(ct)+"/"+string(cs), func(t *testing.T) {
				_, cleanup := setupChangeProjectWithArtifacts(t)
				defer cleanup()

				store := changes.NewFileStore()
				tool := NewChangeTool(store)

				req := mcp.CallToolRequest{}
				req.Params.Arguments = map[string]interface{}{
					"type":        string(ct),
					"size":        string(cs),
					"description": "Test " + string(ct) + " " + string(cs),
				}

				result, err := tool.Handle(context.Background(), req)
				if err != nil {
					t.Fatalf("Handle failed: %v", err)
				}
				if isErrorResult(result) {
					t.Fatalf("expected success, got error: %s", getResultText(result))
				}
			})
		}
	}
}

func TestChangeTool_Definition(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewChangeTool(store)
	def := tool.Definition()

	if def.Name != "sdd_change" {
		t.Errorf("name = %q, want sdd_change", def.Name)
	}
}

// --- Artifact guard tests (TASK-006) ---

func TestChangeTool_Handle_NoArtifacts_SmallProceeds(t *testing.T) {
	_, cleanup := setupChangeProject(t) // no artifacts
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Quick typo fix",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("small change should proceed without artifacts, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Change Created") {
		t.Error("result should contain 'Change Created'")
	}
	if !strings.Contains(text, "No SDD artifacts found") {
		t.Error("result should contain warning about missing artifacts")
	}
	if !strings.Contains(text, "sdd_reverse_engineer") {
		t.Error("warning should mention sdd_reverse_engineer")
	}
}

func TestChangeTool_Handle_NoArtifacts_MediumBlocked(t *testing.T) {
	_, cleanup := setupChangeProject(t) // no artifacts
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "medium",
		"description": "Add auth module",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("medium change should be blocked without artifacts")
	}

	text := getResultText(result)
	if !strings.Contains(text, "No SDD artifacts found") {
		t.Errorf("error should mention missing artifacts: %s", text)
	}
	if !strings.Contains(text, "sdd_reverse_engineer") {
		t.Errorf("error should mention sdd_reverse_engineer: %s", text)
	}
}

func TestChangeTool_Handle_NoArtifacts_LargeBlocked(t *testing.T) {
	_, cleanup := setupChangeProject(t) // no artifacts
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "enhancement",
		"size":        "large",
		"description": "Major rework",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("large change should be blocked without artifacts")
	}

	text := getResultText(result)
	if !strings.Contains(text, "No SDD artifacts found") {
		t.Errorf("error should mention missing artifacts: %s", text)
	}
}

func TestChangeTool_Handle_WithArtifacts_NoWarning(t *testing.T) {
	_, cleanup := setupChangeProjectWithArtifacts(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewChangeTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Small fix with artifacts",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("should succeed, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if strings.Contains(text, "No SDD artifacts found") {
		t.Error("should NOT contain warning when artifacts exist")
	}
}
