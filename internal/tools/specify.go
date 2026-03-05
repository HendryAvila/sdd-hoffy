package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// SpecifyTool handles the sdd_generate_requirements MCP tool.
// It saves formal requirements with content provided by the AI.
type SpecifyTool struct {
	store    config.Store
	renderer templates.Renderer
	bridge   StageObserver
}

// NewSpecifyTool creates a SpecifyTool with its dependencies.
func NewSpecifyTool(store config.Store, renderer templates.Renderer) *SpecifyTool {
	return &SpecifyTool{store: store, renderer: renderer}
}

// SetBridge injects an optional StageObserver that gets notified
// when the specify stage completes. Nil is safe (disables bridge).
func (t *SpecifyTool) SetBridge(obs StageObserver) { t.bridge = obs }

// Definition returns the MCP tool definition for registration.
func (t *SpecifyTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_generate_requirements",
		mcp.WithDescription(
			"Save formal requirements extracted from the proposal document. "+
				"This is Stage 2 of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST read the charter (docs/charter.md), "+
				"analyze it, and generate real requirements with MoSCoW prioritization. "+
				"Pass the ACTUAL requirements content (not placeholders) for each section. "+
				"Each functional requirement needs a unique ID (FR-001, FR-002...). "+
				"Each non-functional requirement needs a unique ID (NFR-001, NFR-002...). "+
				"Requires: sdd_create_charter must have been run first.",
		),
		mcp.WithString("must_have",
			mcp.Required(),
			mcp.Description("Non-negotiable requirements for launch. Use markdown list with IDs. "+
				"Example: '- **FR-001**: Users can create an account with email and password\\n"+
				"- **FR-002**: Users can log time entries with project, duration, and description'"),
		),
		mcp.WithString("should_have",
			mcp.Required(),
			mcp.Description("Important requirements that add significant value but don't block launch. "+
				"Use markdown list with IDs (continue numbering from must_have). "+
				"Example: '- **FR-005**: Users can export time entries as CSV'"),
		),
		mcp.WithString("could_have",
			mcp.Description("Nice-to-have features that can wait for a future version. "+
				"Use markdown list with IDs."),
		),
		mcp.WithString("wont_have",
			mcp.Description("Features explicitly excluded from THIS version. Being explicit prevents scope creep. "+
				"Use markdown list with IDs."),
		),
		mcp.WithString("non_functional",
			mcp.Required(),
			mcp.Description("Performance, security, scalability, usability constraints. Use NFR-XXX IDs. "+
				"Example: '- **NFR-001**: Page load time must be under 2 seconds on 3G\\n"+
				"- **NFR-002**: All user data must be encrypted at rest'"),
		),
		mcp.WithString("constraints",
			mcp.Description("Technical, business, or regulatory limitations. "+
				"Example: '- Must run on Node.js 20+\\n- Budget limited to free-tier cloud services'"),
		),
		mcp.WithString("assumptions",
			mcp.Description("What we assume to be true. If these change, requirements change too."),
		),
		mcp.WithString("dependencies",
			mcp.Description("External systems, APIs, services, or teams we depend on."),
		),
	)
}

// Handle processes the sdd_generate_requirements tool call.
func (t *SpecifyTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mustHave := req.GetString("must_have", "")
	shouldHave := req.GetString("should_have", "")
	couldHave := req.GetString("could_have", "")
	wontHave := req.GetString("wont_have", "")
	nonFunctional := req.GetString("non_functional", "")
	constraints := req.GetString("constraints", "")
	assumptions := req.GetString("assumptions", "")
	dependencies := req.GetString("dependencies", "")

	// Validate required fields.
	if mustHave == "" {
		return mcp.NewToolResultError("'must_have' is required — list the non-negotiable requirements"), nil
	}
	if shouldHave == "" {
		return mcp.NewToolResultError("'should_have' is required — list the important-but-not-blocking requirements"), nil
	}
	if nonFunctional == "" {
		return mcp.NewToolResultError("'non_functional' is required — list performance, security, and usability constraints"), nil
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	cfg, err := t.store.Load(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate pipeline stage.
	if err := pipeline.RequireStage(cfg, config.StageSpecify); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Verify the charter exists.
	charterPath := config.StagePath(projectRoot, config.StageCharter)
	charter, err := readStageFile(charterPath)
	if err != nil {
		return nil, fmt.Errorf("reading charter: %w", err)
	}
	if charter == "" {
		return mcp.NewToolResultError("charter.md is empty — run sdd_create_charter first"), nil
	}

	pipeline.MarkInProgress(cfg)

	// Fill optional fields with "None" if empty.
	if couldHave == "" {
		couldHave = "_None defined for this version._"
	}
	if wontHave == "" {
		wontHave = "_None defined for this version._"
	}
	if constraints == "" {
		constraints = "_None identified._"
	}
	if assumptions == "" {
		assumptions = "_None identified._"
	}
	if dependencies == "" {
		dependencies = "_None identified._"
	}

	// Build requirements with REAL content from the AI.
	data := templates.RequirementsData{
		Name:          cfg.Name,
		MustHave:      mustHave,
		ShouldHave:    shouldHave,
		CouldHave:     couldHave,
		WontHave:      wontHave,
		NonFunctional: nonFunctional,
		Constraints:   constraints,
		Assumptions:   assumptions,
		Dependencies:  dependencies,
	}

	// Render and write via shared function (ADR-001).
	content, err := RenderAndWriteRequirements(projectRoot, t.renderer, data, false)
	if err != nil {
		return nil, err
	}

	// Advance pipeline.
	if err := pipeline.Advance(cfg); err != nil {
		return nil, fmt.Errorf("advancing pipeline: %w", err)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	notifyObserver(t.bridge, cfg.Name, config.StageSpecify, content)

	response := fmt.Sprintf(
		"# Requirements Generated\n\n"+
			"Saved to `docs/requirements.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 3: Clarify (Clarity Gate)**.\n\n"+
			"This is the MOST IMPORTANT stage. Call `sdd_clarify` (without answers) to analyze "+
			"these requirements for ambiguities. The pipeline cannot proceed until the clarity "+
			"score reaches %d/100 (%s mode).\n\n"+
			"**Why this matters:** Ambiguous requirements are the #1 cause of AI hallucinations.",
		content, pipeline.ClarityThreshold(cfg.Mode), cfg.Mode,
	)

	return mcp.NewToolResultText(response), nil
}
