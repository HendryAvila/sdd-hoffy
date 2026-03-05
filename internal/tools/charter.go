package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// CharterTool handles the sdd_create_charter MCP tool.
// It saves a structured project charter with content provided by the AI.
type CharterTool struct {
	store    config.Store
	renderer templates.Renderer
	bridge   StageObserver
}

// NewCharterTool creates a CharterTool with its dependencies.
func NewCharterTool(store config.Store, renderer templates.Renderer) *CharterTool {
	return &CharterTool{store: store, renderer: renderer}
}

// SetBridge injects an optional StageObserver that gets notified
// when the charter stage completes. Nil is safe (disables bridge).
func (t *CharterTool) SetBridge(obs StageObserver) { t.bridge = obs }

// Definition returns the MCP tool definition for registration.
func (t *CharterTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_create_charter",
		mcp.WithDescription(
			"Save a structured project charter for the SDD project. "+
				"This is Stage 2 of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST first discuss the idea with the user, "+
				"ask clarifying questions, and then generate the content for each section. "+
				"Pass the ACTUAL content (not placeholders) for each section. "+
				"Requires: sdd_create_principles must have been run first.",
		),
		mcp.WithString("problem_statement",
			mcp.Required(),
			mcp.Description("The core problem this project solves. 2-3 sentences explaining the pain point. "+
				"Example: 'Freelance designers waste 30+ minutes daily tracking project hours across spreadsheets. "+
				"They need a simple tool that fits their workflow without the complexity of enterprise time-tracking software.'"),
		),
		mcp.WithString("target_users",
			mcp.Required(),
			mcp.Description("2-3 specific user personas. For each: who they are, what they need, and why they care. "+
				"Use markdown list format. "+
				"Example: '- **Freelance designers** who need to track project hours but hate complex tools\\n"+
				"- **Small agency owners** who need team visibility without enterprise overhead'"),
		),
		mcp.WithString("proposed_solution",
			mcp.Required(),
			mcp.Description("High-level description of what we're building. NO tech stack or implementation details — "+
				"just what it does for the user. "+
				"Example: 'A simple web app where freelancers log hours per project and see weekly reports'"),
		),
		mcp.WithString("success_criteria",
			mcp.Required(),
			mcp.Description("2-4 measurable outcomes that define success. Use markdown list format. "+
				"Example: '- Users can log time in under 10 seconds\\n- 80% of test users complete onboarding without help'"),
		),
		mcp.WithString("domain_context",
			mcp.Description("The business domain context — industry, market, regulatory environment. "+
				"Example: 'B2B SaaS for healthcare compliance. Subject to HIPAA regulations.'"),
		),
		mcp.WithString("stakeholders",
			mcp.Description("Key stakeholders and their interests. Use markdown list format. "+
				"Example: '- **Product Owner**: Prioritizes features based on customer feedback\\n"+
				"- **CTO**: Concerned with scalability and technical debt'"),
		),
		mcp.WithString("vision",
			mcp.Description("Long-term vision for the project — where it's headed beyond v1. "+
				"Example: 'Become the default time-tracking tool for freelancers, expanding to team management and invoicing by v3.'"),
		),
		mcp.WithString("boundaries",
			mcp.Description("What's in scope and what's explicitly out of scope. Replaces the old 'out_of_scope' field "+
				"with a more complete view. Use markdown list format. "+
				"Example: '### In Scope\\n- Web app for time tracking\\n- CSV export\\n\\n### Out of Scope\\n- Mobile app\\n- Invoicing'"),
		),
		mcp.WithString("existing_systems",
			mcp.Description("Systems this project must integrate with or replace. "+
				"Example: '- Migrating from legacy PHP app running on shared hosting\\n- Must integrate with existing Slack workspace for notifications'"),
		),
		mcp.WithString("constraints",
			mcp.Description("Technical, business, or regulatory constraints that shape the solution. "+
				"Example: '- Must deploy to AWS GovCloud (FedRAMP requirement)\\n- Budget: $500/month max for infrastructure\\n- Team: 2 developers, 1 designer'"),
		),
	)
}

// Handle processes the sdd_create_charter tool call.
func (t *CharterTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	problemStatement := req.GetString("problem_statement", "")
	targetUsers := req.GetString("target_users", "")
	proposedSolution := req.GetString("proposed_solution", "")
	successCriteria := req.GetString("success_criteria", "")
	domainContext := req.GetString("domain_context", "")
	stakeholders := req.GetString("stakeholders", "")
	vision := req.GetString("vision", "")
	boundaries := req.GetString("boundaries", "")
	existingSystems := req.GetString("existing_systems", "")
	constraints := req.GetString("constraints", "")

	// Validate required fields.
	if problemStatement == "" {
		return mcp.NewToolResultError("'problem_statement' is required — describe the problem this project solves"), nil
	}
	if targetUsers == "" {
		return mcp.NewToolResultError("'target_users' is required — who will use this?"), nil
	}
	if proposedSolution == "" {
		return mcp.NewToolResultError("'proposed_solution' is required — describe what we're building"), nil
	}
	if successCriteria == "" {
		return mcp.NewToolResultError("'success_criteria' is required — how do we know this succeeded?"), nil
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	cfg, err := t.store.Load(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate we're at the right stage.
	if err := pipeline.RequireStage(cfg, config.StageCharter); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipeline.MarkInProgress(cfg)

	// Build charter with REAL content from the AI.
	data := templates.CharterData{
		Name:             cfg.Name,
		ProblemStatement: problemStatement,
		TargetUsers:      targetUsers,
		ProposedSolution: proposedSolution,
		SuccessCriteria:  successCriteria,
		DomainContext:    domainContext,
		Stakeholders:     stakeholders,
		Vision:           vision,
		Boundaries:       boundaries,
		ExistingSystems:  existingSystems,
		Constraints:      constraints,
	}

	content, err := t.renderer.Render(templates.Charter, data)
	if err != nil {
		return nil, fmt.Errorf("rendering charter: %w", err)
	}

	// Write the charter file.
	charterPath := config.StagePath(projectRoot, config.StageCharter)
	if err := writeStageFile(charterPath, content); err != nil {
		return nil, fmt.Errorf("writing charter: %w", err)
	}

	// Advance pipeline to next stage.
	if err := pipeline.Advance(cfg); err != nil {
		return nil, fmt.Errorf("advancing pipeline: %w", err)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	notifyObserver(t.bridge, cfg.Name, config.StageCharter, content)

	response := fmt.Sprintf(
		"# Charter Created\n\n"+
			"Saved to `%s/charter.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 3: Specify**.\n\n"+
			"Now analyze this charter and extract formal requirements using MoSCoW prioritization "+
			"(Must Have, Should Have, Could Have, Won't Have). Each requirement needs a unique ID "+
			"(FR-001 for functional, NFR-001 for non-functional).\n\n"+
			"Call `sdd_generate_requirements` with the extracted requirements.",
		config.DocsDir, content,
	)

	return mcp.NewToolResultText(response), nil
}
