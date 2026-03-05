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

// --- ADRTool tests ---

func TestADRTool_Definition(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewADRTool(store)
	def := tool.Definition()

	if def.Name != "sdd_adr" {
		t.Errorf("name = %q, want sdd_adr", def.Name)
	}
}

func TestADRTool_Handle_WithActiveChange(t *testing.T) {
	tmpDir, cleanup, change := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "adr with active")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Use PostgreSQL over MongoDB",
		"context":   "Need to store relational data with transactions.",
		"decision":  "Use PostgreSQL for all persistent storage.",
		"rationale": "ACID compliance required. Data is inherently relational.",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "ADR Captured") {
		t.Error("result should contain 'ADR Captured'")
	}
	if !strings.Contains(text, "ADR-001") {
		t.Error("result should contain ADR ID 'ADR-001'")
	}
	if !strings.Contains(text, change.ID) {
		t.Error("result should reference the active change ID")
	}
	if !strings.Contains(text, "docs/adrs/") {
		t.Error("result should show docs/adrs/ path")
	}

	// Verify ADR file was written to unified docs/adrs/ directory.
	adrPath := filepath.Join(tmpDir, "docs", "adrs", "001-use-postgresql-over-mongodb.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("ADR file should exist at %s: %v", adrPath, err)
	}
	adrContent := string(data)
	if !strings.Contains(adrContent, "Use PostgreSQL over MongoDB") {
		t.Error("ADR file should contain the title")
	}
	if !strings.Contains(adrContent, "ACID compliance") {
		t.Error("ADR file should contain the rationale")
	}
	if !strings.Contains(adrContent, change.ID) {
		t.Error("ADR file should reference the change ID")
	}

	// Verify change record was updated.
	loaded, err := store.Load(tmpDir, change.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.ADRs) != 1 {
		t.Fatalf("ADRs count = %d, want 1", len(loaded.ADRs))
	}
	if loaded.ADRs[0] != "ADR-001" {
		t.Errorf("ADR ID = %q, want ADR-001", loaded.ADRs[0])
	}
}

func TestADRTool_Handle_MultipleADRs(t *testing.T) {
	tmpDir, cleanup, change := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "multi adr")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	// Create first ADR.
	req1 := mcp.CallToolRequest{}
	req1.Params.Arguments = map[string]interface{}{
		"title":     "First decision",
		"context":   "Context 1",
		"decision":  "Decision 1",
		"rationale": "Rationale 1",
	}
	if _, err := tool.Handle(context.Background(), req1); err != nil {
		t.Fatalf("first ADR failed: %v", err)
	}

	// Create second ADR.
	req2 := mcp.CallToolRequest{}
	req2.Params.Arguments = map[string]interface{}{
		"title":     "Second decision",
		"context":   "Context 2",
		"decision":  "Decision 2",
		"rationale": "Rationale 2",
	}
	result, err := tool.Handle(context.Background(), req2)
	if err != nil {
		t.Fatalf("second ADR failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "ADR-002") {
		t.Error("second ADR should be numbered ADR-002")
	}

	// Verify both files exist in docs/adrs/ with sequential numbering.
	adr1Path := filepath.Join(tmpDir, "docs", "adrs", "001-first-decision.md")
	adr2Path := filepath.Join(tmpDir, "docs", "adrs", "002-second-decision.md")
	if _, err := os.Stat(adr1Path); os.IsNotExist(err) {
		t.Errorf("ADR-001 should exist at %s", adr1Path)
	}
	if _, err := os.Stat(adr2Path); os.IsNotExist(err) {
		t.Errorf("ADR-002 should exist at %s", adr2Path)
	}

	// Verify change record has both.
	loaded, err := store.Load(tmpDir, change.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.ADRs) != 2 {
		t.Fatalf("ADRs count = %d, want 2", len(loaded.ADRs))
	}
}

func TestADRTool_Handle_Standalone(t *testing.T) {
	tmpDir, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Standalone decision",
		"context":   "No active change exists.",
		"decision":  "Use central ADR storage.",
		"rationale": "Unified location for all decisions.",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "ADR Captured") {
		t.Error("result should contain 'ADR Captured'")
	}
	if !strings.Contains(text, "docs/adrs/") {
		t.Error("result should show docs/adrs/ path")
	}

	// Verify file was still created (not memory-only anymore).
	adrPath := filepath.Join(tmpDir, "docs", "adrs", "001-standalone-decision.md")
	if _, err := os.Stat(adrPath); os.IsNotExist(err) {
		t.Errorf("standalone ADR should still create a file at %s", adrPath)
	}
}

func TestADRTool_Handle_MissingTitle(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"context":   "Some context",
		"decision":  "Some decision",
		"rationale": "Some rationale",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for missing title")
	}
	text := getResultText(result)
	if !strings.Contains(text, "title") {
		t.Errorf("error should mention 'title': %s", text)
	}
}

func TestADRTool_Handle_MissingContext(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Some title",
		"decision":  "Some decision",
		"rationale": "Some rationale",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for missing context")
	}
}

func TestADRTool_Handle_MissingDecision(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Some title",
		"context":   "Some context",
		"rationale": "Some rationale",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for missing decision")
	}
}

func TestADRTool_Handle_MissingRationale(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":    "Some title",
		"context":  "Some context",
		"decision": "Some decision",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for missing rationale")
	}
}

func TestADRTool_Handle_InvalidStatus(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Some title",
		"context":   "Some context",
		"decision":  "Some decision",
		"rationale": "Some rationale",
		"status":    "invalid",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for invalid status")
	}
	text := getResultText(result)
	if !strings.Contains(text, "invalid") {
		t.Errorf("error should mention invalid status: %s", text)
	}
}

func TestADRTool_Handle_ValidStatuses(t *testing.T) {
	statuses := []string{"proposed", "accepted", "deprecated", "superseded"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			_, cleanup := setupChangeProject(t)
			defer cleanup()

			store := changes.NewFileStore()
			tool := NewADRTool(store)

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]interface{}{
				"title":     "Test " + status,
				"context":   "Context",
				"decision":  "Decision",
				"rationale": "Rationale",
				"status":    status,
			}

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if isErrorResult(result) {
				t.Fatalf("status %q should be valid, got error: %s", status, getResultText(result))
			}

			text := getResultText(result)
			if !strings.Contains(text, status) {
				t.Errorf("result should show status %q", status)
			}
		})
	}
}

func TestADRTool_Handle_DefaultStatus(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Default status test",
		"context":   "Context",
		"decision":  "Decision",
		"rationale": "Rationale",
		// No status provided — should default to "accepted".
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "accepted") {
		t.Error("default status should be 'accepted'")
	}
}

func TestADRTool_Handle_WithAlternatives(t *testing.T) {
	tmpDir, cleanup, _ := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "adr alternatives")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":                 "DB choice",
		"context":               "Need a database.",
		"decision":              "PostgreSQL.",
		"rationale":             "ACID required.",
		"alternatives_rejected": "MongoDB (no joins), DynamoDB (too expensive)",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Alternatives Rejected") {
		t.Error("result should contain 'Alternatives Rejected' section")
	}

	// Verify file content includes alternatives — now in docs/adrs/.
	adrPath := filepath.Join(tmpDir, "docs", "adrs", "001-db-choice.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("read ADR file: %v", err)
	}
	if !strings.Contains(string(data), "MongoDB") {
		t.Error("ADR file should contain alternatives")
	}
}

func TestADRTool_Handle_WithoutAlternatives(t *testing.T) {
	tmpDir, cleanup, _ := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "no alternatives")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Simple decision",
		"context":   "Context.",
		"decision":  "Decision.",
		"rationale": "Rationale.",
	}

	if _, err := tool.Handle(context.Background(), req); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Verify file content does NOT include alternatives section.
	adrPath := filepath.Join(tmpDir, "docs", "adrs", "001-simple-decision.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("read ADR file: %v", err)
	}
	if strings.Contains(string(data), "Alternatives Rejected") {
		t.Error("ADR file should NOT contain 'Alternatives Rejected' when none provided")
	}
}

func TestADRTool_SetBridge(t *testing.T) {
	store := changes.NewFileStore()
	tool := NewADRTool(store)

	tool.SetBridge(nil)
	if tool.bridge != nil {
		t.Error("bridge should be nil after SetBridge(nil)")
	}
}

func TestADRTool_Handle_BridgeNotification_WithChange(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFeature, changes.SizeLarge, "adr bridge test")
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	var notified bool
	var notifiedStage changes.ChangeStage
	mock := &mockChangeObserver{
		fn: func(changeID string, stage changes.ChangeStage, content string) {
			notified = true
			notifiedStage = stage
		},
	}
	tool.SetBridge(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Bridge test",
		"context":   "Context",
		"decision":  "Decision",
		"rationale": "Rationale",
	}

	if _, err := tool.Handle(context.Background(), req); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if !notified {
		t.Error("bridge should have been notified")
	}
	if notifiedStage != "adr" {
		t.Errorf("notified stage = %q, want 'adr'", notifiedStage)
	}
}

func TestADRTool_Handle_BridgeNotification_Standalone(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	var notified bool
	var notifiedChangeID string
	mock := &mockChangeObserver{
		fn: func(changeID string, stage changes.ChangeStage, content string) {
			notified = true
			notifiedChangeID = changeID
		},
	}
	tool.SetBridge(mock)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Standalone bridge",
		"context":   "Context",
		"decision":  "Decision",
		"rationale": "Rationale",
	}

	if _, err := tool.Handle(context.Background(), req); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if !notified {
		t.Error("bridge should have been notified even for standalone ADR")
	}
	if notifiedChangeID != "" {
		t.Errorf("standalone ADR should have empty changeID, got %q", notifiedChangeID)
	}
}

func TestADRTool_Handle_WhitespaceFields(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	tool := NewADRTool(store)

	// All required fields are whitespace.
	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{"whitespace title", map[string]interface{}{"title": "  ", "context": "c", "decision": "d", "rationale": "r"}},
		{"whitespace context", map[string]interface{}{"title": "t", "context": "  ", "decision": "d", "rationale": "r"}},
		{"whitespace decision", map[string]interface{}{"title": "t", "context": "c", "decision": "  ", "rationale": "r"}},
		{"whitespace rationale", map[string]interface{}{"title": "t", "context": "c", "decision": "d", "rationale": "  "}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.args

			result, err := tool.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle failed: %v", err)
			}
			if !isErrorResult(result) {
				t.Error("should return error for whitespace-only required field")
			}
		})
	}
}

// --- Unit tests for ADR helpers ---

func TestNextADRNumber_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if got := nextADRNumber(dir); got != 1 {
		t.Errorf("nextADRNumber(empty) = %d, want 1", got)
	}
}

func TestNextADRNumber_WithExisting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "001-first.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "002-second.md"), []byte(""), 0o644)

	if got := nextADRNumber(dir); got != 3 {
		t.Errorf("nextADRNumber = %d, want 3", got)
	}
}

func TestNextADRNumber_NonSequential(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "001-first.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "005-jumped.md"), []byte(""), 0o644)

	if got := nextADRNumber(dir); got != 6 {
		t.Errorf("nextADRNumber = %d, want 6", got)
	}
}

func TestNextADRNumber_IgnoresNonMatching(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "001-first.md"), []byte(""), 0o644)

	if got := nextADRNumber(dir); got != 2 {
		t.Errorf("nextADRNumber = %d, want 2", got)
	}
}

func TestSlugifyTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Use PostgreSQL over MongoDB", "use-postgresql-over-mongodb"},
		{"Fix FTS5 empty query crash", "fix-fts5-empty-query-crash"},
		{"Simple", "simple"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Special!@#Characters$%^", "special-characters"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := slugifyTitle(tc.input); got != tc.want {
				t.Errorf("slugifyTitle(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestListADRFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "002-second.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "001-first.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "README.txt"), []byte(""), 0o644) // not .md

	files := listADRFiles(dir)
	if len(files) != 2 {
		t.Fatalf("listADRFiles = %d files, want 2", len(files))
	}
	if files[0] != "001-first.md" {
		t.Errorf("files[0] = %q, want 001-first.md", files[0])
	}
	if files[1] != "002-second.md" {
		t.Errorf("files[1] = %q, want 002-second.md", files[1])
	}
}
