// Package templates provides embedded markdown templates for SDD artifacts.
//
// Uses Go's embed package to bundle templates at compile time — no external
// file dependencies at runtime (Dependency Inversion: depend on abstractions,
// the binary carries everything it needs).
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.tmpl
var templateFS embed.FS

// Name constants for each template.
const (
	Principles        = "principles.md.tmpl"
	Charter           = "charter.md.tmpl"
	Requirements      = "requirements.md.tmpl"
	BusinessRules     = "business-rules.md.tmpl"
	Clarifications    = "clarifications.md.tmpl"
	Design            = "design.md.tmpl"
	Tasks             = "tasks.md.tmpl"
	AgentInstructions = "agent-instructions.md.tmpl"
)

// Renderer renders markdown templates with provided data.
// Abstracted as interface for testability (DIP).
type Renderer interface {
	Render(templateName string, data any) (string, error)
}

// EmbedRenderer renders templates from the embedded filesystem.
type EmbedRenderer struct {
	templates *template.Template
}

// NewRenderer creates a renderer with all embedded templates parsed.
func NewRenderer() (*EmbedRenderer, error) {
	tmpl, err := template.ParseFS(templateFS, "*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &EmbedRenderer{templates: tmpl}, nil
}

// Render executes the named template with the given data and returns
// the resulting markdown string.
func (r *EmbedRenderer) Render(templateName string, data any) (string, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("rendering %s: %w", templateName, err)
	}
	return buf.String(), nil
}

// --- Template data structures ---

// PrinciplesData holds the data for rendering project principles.
type PrinciplesData struct {
	Name            string
	Principles      string
	CodingStandards string
	DomainTruths    string
}

// CharterData holds the data for rendering a project charter.
type CharterData struct {
	Name             string
	ProblemStatement string
	TargetUsers      string
	ProposedSolution string
	SuccessCriteria  string
	DomainContext    string
	Stakeholders     string
	Vision           string
	Boundaries       string
	ExistingSystems  string
	Constraints      string
}

// RequirementsData holds the data for rendering requirements.
type RequirementsData struct {
	Name          string
	MustHave      string
	ShouldHave    string
	CouldHave     string
	WontHave      string
	NonFunctional string
	Constraints   string
	Assumptions   string
	Dependencies  string
}

// ClarificationsData holds the data for rendering the clarifications log.
type ClarificationsData struct {
	Name         string
	ClarityScore int
	Mode         string
	Threshold    int
	Status       string
	Rounds       string
}

// DesignData holds the data for rendering a technical design document.
type DesignData struct {
	Name                 string
	ArchitectureOverview string
	TechStack            string
	Components           string
	APIContracts         string
	DataModel            string
	Infrastructure       string
	Security             string
	QualityAnalysis      string
}

// TasksData holds the data for rendering an implementation task breakdown.
type TasksData struct {
	Name               string
	TotalTasks         string
	EstimatedEffort    string
	Tasks              string
	DependencyGraph    string
	WaveAssignments    string // optional: parallel execution wave groupings
	AcceptanceCriteria string
}

// BusinessRulesData holds the data for rendering business rules.
type BusinessRulesData struct {
	Name        string
	Definitions string
	Facts       string
	Constraints string
	Derivations string // optional: computed/inferred knowledge
	Glossary    string // optional: additional domain vocabulary
}

// AgentInstructionsData holds the data for rendering the agent instructions section.
type AgentInstructionsData struct {
	Name    string
	DocsDir string
}
