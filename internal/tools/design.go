package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/HendryAvila/sdd-hoffy/internal/pipeline"
	"github.com/HendryAvila/sdd-hoffy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// DesignTool handles the sdd_create_design MCP tool.
// It saves a technical design document with content provided by the AI.
type DesignTool struct {
	store    config.Store
	renderer templates.Renderer
}

// NewDesignTool creates a DesignTool with its dependencies.
func NewDesignTool(store config.Store, renderer templates.Renderer) *DesignTool {
	return &DesignTool{store: store, renderer: renderer}
}

// Definition returns the MCP tool definition for registration.
func (t *DesignTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_create_design",
		mcp.WithDescription(
			"Save a technical design document for the SDD project. "+
				"This is Stage 4 of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST read the requirements and clarifications "+
				"(use sdd_get_context), analyze them, and generate a technical architecture that addresses "+
				"ALL requirements. Pass the ACTUAL design content (not placeholders) for each section. "+
				"Requires: Clarity Gate must have been passed first.",
		),
		mcp.WithString("architecture_overview",
			mcp.Required(),
			mcp.Description("High-level architecture description. Include the architectural pattern "+
				"(monolith, microservices, serverless, etc.), key design principles, and how components interact. "+
				"Example: 'A modular monolith using Clean Architecture with 3 layers: presentation (REST API), "+
				"application (use cases), and domain (business logic). Communication is synchronous within the monolith "+
				"with an event bus for async operations like email notifications.'"),
		),
		mcp.WithString("tech_stack",
			mcp.Required(),
			mcp.Description("Technology choices with rationale for each. Use markdown list format. "+
				"Example: '- **Runtime**: Node.js 20 LTS — mature ecosystem, team expertise\\n"+
				"- **Framework**: Express.js — lightweight, flexible, well-documented\\n"+
				"- **Database**: PostgreSQL 16 — relational data, ACID compliance needed for financial records\\n"+
				"- **ORM**: Prisma — type-safe queries, excellent DX'"),
		),
		mcp.WithString("components",
			mcp.Required(),
			mcp.Description("Component breakdown with responsibilities and boundaries. Each component should "+
				"map to one or more requirements (FR-XXX). Use markdown format. "+
				"Example: '### AuthModule\\n- **Responsibility**: User registration, login, session management\\n"+
				"- **Covers**: FR-001, FR-002\\n- **Exposes**: POST /auth/register, POST /auth/login\\n"+
				"- **Depends on**: DatabaseModule, EmailModule'"),
		),
		mcp.WithString("api_contracts",
			mcp.Description("API endpoint definitions, request/response schemas, error codes. "+
				"Include authentication requirements. Use markdown format with code blocks for schemas. "+
				"Leave empty if the project has no API (e.g., CLI tool, library)."),
		),
		mcp.WithString("data_model",
			mcp.Required(),
			mcp.Description("Database schema, entity relationships, key constraints. "+
				"Use markdown tables or descriptions. Include indexes for performance-critical queries. "+
				"Example: '### User\\n| Field | Type | Constraints |\\n|-------|------|-------------|\\n"+
				"| id | UUID | PK |\\n| email | VARCHAR(255) | UNIQUE, NOT NULL |'"),
		),
		mcp.WithString("infrastructure",
			mcp.Description("Deployment strategy, hosting, CI/CD, environments. "+
				"Example: '- **Hosting**: Vercel (frontend) + Railway (API + DB)\\n"+
				"- **CI/CD**: GitHub Actions — lint, test, deploy on merge to main\\n"+
				"- **Environments**: dev (auto-deploy on PR), staging (merge to main), prod (manual promote)'"),
		),
		mcp.WithString("security",
			mcp.Description("Security measures, authentication strategy, data protection. "+
				"Should address NFR security requirements. "+
				"Example: '- JWT with refresh tokens (15min access, 7d refresh)\\n"+
				"- bcrypt for password hashing (cost factor 12)\\n"+
				"- Rate limiting: 100 req/min per IP'"),
		),
		mcp.WithString("design_decisions",
			mcp.Description("Key architectural decisions and their rationale (ADRs). "+
				"Include alternatives considered and why they were rejected. Use markdown format. "+
				"Example: '### ADR-001: PostgreSQL over MongoDB\\n"+
				"**Context**: Need to store relational data with transactions\\n"+
				"**Decision**: PostgreSQL\\n"+
				"**Rationale**: ACID compliance required for financial records; "+
				"data is inherently relational\\n"+
				"**Alternatives rejected**: MongoDB (no native joins, eventual consistency)'"),
		),
	)
}

// Handle processes the sdd_create_design tool call.
func (t *DesignTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	archOverview := req.GetString("architecture_overview", "")
	techStack := req.GetString("tech_stack", "")
	components := req.GetString("components", "")
	apiContracts := req.GetString("api_contracts", "")
	dataModel := req.GetString("data_model", "")
	infrastructure := req.GetString("infrastructure", "")
	security := req.GetString("security", "")
	designDecisions := req.GetString("design_decisions", "")

	// Validate required fields.
	if archOverview == "" {
		return mcp.NewToolResultError("'architecture_overview' is required — describe the system architecture"), nil
	}
	if techStack == "" {
		return mcp.NewToolResultError("'tech_stack' is required — list technology choices with rationale"), nil
	}
	if components == "" {
		return mcp.NewToolResultError("'components' is required — break down the system into components with responsibilities"), nil
	}
	if dataModel == "" {
		return mcp.NewToolResultError("'data_model' is required — define the data schema and relationships"), nil
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
	if err := pipeline.RequireStage(cfg, config.StageDesign); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Verify requirements and clarifications exist.
	reqPath := config.StagePath(projectRoot, config.StageSpecify)
	requirements, err := readStageFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("reading requirements: %w", err)
	}
	if requirements == "" {
		return mcp.NewToolResultError("requirements.md is empty — the specify stage must be completed first"), nil
	}

	pipeline.MarkInProgress(cfg)

	// Fill optional fields with defaults.
	if apiContracts == "" {
		apiContracts = "_No API contracts defined — this project does not expose an API._"
	}
	if infrastructure == "" {
		infrastructure = "_Not yet defined._"
	}
	if security == "" {
		security = "_Not yet defined._"
	}
	if designDecisions == "" {
		designDecisions = "_No explicit ADRs recorded._"
	}

	// Build design document with REAL content from the AI.
	data := templates.DesignData{
		Name:                 cfg.Name,
		ArchitectureOverview: archOverview,
		TechStack:            techStack,
		Components:           components,
		APIContracts:         apiContracts,
		DataModel:            dataModel,
		Infrastructure:       infrastructure,
		Security:             security,
		DesignDecisions:      designDecisions,
	}

	content, err := t.renderer.Render(templates.Design, data)
	if err != nil {
		return nil, fmt.Errorf("rendering design: %w", err)
	}

	// Write the design file.
	designPath := config.StagePath(projectRoot, config.StageDesign)
	if err := writeStageFile(designPath, content); err != nil {
		return nil, fmt.Errorf("writing design: %w", err)
	}

	// Advance pipeline to next stage.
	if err := pipeline.Advance(cfg); err != nil {
		return nil, fmt.Errorf("advancing pipeline: %w", err)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	response := fmt.Sprintf(
		"# Technical Design Created\n\n"+
			"Saved to `sdd/design.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 5: Tasks**.\n\n"+
			"Now break this design into atomic, AI-ready implementation tasks. "+
			"Each task should be small enough for a single commit, include acceptance criteria, "+
			"and reference the requirements (FR-XXX) and components it implements.\n\n"+
			"Call `sdd_create_tasks` with the task breakdown.",
		content,
	)

	return mcp.NewToolResultText(response), nil
}
