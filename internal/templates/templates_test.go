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

// --- Render: Proposal ---

func TestRender_Proposal(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := ProposalData{
		Name:             "Test Project",
		ProblemStatement: "Users struggle with X",
		TargetUsers:      "Developers and designers",
		ProposedSolution: "Build a tool that does Y",
		OutOfScope:       "Mobile support, offline mode",
		SuccessCriteria:  "50% reduction in time spent",
		OpenQuestions:    "What about edge case Z?",
	}

	result, err := r.Render(Proposal, data)
	if err != nil {
		t.Fatalf("Render(Proposal) failed: %v", err)
	}

	// Verify key sections are present.
	checks := []string{
		"# Test Project — Proposal",
		"## Problem Statement",
		"Users struggle with X",
		"## Target Users",
		"Developers and designers",
		"## Proposed Solution",
		"Build a tool that does Y",
		"## Out of Scope",
		"Mobile support, offline mode",
		"## Success Criteria",
		"50% reduction in time spent",
		"## Open Questions",
		"What about edge case Z?",
		"SDD-Hoffy", // Attribution link.
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Proposal output missing: %q", check)
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

func TestRender_EmptyProposalData(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	// Should render without error even with zero values.
	result, err := r.Render(Proposal, ProposalData{})
	if err != nil {
		t.Fatalf("Render(Proposal, empty) failed: %v", err)
	}

	// Structure should still be present.
	if !strings.Contains(result, "## Problem Statement") {
		t.Error("empty proposal should still contain section headers")
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
