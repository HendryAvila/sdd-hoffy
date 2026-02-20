package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/HendryAvila/sdd-hoffy/internal/pipeline"
	"github.com/HendryAvila/sdd-hoffy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// ProposeTool handles the sdd_create_proposal MCP tool.
// It saves a structured proposal document with content provided by the AI.
type ProposeTool struct {
	store    config.Store
	renderer templates.Renderer
}

// NewProposeTool creates a ProposeTool with its dependencies.
func NewProposeTool(store config.Store, renderer templates.Renderer) *ProposeTool {
	return &ProposeTool{store: store, renderer: renderer}
}

// Definition returns the MCP tool definition for registration.
func (t *ProposeTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_create_proposal",
		mcp.WithDescription(
			"Save a structured proposal document for the SDD project. "+
				"This is Stage 1 of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST first discuss the idea with the user, "+
				"ask clarifying questions, and then generate the content for each section. "+
				"Pass the ACTUAL content (not placeholders) for each section. "+
				"Requires: sdd_init_project must have been run first.",
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
		mcp.WithString("out_of_scope",
			mcp.Required(),
			mcp.Description("3-5 things this project will NOT do. This prevents scope creep. Use markdown list format. "+
				"Example: '- Will NOT handle invoicing or payments\\n- Will NOT support offline mode in v1'"),
		),
		mcp.WithString("success_criteria",
			mcp.Required(),
			mcp.Description("2-4 measurable outcomes that define success. Use markdown list format. "+
				"Example: '- Users can log time in under 10 seconds\\n- 80% of test users complete onboarding without help'"),
		),
		mcp.WithString("open_questions",
			mcp.Description("Things still undecided or unknown. Use markdown list format. "+
				"Example: '- Should we support mobile from day one?\\n- What's the deployment target?'"),
		),
	)
}

// Handle processes the sdd_create_proposal tool call.
func (t *ProposeTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	problemStatement := req.GetString("problem_statement", "")
	targetUsers := req.GetString("target_users", "")
	proposedSolution := req.GetString("proposed_solution", "")
	outOfScope := req.GetString("out_of_scope", "")
	successCriteria := req.GetString("success_criteria", "")
	openQuestions := req.GetString("open_questions", "")

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
	if outOfScope == "" {
		return mcp.NewToolResultError("'out_of_scope' is required — what does this project NOT do?"), nil
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
	if err := pipeline.RequireStage(cfg, config.StagePropose); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipeline.MarkInProgress(cfg)

	// Build proposal with REAL content from the AI.
	data := templates.ProposalData{
		Name:             cfg.Name,
		ProblemStatement: problemStatement,
		TargetUsers:      targetUsers,
		ProposedSolution: proposedSolution,
		OutOfScope:       outOfScope,
		SuccessCriteria:  successCriteria,
		OpenQuestions:     openQuestions,
	}

	content, err := t.renderer.Render(templates.Proposal, data)
	if err != nil {
		return nil, fmt.Errorf("rendering proposal: %w", err)
	}

	// Write the proposal file.
	proposalPath := config.StagePath(projectRoot, config.StagePropose)
	if err := writeStageFile(proposalPath, content); err != nil {
		return nil, fmt.Errorf("writing proposal: %w", err)
	}

	// Advance pipeline to next stage.
	if err := pipeline.Advance(cfg); err != nil {
		return nil, fmt.Errorf("advancing pipeline: %w", err)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	response := fmt.Sprintf(
		"# Proposal Created\n\n"+
			"Saved to `sdd/proposal.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 2: Specify**.\n\n"+
			"Now analyze this proposal and extract formal requirements using MoSCoW prioritization "+
			"(Must Have, Should Have, Could Have, Won't Have). Each requirement needs a unique ID "+
			"(FR-001 for functional, NFR-001 for non-functional).\n\n"+
			"Call `sdd_generate_requirements` with the extracted requirements.",
		content,
	)

	return mcp.NewToolResultText(response), nil
}
