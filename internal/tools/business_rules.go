package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// BusinessRulesTool handles the sdd_create_business_rules MCP tool.
// It saves business rules extracted from requirements using BRG taxonomy
// and DDD Ubiquitous Language patterns.
type BusinessRulesTool struct {
	store    config.Store
	renderer templates.Renderer
	bridge   StageObserver
}

// NewBusinessRulesTool creates a BusinessRulesTool with its dependencies.
func NewBusinessRulesTool(store config.Store, renderer templates.Renderer) *BusinessRulesTool {
	return &BusinessRulesTool{store: store, renderer: renderer}
}

// SetBridge injects an optional StageObserver that gets notified
// when the business-rules stage completes. Nil is safe (disables bridge).
func (t *BusinessRulesTool) SetBridge(obs StageObserver) { t.bridge = obs }

// Definition returns the MCP tool definition for registration.
func (t *BusinessRulesTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_create_business_rules",
		mcp.WithDescription(
			"Save business rules extracted from the requirements document. "+
				"This is Stage 3 of the SDD pipeline (between Specify and Clarify). "+
				"IMPORTANT: Before calling this tool, the AI MUST read the requirements "+
				"(use sdd_get_context stage=requirements), analyze them, and extract "+
				"declarative business rules using BRG taxonomy (Business Rules Group) "+
				"and DDD Ubiquitous Language (Eric Evans). "+
				"Pass the ACTUAL business rules (not placeholders) for each section. "+
				"Requires: sdd_generate_requirements must have been run first.",
		),
		mcp.WithString("definitions",
			mcp.Required(),
			mcp.Description("Domain terms and their precise definitions (Ubiquitous Language glossary). "+
				"Each term must have one and only one meaning across the entire project. "+
				"Example: '- **Customer**: A person who has completed at least one purchase\\n"+
				"- **Order**: A confirmed request for one or more products with a delivery address'"),
		),
		mcp.WithString("facts",
			mcp.Required(),
			mcp.Description("Relationships between domain terms that are always true. "+
				"These are structural truths about your domain model. "+
				"Example: '- A Customer has exactly one Account\\n"+
				"- An Account can have zero or more Orders\\n"+
				"- Each Order contains at least one Line Item'"),
		),
		mcp.WithString("constraints",
			mcp.Required(),
			mcp.Description("Behavioral boundaries and action assertions. "+
				"Use the format: When <condition> Then <imposition> [Otherwise <consequence>]. "+
				"(Based on Business Rules Manifesto, Ronald Ross, v2.0) "+
				"Example: '- When an Order total exceeds $500, Then manager approval is required\\n"+
				"- When a Customer has 3 failed payments, Then their account is suspended'"),
		),
		mcp.WithString("derivations",
			mcp.Description("Computed or inferred knowledge from facts and constraints. "+
				"Knowledge that is derived, not directly stated. "+
				"Example: '- A Customer is \"premium\" when their total spend exceeds $10,000 in the last 12 months\\n"+
				"- Order priority is \"high\" when the customer is premium AND delivery is express'"),
		),
		mcp.WithString("glossary",
			mcp.Description("Additional domain vocabulary and abbreviations beyond the core definitions. "+
				"Use for industry jargon, acronyms, or terms the team needs to agree on."),
		),
	)
}

// Handle processes the sdd_create_business_rules tool call.
func (t *BusinessRulesTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	definitions := req.GetString("definitions", "")
	facts := req.GetString("facts", "")
	constraints := req.GetString("constraints", "")
	derivations := req.GetString("derivations", "")
	glossary := req.GetString("glossary", "")

	// Validate required fields.
	if definitions == "" {
		return mcp.NewToolResultError("'definitions' is required — list domain terms with precise definitions (Ubiquitous Language)"), nil
	}
	if facts == "" {
		return mcp.NewToolResultError("'facts' is required — list relationships between domain terms that are always true"), nil
	}
	if constraints == "" {
		return mcp.NewToolResultError("'constraints' is required — list behavioral boundaries using When/Then/Otherwise format"), nil
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
	if err := pipeline.RequireStage(cfg, config.StageBusinessRules); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Verify requirements exist.
	reqPath := config.StagePath(projectRoot, config.StageSpecify)
	reqContent, err := readStageFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("reading requirements: %w", err)
	}
	if reqContent == "" {
		return mcp.NewToolResultError("requirements.md is empty — run sdd_generate_requirements first"), nil
	}

	pipeline.MarkInProgress(cfg)

	// Build template data.
	data := templates.BusinessRulesData{
		Name:        cfg.Name,
		Definitions: definitions,
		Facts:       facts,
		Constraints: constraints,
		Derivations: derivations,
		Glossary:    glossary,
	}

	// Render and write via shared function (ADR-001).
	content, err := RenderAndWriteBusinessRules(projectRoot, t.renderer, data, false)
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

	notifyObserver(t.bridge, cfg.Name, config.StageBusinessRules, content)

	response := fmt.Sprintf(
		"# Business Rules Documented\n\n"+
			"Saved to `docs/business-rules.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 4: Clarify (Clarity Gate)**.\n\n"+
			"The Clarity Gate now evaluates BOTH requirements AND business rules.\n"+
			"Call `sdd_clarify` (without answers) to analyze for ambiguities.\n"+
			"The pipeline cannot proceed until the clarity score reaches %d/100 (%s mode).\n\n"+
			"**Why this matters:** Business rules are the DNA of your system. "+
			"The Clarity Gate validates that every constraint is unambiguous and every "+
			"term in your Ubiquitous Language has exactly one meaning.",
		content, pipeline.ClarityThreshold(cfg.Mode), cfg.Mode,
	)

	return mcp.NewToolResultText(response), nil
}
