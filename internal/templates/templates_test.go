package templates

import (
	"strings"
	"testing"
)

// --- NewRenderer ---

func TestNewRenderer_Succeeds(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() failed: %v", err)
	}
	if r == nil {
		t.Fatal("NewRenderer() returned nil")
	}
}

// --- Render: Principles ---

func TestRender_Principles(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := PrinciplesData{
		Name:            "Test Project",
		Principles:      "- Never store passwords in plain text\n- All API responses must include correlation IDs",
		CodingStandards: "- Use conventional commits\n- No magic numbers",
		DomainTruths:    "- Prices are always in cents (integer)\n- All timestamps are UTC",
	}

	result, err := r.Render(Principles, data)
	if err != nil {
		t.Fatalf("Render(Principles) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Principles",
		"## Golden Invariants",
		"Never store passwords in plain text",
		"correlation IDs",
		"## Coding Standards",
		"conventional commits",
		"## Domain Truths",
		"Prices are always in cents",
		"Hoofy", // Attribution link.
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Principles output missing: %q", check)
		}
	}
}

func TestRender_Principles_OptionalSections(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// Only required field — optional sections should NOT render.
	data := PrinciplesData{
		Name:       "Test Project",
		Principles: "- No business logic in controllers",
	}

	result, err := r.Render(Principles, data)
	if err != nil {
		t.Fatalf("Render(Principles) failed: %v", err)
	}

	if !strings.Contains(result, "## Golden Invariants") {
		t.Error("should contain Golden Invariants section")
	}
	if strings.Contains(result, "## Coding Standards") {
		t.Error("Coding Standards should NOT render when empty")
	}
	if strings.Contains(result, "## Domain Truths") {
		t.Error("Domain Truths should NOT render when empty")
	}
}

// --- Render: Charter ---

func TestRender_Charter(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := CharterData{
		Name:             "Test Project",
		ProblemStatement: "Users struggle with X",
		TargetUsers:      "Developers and designers",
		ProposedSolution: "Build a tool that does Y",
		SuccessCriteria:  "50% reduction in time spent",
		DomainContext:    "B2B SaaS for healthcare",
		Stakeholders:     "Product Owner, CTO",
		Vision:           "Become the default tool",
		Boundaries:       "### In Scope\n- Web app\n### Out of Scope\n- Mobile",
		ExistingSystems:  "Legacy PHP app",
		Constraints:      "AWS GovCloud, $500/month",
	}

	result, err := r.Render(Charter, data)
	if err != nil {
		t.Fatalf("Render(Charter) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Charter",
		"## Problem Statement",
		"Users struggle with X",
		"## Target Users",
		"Developers and designers",
		"## Proposed Solution",
		"Build a tool that does Y",
		"## Success Criteria",
		"50% reduction in time spent",
		"## Domain Context",
		"B2B SaaS for healthcare",
		"## Stakeholders",
		"Product Owner, CTO",
		"## Vision",
		"Become the default tool",
		"## Boundaries",
		"Out of Scope",
		"## Existing Systems",
		"Legacy PHP app",
		"## Constraints",
		"AWS GovCloud",
		"Hoofy", // Attribution link.
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Charter output missing: %q", check)
		}
	}
}

func TestRender_Charter_OptionalSections(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// Only required fields — optional sections should NOT render.
	data := CharterData{
		Name:             "Test Project",
		ProblemStatement: "Problem",
		TargetUsers:      "Users",
		ProposedSolution: "Solution",
		SuccessCriteria:  "Criteria",
	}

	result, err := r.Render(Charter, data)
	if err != nil {
		t.Fatalf("Render(Charter) failed: %v", err)
	}

	// Required sections must be present.
	for _, section := range []string{"## Problem Statement", "## Target Users", "## Proposed Solution", "## Success Criteria"} {
		if !strings.Contains(result, section) {
			t.Errorf("Charter should contain required section: %q", section)
		}
	}

	// Optional sections must NOT be present.
	for _, section := range []string{"## Domain Context", "## Stakeholders", "## Vision", "## Boundaries", "## Existing Systems", "## Constraints"} {
		if strings.Contains(result, section) {
			t.Errorf("Charter should NOT contain optional section when empty: %q", section)
		}
	}
}

// --- Render: Requirements ---

func TestRender_Requirements(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := RequirementsData{
		Name:          "Test Project",
		MustHave:      "- User authentication\n- Dashboard",
		ShouldHave:    "- Email notifications",
		CouldHave:     "- Dark mode",
		WontHave:      "- Mobile app",
		NonFunctional: "- Response time < 200ms",
		Constraints:   "- Must use PostgreSQL",
		Assumptions:   "- Users have modern browsers",
		Dependencies:  "- Auth0 for authentication",
	}

	result, err := r.Render(Requirements, data)
	if err != nil {
		t.Fatalf("Render(Requirements) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Requirements",
		"### Must Have",
		"User authentication",
		"### Should Have",
		"Email notifications",
		"### Could Have",
		"Dark mode",
		"### Won't Have",
		"Mobile app",
		"## Non-Functional Requirements",
		"Response time < 200ms",
		"## Constraints",
		"Must use PostgreSQL",
		"## Assumptions",
		"Users have modern browsers",
		"## Dependencies",
		"Auth0 for authentication",
		"SDD-Hoffy", // Attribution link.
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Requirements output missing: %q", check)
		}
	}
}

// --- Render: Clarifications ---

func TestRender_Clarifications(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := ClarificationsData{
		Name:         "Test Project",
		ClarityScore: 75,
		Mode:         "guided",
		Threshold:    70,
		Status:       "PASSED",
		Rounds:       "### Round 1\n\nQ: Who are the users?\nA: Developers",
	}

	result, err := r.Render(Clarifications, data)
	if err != nil {
		t.Fatalf("Render(Clarifications) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Clarifications",
		"Clarity Score: 75/100",
		"guided",
		"Threshold:** 70",
		"PASSED",
		"Round 1",
		"Who are the users?",
		"Developers",
		"Clarity Gate",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Clarifications output missing: %q", check)
		}
	}
}

// --- Render: Unknown template ---

func TestRender_UnknownTemplate(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, err = r.Render("nonexistent.md.tmpl", nil)
	if err == nil {
		t.Fatal("Render(nonexistent) should fail")
	}
}

// --- Render: Empty data ---

func TestRender_EmptyCharterData(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// Should render without error even with zero values.
	result, err := r.Render(Charter, CharterData{})
	if err != nil {
		t.Fatalf("Render(Charter, empty) failed: %v", err)
	}

	// Structure should still be present.
	if !strings.Contains(result, "## Problem Statement") {
		t.Error("empty charter should still contain section headers")
	}
}

// --- Render: Tasks ---

func TestRender_Tasks_WithWaveAssignments(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := TasksData{
		Name:               "Test Project",
		TotalTasks:         "5",
		EstimatedEffort:    "3-4 days",
		Tasks:              "### TASK-001: Scaffolding\n**Component**: Setup",
		DependencyGraph:    "TASK-001 → TASK-002",
		WaveAssignments:    "**Wave 1**:\n- TASK-001: Scaffolding\n\n**Wave 2**:\n- TASK-002: API endpoints",
		AcceptanceCriteria: "- All tests pass",
	}

	result, err := r.Render(Tasks, data)
	if err != nil {
		t.Fatalf("Render(Tasks) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Implementation Tasks",
		"**Total Tasks:** 5",
		"**Estimated Effort:** 3-4 days",
		"TASK-001",
		"TASK-001 → TASK-002",
		"## Execution Waves",
		"in parallel",
		"**Wave 1**",
		"**Wave 2**",
		"## Acceptance Criteria",
		"All tests pass",
		"SDD-Hoffy",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Tasks output missing: %q", check)
		}
	}
}

func TestRender_Tasks_WithoutWaveAssignments(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := TasksData{
		Name:               "Test Project",
		TotalTasks:         "3",
		EstimatedEffort:    "2 days",
		Tasks:              "### TASK-001: Scaffolding",
		DependencyGraph:    "TASK-001 → TASK-002",
		WaveAssignments:    "", // empty — should NOT render wave section
		AcceptanceCriteria: "- All tests pass",
	}

	result, err := r.Render(Tasks, data)
	if err != nil {
		t.Fatalf("Render(Tasks) failed: %v", err)
	}

	// Wave section must NOT be present.
	if strings.Contains(result, "## Execution Waves") {
		t.Error("Execution Waves section should NOT render when WaveAssignments is empty")
	}
	if strings.Contains(result, "in parallel") {
		t.Error("wave blockquote should NOT render when WaveAssignments is empty")
	}

	// Other sections must still be present (backwards compatibility).
	checks := []string{
		"# Test Project — Implementation Tasks",
		"TASK-001",
		"## Dependency Graph",
		"## Acceptance Criteria",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Tasks output missing: %q", check)
		}
	}
}

// --- Render: Design ---

func TestRender_Design_WithQualityAnalysis(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := DesignData{
		Name:                 "Test Project",
		ArchitectureOverview: "A modular monolith using Clean Architecture",
		TechStack:            "- **Runtime**: Go 1.25",
		Components:           "### AuthModule\n- Handles user auth",
		APIContracts:         "POST /auth/login",
		DataModel:            "### User\n| id | UUID |",
		Infrastructure:       "Docker + Railway",
		Security:             "JWT with refresh tokens",
		QualityAnalysis:      "### SOLID Compliance\n- SRP: AuthModule has single responsibility\n\n### Potential Code Smells\n- No Shotgun Surgery detected\n\n### Coupling & Cohesion\n- Low coupling between modules\n\n### Mitigations\n- DIP via interface injection",
	}

	result, err := r.Render(Design, data)
	if err != nil {
		t.Fatalf("Render(Design) failed: %v", err)
	}

	checks := []string{
		"# Test Project — Technical Design",
		"## Architecture Overview",
		"Clean Architecture",
		"## Tech Stack",
		"Go 1.25",
		"## Components",
		"AuthModule",
		"## API Contracts",
		"POST /auth/login",
		"## Data Model",
		"User",
		"## Infrastructure & Deployment",
		"Docker + Railway",
		"## Security Considerations",
		"JWT with refresh tokens",
		"## Structural Quality Analysis",
		"SOLID Compliance",
		"Shotgun Surgery",
		"Coupling & Cohesion",
		"Mitigations",
		"SDD-Hoffy", // Attribution link.
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Design output missing: %q", check)
		}
	}
}

func TestRender_Design_WithoutQualityAnalysis(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// QualityAnalysis is empty — section header should still render.
	data := DesignData{
		Name:                 "Test Project",
		ArchitectureOverview: "Microservices",
		TechStack:            "Node.js",
		Components:           "API Gateway",
		DataModel:            "Users table",
	}

	result, err := r.Render(Design, data)
	if err != nil {
		t.Fatalf("Render(Design) failed: %v", err)
	}

	// Section header should still be present even with empty content.
	if !strings.Contains(result, "## Structural Quality Analysis") {
		t.Error("Design output should contain Structural Quality Analysis header even when empty")
	}
}

// --- Renderer interface compliance ---

func TestEmbedRenderer_ImplementsRenderer(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// Compile-time interface check.
	var _ Renderer = r
}
