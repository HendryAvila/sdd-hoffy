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

// --- Integration tests (TASK-010) ---
//
// These tests exercise the full change pipeline end-to-end:
// create → advance through all stages → complete, using
// multiple tools together as a real user would.

func TestIntegration_FullFixSmallFlow(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)
	advanceTool := NewChangeAdvanceTool(store)
	statusTool := NewChangeStatusTool(store)

	// Step 1: Create a fix/small change.
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Fix null pointer in search handler",
	}

	result, err := changeTool.Handle(context.Background(), createReq)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("create: expected success: %s", getResultText(result))
	}
	text := getResultText(result)
	if !strings.Contains(text, "fix-null-pointer-in-search-handler") {
		t.Fatal("create: should contain slug ID")
	}

	// Step 2: Check status — should show active, stage describe in_progress.
	statusReq := mcp.CallToolRequest{}
	statusReq.Params.Arguments = map[string]interface{}{}

	result, err = statusTool.Handle(context.Background(), statusReq)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	text = getResultText(result)
	if !strings.Contains(text, "active") {
		t.Error("status: should show active")
	}
	if !strings.Contains(text, "🔄") {
		t.Error("status: should show in_progress marker")
	}

	// Step 3: Advance through all 4 stages (describe → context-check → tasks → verify).
	stageContents := []struct {
		content      string
		expectNext   string
		isCompletion bool
	}{
		{
			content:    "# Bug Description\n\nNull pointer dereference when search query is empty.\n\n## Root Cause\n\nMissing nil check on `results` before accessing `.Rows`.",
			expectNext: "context-check",
		},
		{
			content:    "# Context Check\n\nNo conflicting specs or business rules found. Safe to proceed.",
			expectNext: "tasks",
		},
		{
			content:    "# Implementation Tasks\n\n- [ ] Add nil check before accessing results.Rows\n- [ ] Add test for empty query case\n- [ ] Update error handling in search handler",
			expectNext: "verify",
		},
		{
			content:      "# Verification\n\n## Tests Added\n- TestSearchHandler_EmptyQuery\n- TestSearchHandler_NilResults\n\n## Manual Testing\n- Verified empty query no longer crashes\n- Existing search functionality unaffected",
			isCompletion: true,
		},
	}

	for i, sc := range stageContents {
		advanceReq := mcp.CallToolRequest{}
		advanceReq.Params.Arguments = map[string]interface{}{
			"content": sc.content,
		}

		result, err = advanceTool.Handle(context.Background(), advanceReq)
		if err != nil {
			t.Fatalf("advance stage %d: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("advance stage %d: expected success: %s", i, getResultText(result))
		}

		text = getResultText(result)
		if sc.isCompletion {
			if !strings.Contains(text, "Change completed!") {
				t.Errorf("stage %d: should contain 'Change completed!'", i)
			}
		} else {
			if !strings.Contains(text, sc.expectNext) {
				t.Errorf("stage %d: should mention next stage %q", i, sc.expectNext)
			}
		}
	}

	// Step 4: Verify status by ID shows completed.
	statusReq.Params.Arguments = map[string]interface{}{
		"change_id": "fix-null-pointer-in-search-handler",
	}
	result, err = statusTool.Handle(context.Background(), statusReq)
	if err != nil {
		t.Fatalf("final status: %v", err)
	}
	text = getResultText(result)
	if !strings.Contains(text, "completed") {
		t.Error("final status: should show completed")
	}

	// Step 5: Cannot create another change while completed one exists
	// (LoadActive returns nil for completed — so a new one SHOULD work).
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Another fix",
	}
	result, err = changeTool.Handle(context.Background(), createReq)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("second create: should succeed after first completed: %s", getResultText(result))
	}
}

func TestIntegration_FeatureLargeWithADRs(t *testing.T) {
	tmpDir, cleanup := setupChangeProjectWithArtifacts(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)
	advanceTool := NewChangeAdvanceTool(store)
	adrTool := NewADRTool(store)
	statusTool := NewChangeStatusTool(store)

	// Create feature/large change.
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "large",
		"description": "Add user authentication",
	}

	if _, err := changeTool.Handle(context.Background(), createReq); err != nil {
		t.Fatalf("create: %v", err)
	}

	// feature/large: charter → context-check → spec → clarify → design → tasks → verify
	stageContents := []string{
		"# Charter\n\nAdd JWT-based user authentication with email/password login.",
		"# Context Check\n\nNo conflicting specs found. No prior auth changes detected.",
		"# Specification\n\n## FR-001: User Registration\nUsers can register with email and password.",
		"# Clarifications\n\nQ: OAuth support?\nA: Not in v1, only email/password.",
		"# Design\n\n## Architecture\nJWT with refresh tokens, bcrypt hashing.\n\n## Components\n- AuthModule\n- UserModule",
		"# Tasks\n\n### TASK-001: Create user model\n### TASK-002: Implement JWT middleware",
		"# Verification\n\nAll requirements covered. Tests passing.",
	}

	// Capture an ADR after the design stage (stage index 4).
	for i, content := range stageContents {
		advanceReq := mcp.CallToolRequest{}
		advanceReq.Params.Arguments = map[string]interface{}{
			"content": content,
		}

		result, err := advanceTool.Handle(context.Background(), advanceReq)
		if err != nil {
			t.Fatalf("advance stage %d: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("advance stage %d: error: %s", i, getResultText(result))
		}

		// After design stage (index 4), capture an ADR.
		if i == 4 {
			adrReq := mcp.CallToolRequest{}
			adrReq.Params.Arguments = map[string]interface{}{
				"title":                 "JWT over session cookies",
				"context":               "Need stateless auth for API-first architecture.",
				"decision":              "Use JWT with refresh tokens.",
				"rationale":             "Stateless, works with mobile and SPA clients.",
				"alternatives_rejected": "Session cookies (require sticky sessions), OAuth only (overkill for v1)",
			}

			adrResult, adrErr := adrTool.Handle(context.Background(), adrReq)
			if adrErr != nil {
				t.Fatalf("adr: %v", adrErr)
			}
			if isErrorResult(adrResult) {
				t.Fatalf("adr: error: %s", getResultText(adrResult))
			}
			adrText := getResultText(adrResult)
			if !strings.Contains(adrText, "ADR-001") {
				t.Error("ADR should be numbered ADR-001")
			}
		}
	}

	// Verify the completed change has the ADR.
	change, err := store.Load(tmpDir, "add-user-authentication")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if change.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", change.Status)
	}
	if len(change.ADRs) != 1 {
		t.Fatalf("ADRs count = %d, want 1", len(change.ADRs))
	}

	// Verify ADR file exists in unified docs/adrs/ directory.
	adrPath := filepath.Join(tmpDir, "docs", "adrs", "001-jwt-over-session-cookies.md")
	if _, err := os.Stat(adrPath); os.IsNotExist(err) {
		t.Error("ADR file should exist at docs/adrs/001-jwt-over-session-cookies.md")
	}

	// Verify status shows ADRs.
	statusReq := mcp.CallToolRequest{}
	statusReq.Params.Arguments = map[string]interface{}{
		"change_id": "add-user-authentication",
	}
	result, err := statusTool.Handle(context.Background(), statusReq)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	text := getResultText(result)
	if !strings.Contains(text, "ADR-001") {
		t.Error("status should show ADR-001")
	}

	// Verify all 7 stage artifacts exist.
	expectedFiles := []string{
		"charter.md", "context-check.md", "spec.md", "clarify.md",
		"design.md", "tasks.md", "verify.md",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, "docs", "changes", "add-user-authentication", f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("%s should exist", f)
		}
	}
}

func TestIntegration_ConcurrentChangeRejection(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)

	// Create first change.
	req1 := mcp.CallToolRequest{}
	req1.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "First fix",
	}
	result, err := changeTool.Handle(context.Background(), req1)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("first create: error: %s", getResultText(result))
	}

	// Attempt second change — should be rejected.
	req2 := mcp.CallToolRequest{}
	req2.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "medium",
		"description": "Second feature",
	}
	result, err = changeTool.Handle(context.Background(), req2)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("second create should be rejected while first is active")
	}
	text := getResultText(result)
	if !strings.Contains(text, "active change already exists") {
		t.Errorf("error should mention existing active change: %s", text)
	}
}

func TestIntegration_RefactorMediumFlow(t *testing.T) {
	tmpDir, cleanup := setupChangeProjectWithArtifacts(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)
	advanceTool := NewChangeAdvanceTool(store)

	// refactor/medium: scope → context-check → design → tasks → verify
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "refactor",
		"size":        "medium",
		"description": "Extract auth module",
	}

	if _, err := changeTool.Handle(context.Background(), createReq); err != nil {
		t.Fatalf("create: %v", err)
	}

	stageContents := []string{
		"# Scope\n\n## What Changes\n- Extract auth logic from handlers into AuthModule\n\n## What Doesn't Change\n- API contract remains the same\n- Database schema unchanged",
		"# Context Check\n\nNo conflicting specs found. Safe to proceed with refactor.",
		"# Design\n\n## AuthModule\n- Handles JWT creation and validation\n- Encapsulates bcrypt hashing\n- Exposes clean interface for handlers",
		"# Tasks\n\n### TASK-001: Create AuthModule interface\n### TASK-002: Move JWT logic\n### TASK-003: Update handlers to use AuthModule",
		"# Verification\n\n- All existing tests pass\n- No API changes\n- AuthModule has 95% coverage",
	}

	for i, c := range stageContents {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"content": c}
		result, err := advanceTool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("stage %d: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("stage %d: error: %s", i, getResultText(result))
		}
	}

	// Verify completed.
	change, _ := store.Load(tmpDir, "extract-auth-module")
	if change.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", change.Status)
	}

	// Verify correct files: scope.md (not describe.md).
	scopePath := filepath.Join(tmpDir, "docs", "changes", "extract-auth-module", "scope.md")
	if _, err := os.Stat(scopePath); os.IsNotExist(err) {
		t.Error("scope.md should exist for refactor flow")
	}
	describePath := filepath.Join(tmpDir, "docs", "changes", "extract-auth-module", "describe.md")
	if _, err := os.Stat(describePath); !os.IsNotExist(err) {
		t.Error("describe.md should NOT exist for refactor flow")
	}
}

func TestIntegration_StandaloneADR(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	adrTool := NewADRTool(store)

	// Create ADR without any active change.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"title":     "Use Go modules over dep",
		"context":   "Need a dependency management solution.",
		"decision":  "Use Go modules (go.mod).",
		"rationale": "Official Go toolchain support, no external tool needed.",
		"status":    "accepted",
	}

	result, err := adrTool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "ADR Captured") {
		t.Error("should contain 'ADR Captured'")
	}
	if !strings.Contains(text, "docs/adrs/") {
		t.Error("should show docs/adrs/ path for standalone ADR")
	}
}

func TestIntegration_MultipleADRsDuringChange(t *testing.T) {
	tmpDir, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)
	adrTool := NewADRTool(store)

	// Create a change.
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "feature",
		"size":        "small",
		"description": "Multi ADR test",
	}
	if _, err := changeTool.Handle(context.Background(), createReq); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Create 3 ADRs.
	for i := 1; i <= 3; i++ {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"title":     "Decision " + string(rune('0'+i)),
			"context":   "Context",
			"decision":  "Decision",
			"rationale": "Rationale",
		}
		result, err := adrTool.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("adr %d: %v", i, err)
		}
		if isErrorResult(result) {
			t.Fatalf("adr %d: error: %s", i, getResultText(result))
		}
	}

	// Verify all 3 ADRs exist.
	change, _ := store.Load(tmpDir, "multi-adr-test")
	if len(change.ADRs) != 3 {
		t.Fatalf("ADRs count = %d, want 3", len(change.ADRs))
	}
	if change.ADRs[0] != "ADR-001" || change.ADRs[1] != "ADR-002" || change.ADRs[2] != "ADR-003" {
		t.Errorf("ADR IDs = %v, want [ADR-001, ADR-002, ADR-003]", change.ADRs)
	}

	// Verify files exist in unified docs/adrs/ directory.
	expectedADRFiles := []string{
		"001-decision-1.md",
		"002-decision-2.md",
		"003-decision-3.md",
	}
	for _, f := range expectedADRFiles {
		adrPath := filepath.Join(tmpDir, "docs", "adrs", f)
		if _, err := os.Stat(adrPath); os.IsNotExist(err) {
			t.Errorf("%s should exist in docs/adrs/", f)
		}
	}
}

func TestIntegration_AdvanceAfterCompletion(t *testing.T) {
	_, cleanup, _ := createActiveChange(t, changes.TypeFix, changes.SizeSmall, "advance after done")
	defer cleanup()

	store := changes.NewFileStore()
	advanceTool := NewChangeAdvanceTool(store)

	// Complete all stages (fix/small: describe → context-check → tasks → verify).
	stages := []string{
		"# Describe\n\nContent.",
		"# Context Check\n\nContent.",
		"# Tasks\n\nContent.",
		"# Verify\n\nContent.",
	}
	for _, c := range stages {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"content": c}
		if _, err := advanceTool.Handle(context.Background(), req); err != nil {
			t.Fatalf("advance: %v", err)
		}
	}

	// Try to advance again — should fail (no active change).
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"content": "# More content"}
	result, err := advanceTool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("post-completion advance: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when trying to advance after completion")
	}
}

func TestIntegration_BridgeAcrossTools(t *testing.T) {
	_, cleanup := setupChangeProject(t)
	defer cleanup()

	store := changes.NewFileStore()
	changeTool := NewChangeTool(store)
	advanceTool := NewChangeAdvanceTool(store)
	adrTool := NewADRTool(store)

	// Track all bridge notifications.
	var notifications []struct {
		changeID string
		stage    changes.ChangeStage
	}
	mock := &mockChangeObserver{
		fn: func(changeID string, stage changes.ChangeStage, content string) {
			notifications = append(notifications, struct {
				changeID string
				stage    changes.ChangeStage
			}{changeID, stage})
		},
	}
	advanceTool.SetBridge(mock)
	adrTool.SetBridge(mock)

	// Create change.
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "fix",
		"size":        "small",
		"description": "Bridge integration",
	}
	if _, err := changeTool.Handle(context.Background(), createReq); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Advance describe stage.
	advanceReq := mcp.CallToolRequest{}
	advanceReq.Params.Arguments = map[string]interface{}{
		"content": "# Description\n\nBridge test.",
	}
	if _, err := advanceTool.Handle(context.Background(), advanceReq); err != nil {
		t.Fatalf("advance: %v", err)
	}

	// Capture an ADR.
	adrReq := mcp.CallToolRequest{}
	adrReq.Params.Arguments = map[string]interface{}{
		"title":     "Bridge ADR",
		"context":   "Context",
		"decision":  "Decision",
		"rationale": "Rationale",
	}
	if _, err := adrTool.Handle(context.Background(), adrReq); err != nil {
		t.Fatalf("adr: %v", err)
	}

	// Should have 2 notifications: describe stage + adr.
	if len(notifications) != 2 {
		t.Fatalf("notifications = %d, want 2", len(notifications))
	}
	if notifications[0].stage != changes.StageDescribe {
		t.Errorf("first notification stage = %q, want describe", notifications[0].stage)
	}
	if notifications[1].stage != "adr" {
		t.Errorf("second notification stage = %q, want adr", notifications[1].stage)
	}
}

func TestIntegration_ContextCheckToolInFlow(t *testing.T) {
	tmpDir, cleanup := setupChangeProject(t)
	defer cleanup()

	changeStore := changes.NewFileStore()
	changeTool := NewChangeTool(changeStore)
	advanceTool := NewChangeAdvanceTool(changeStore)
	contextCheckTool := NewContextCheckTool(changeStore, nil) // nil memory — degrades gracefully

	// Create SDD artifacts that context-check will scan.
	reqContent := "# Requirements\n\n- **FR-001**: Users can register with email and password\n- **FR-002**: Users can log in and receive a JWT token\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "docs", "requirements.md"), []byte(reqContent), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	// Create an enhancement/small change.
	createReq := mcp.CallToolRequest{}
	createReq.Params.Arguments = map[string]interface{}{
		"type":        "enhancement",
		"size":        "small",
		"description": "Add password reset via email token",
	}
	result, err := changeTool.Handle(context.Background(), createReq)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("create error: %s", getResultText(result))
	}

	// Stage 1: Advance describe stage.
	advanceReq := mcp.CallToolRequest{}
	advanceReq.Params.Arguments = map[string]interface{}{
		"content": "# Description\n\nAdd a password reset flow where users receive an email with a reset token.",
	}
	result, err = advanceTool.Handle(context.Background(), advanceReq)
	if err != nil {
		t.Fatalf("advance describe: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("advance describe error: %s", getResultText(result))
	}
	text := getResultText(result)
	if !strings.Contains(text, "context-check") {
		t.Error("after describe, next stage should be context-check")
	}

	// Stage 2: Call the context-check SCANNER tool to get a report.
	checkReq := mcp.CallToolRequest{}
	checkReq.Params.Arguments = map[string]interface{}{
		"change_description": "Add password reset via email token",
	}
	result, err = contextCheckTool.Handle(context.Background(), checkReq)
	if err != nil {
		t.Fatalf("context-check tool: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("context-check tool error: %s", getResultText(result))
	}

	// Verify the scanner found our requirements artifact.
	report := getResultText(result)
	if !strings.Contains(report, "requirements.md") {
		t.Error("context-check report should mention requirements.md")
	}

	// Now advance context-check with the analysis (AI would generate this).
	advanceReq.Params.Arguments = map[string]interface{}{
		"content": "# Context Check\n\n## Artifacts Scanned\n- requirements.md (2 FRs found)\n\n## Impact Classification\n- Non-breaking: adds new behavior (password reset) without modifying existing auth flow\n\n## Conflicts\nNone — password reset is additive to FR-001/FR-002.\n\n## Verdict\nAll clear. Safe to proceed.",
	}
	result, err = advanceTool.Handle(context.Background(), advanceReq)
	if err != nil {
		t.Fatalf("advance context-check: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("advance context-check error: %s", getResultText(result))
	}
	text = getResultText(result)
	if !strings.Contains(text, "tasks") {
		t.Error("after context-check, next stage should be tasks")
	}

	// Stage 3: Advance tasks.
	advanceReq.Params.Arguments = map[string]interface{}{
		"content": "# Tasks\n\n### TASK-001: Create password reset token model\n### TASK-002: Add POST /auth/reset-password endpoint\n### TASK-003: Send reset email with token link",
	}
	if _, err = advanceTool.Handle(context.Background(), advanceReq); err != nil {
		t.Fatalf("advance tasks: %v", err)
	}

	// Stage 4: Advance verify — completes the change.
	advanceReq.Params.Arguments = map[string]interface{}{
		"content": "# Verification\n\nAll tasks trace to password reset functionality.\nNo conflicts with existing auth requirements (FR-001, FR-002).\nTest coverage planned for all 3 tasks.",
	}
	result, err = advanceTool.Handle(context.Background(), advanceReq)
	if err != nil {
		t.Fatalf("advance verify: %v", err)
	}
	text = getResultText(result)
	if !strings.Contains(text, "Change completed!") {
		t.Error("final stage should complete the change")
	}

	// Verify the context-check artifact was written.
	checkPath := filepath.Join(tmpDir, "docs", "changes", "add-password-reset-via-email-token", "context-check.md")
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		t.Error("context-check.md artifact should exist")
	}

	// Verify the change is completed.
	change, err := changeStore.Load(tmpDir, "add-password-reset-via-email-token")
	if err != nil {
		t.Fatalf("load change: %v", err)
	}
	if change.Status != changes.StatusCompleted {
		t.Errorf("status = %q, want completed", change.Status)
	}
}
