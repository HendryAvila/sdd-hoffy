// Package tools — see helpers.go for package doc.
//
// bootstrap.go implements the sdd_bootstrap MCP tool.
// It writes SDD artifacts for projects that bypassed the greenfield pipeline.
// No pipeline state machine interaction, no stage guards, no hoofy.json required.
// Uses the shared rendering functions from artifacts.go (ADR-001).
//
// ADR-002: Separate tool from sdd_reverse_engineer (scanner = read-only,
// bootstrap = write-only).
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// BootstrapTool handles the sdd_bootstrap MCP tool.
// It writes missing SDD artifacts without pipeline guards.
type BootstrapTool struct {
	renderer templates.Renderer
}

// NewBootstrapTool creates a BootstrapTool with its dependencies.
func NewBootstrapTool(renderer templates.Renderer) *BootstrapTool {
	return &BootstrapTool{renderer: renderer}
}

// Definition returns the MCP tool definition for registration.
func (t *BootstrapTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_bootstrap",
		mcp.WithDescription(
			"Write missing SDD artifacts for projects that bypassed the greenfield pipeline. "+
				"Call this AFTER `sdd_reverse_engineer` to save the AI-generated artifacts. "+
				"Only writes artifacts that don't already exist — existing ones are skipped. "+
				"Does NOT require hoofy.json or an active pipeline. "+
				"At least one artifact group must have content.",
		),
		// --- Requirements artifact ---
		mcp.WithString("requirements_must_have",
			mcp.Description("Non-negotiable requirements (FR-XXX IDs). Part of the requirements artifact."),
		),
		mcp.WithString("requirements_should_have",
			mcp.Description("Important but non-blocking requirements. Part of the requirements artifact."),
		),
		mcp.WithString("requirements_non_functional",
			mcp.Description("Performance, security, usability constraints (NFR-XXX IDs). Part of the requirements artifact."),
		),
		mcp.WithString("requirements_could_have",
			mcp.Description("Nice-to-have features for future versions."),
		),
		mcp.WithString("requirements_wont_have",
			mcp.Description("Explicitly excluded features."),
		),
		mcp.WithString("requirements_constraints",
			mcp.Description("Technical, business, or regulatory limitations."),
		),
		mcp.WithString("requirements_assumptions",
			mcp.Description("Assumptions that, if changed, would change requirements."),
		),
		mcp.WithString("requirements_dependencies",
			mcp.Description("External systems, APIs, or services depended upon."),
		),
		// --- Business rules artifact ---
		mcp.WithString("business_rules_definitions",
			mcp.Description("Domain terms with precise definitions (Ubiquitous Language)."),
		),
		mcp.WithString("business_rules_facts",
			mcp.Description("Relationships between domain terms that are always true."),
		),
		mcp.WithString("business_rules_constraints",
			mcp.Description("Behavioral boundaries: When <condition> Then <imposition>."),
		),
		mcp.WithString("business_rules_derivations",
			mcp.Description("Computed or inferred knowledge from facts and constraints."),
		),
		mcp.WithString("business_rules_glossary",
			mcp.Description("Additional domain vocabulary and abbreviations."),
		),
		// --- Design artifact ---
		mcp.WithString("design_architecture",
			mcp.Description("High-level architecture overview (pattern, principles, interactions)."),
		),
		mcp.WithString("design_tech_stack",
			mcp.Description("Technology choices with rationale."),
		),
		mcp.WithString("design_components",
			mcp.Description("Component breakdown with responsibilities and boundaries."),
		),
		mcp.WithString("design_data_model",
			mcp.Description("Database schema, entity relationships, constraints."),
		),
		mcp.WithString("design_api_contracts",
			mcp.Description("API endpoint definitions, schemas, error codes."),
		),
		mcp.WithString("design_infrastructure",
			mcp.Description("Deployment strategy, hosting, CI/CD."),
		),
		mcp.WithString("design_security",
			mcp.Description("Security measures, auth strategy, data protection."),
		),
		mcp.WithString("design_quality_analysis",
			mcp.Description("Structural quality analysis: SOLID compliance, code smell detection "+
				"(Shotgun Surgery, Feature Envy, God Class, Divergent Change, Inappropriate Intimacy), "+
				"coupling & cohesion analysis, and mitigations."),
		),
		// --- Project name ---
		mcp.WithString("project_name",
			mcp.Description("Project name for the artifact headers. Defaults to the directory name."),
		),
	)
}

// Handle processes the sdd_bootstrap tool call.
func (t *BootstrapTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	// Resolve project name.
	projectName := req.GetString("project_name", "")
	if projectName == "" {
		projectName = filepath.Base(projectRoot)
	}

	// Ensure docs/ directory exists.
	docsDir := config.DocsPath(projectRoot)
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating docs directory: %w", err)
	}

	// Check which artifacts exist.
	hasReqs := ArtifactExists(projectRoot, config.StageSpecify)
	hasRules := ArtifactExists(projectRoot, config.StageBusinessRules)
	hasDesign := ArtifactExists(projectRoot, config.StageDesign)

	// Parse all parameters.
	// Requirements group.
	reqMustHave := req.GetString("requirements_must_have", "")
	reqShouldHave := req.GetString("requirements_should_have", "")
	reqNonFunctional := req.GetString("requirements_non_functional", "")
	reqCouldHave := req.GetString("requirements_could_have", "")
	reqWontHave := req.GetString("requirements_wont_have", "")
	reqConstraints := req.GetString("requirements_constraints", "")
	reqAssumptions := req.GetString("requirements_assumptions", "")
	reqDependencies := req.GetString("requirements_dependencies", "")

	// Business rules group.
	brDefinitions := req.GetString("business_rules_definitions", "")
	brFacts := req.GetString("business_rules_facts", "")
	brConstraints := req.GetString("business_rules_constraints", "")
	brDerivations := req.GetString("business_rules_derivations", "")
	brGlossary := req.GetString("business_rules_glossary", "")

	// Design group.
	desArch := req.GetString("design_architecture", "")
	desTechStack := req.GetString("design_tech_stack", "")
	desComponents := req.GetString("design_components", "")
	desDataModel := req.GetString("design_data_model", "")
	desAPI := req.GetString("design_api_contracts", "")
	desInfra := req.GetString("design_infrastructure", "")
	desSecurity := req.GetString("design_security", "")
	desQualityAnalysis := req.GetString("design_quality_analysis", "")

	// Check if at least one artifact group has content.
	hasReqContent := reqMustHave != "" || reqShouldHave != "" || reqNonFunctional != ""
	hasRulesContent := brDefinitions != "" || brFacts != "" || brConstraints != ""
	hasDesignContent := desArch != "" || desTechStack != "" || desComponents != "" || desDataModel != ""

	if !hasReqContent && !hasRulesContent && !hasDesignContent {
		return mcp.NewToolResultError(
			"No artifact content provided. At least one artifact group must have content.\n\n" +
				"Provide parameters for one or more of:\n" +
				"- **Requirements**: requirements_must_have, requirements_should_have, requirements_non_functional\n" +
				"- **Business rules**: business_rules_definitions, business_rules_facts, business_rules_constraints\n" +
				"- **Design**: design_architecture, design_tech_stack, design_components, design_data_model",
		), nil
	}

	var written []string
	var skipped []string

	// --- Write requirements ---
	if hasReqContent && !hasReqs {
		// Fill defaults for empty optional fields.
		if reqCouldHave == "" {
			reqCouldHave = "_None defined for this version._"
		}
		if reqWontHave == "" {
			reqWontHave = "_None defined for this version._"
		}
		if reqConstraints == "" {
			reqConstraints = "_None identified._"
		}
		if reqAssumptions == "" {
			reqAssumptions = "_None identified._"
		}
		if reqDependencies == "" {
			reqDependencies = "_None identified._"
		}
		// Fill required fields with placeholder if only some are provided.
		if reqMustHave == "" {
			reqMustHave = "_To be extracted from project analysis._"
		}
		if reqShouldHave == "" {
			reqShouldHave = "_To be extracted from project analysis._"
		}
		if reqNonFunctional == "" {
			reqNonFunctional = "_To be extracted from project analysis._"
		}

		data := templates.RequirementsData{
			Name:          projectName,
			MustHave:      reqMustHave,
			ShouldHave:    reqShouldHave,
			CouldHave:     reqCouldHave,
			WontHave:      reqWontHave,
			NonFunctional: reqNonFunctional,
			Constraints:   reqConstraints,
			Assumptions:   reqAssumptions,
			Dependencies:  reqDependencies,
		}

		if _, err := RenderAndWriteRequirements(projectRoot, t.renderer, data, true); err != nil {
			return nil, fmt.Errorf("writing requirements: %w", err)
		}
		written = append(written, "requirements.md")
	} else if hasReqs {
		skipped = append(skipped, "requirements.md (already exists)")
	}

	// --- Write business rules ---
	if hasRulesContent && !hasRules {
		if brDefinitions == "" {
			brDefinitions = "_To be extracted from project analysis._"
		}
		if brFacts == "" {
			brFacts = "_To be extracted from project analysis._"
		}
		if brConstraints == "" {
			brConstraints = "_To be extracted from project analysis._"
		}

		data := templates.BusinessRulesData{
			Name:        projectName,
			Definitions: brDefinitions,
			Facts:       brFacts,
			Constraints: brConstraints,
			Derivations: brDerivations,
			Glossary:    brGlossary,
		}

		if _, err := RenderAndWriteBusinessRules(projectRoot, t.renderer, data, true); err != nil {
			return nil, fmt.Errorf("writing business rules: %w", err)
		}
		written = append(written, "business-rules.md")
	} else if hasRules {
		skipped = append(skipped, "business-rules.md (already exists)")
	}

	// --- Write design ---
	if hasDesignContent && !hasDesign {
		if desArch == "" {
			desArch = "_To be extracted from project analysis._"
		}
		if desTechStack == "" {
			desTechStack = "_To be extracted from project analysis._"
		}
		if desComponents == "" {
			desComponents = "_To be extracted from project analysis._"
		}
		if desDataModel == "" {
			desDataModel = "_To be extracted from project analysis._"
		}
		if desAPI == "" {
			desAPI = "_No API contracts defined — this project does not expose an API._"
		}
		if desInfra == "" {
			desInfra = "_Not yet defined._"
		}
		if desSecurity == "" {
			desSecurity = "_Not yet defined._"
		}
		if desQualityAnalysis == "" {
			desQualityAnalysis = "_No structural quality analysis provided._"
		}

		data := templates.DesignData{
			Name:                 projectName,
			ArchitectureOverview: desArch,
			TechStack:            desTechStack,
			Components:           desComponents,
			APIContracts:         desAPI,
			DataModel:            desDataModel,
			Infrastructure:       desInfra,
			Security:             desSecurity,
			QualityAnalysis:      desQualityAnalysis,
		}

		if _, err := RenderAndWriteDesign(projectRoot, t.renderer, data, true); err != nil {
			return nil, fmt.Errorf("writing design: %w", err)
		}
		written = append(written, "design.md")
	} else if hasDesign {
		skipped = append(skipped, "design.md (already exists)")
	}

	// Build response.
	var response strings.Builder
	response.WriteString("# SDD Bootstrap Complete\n\n")

	if len(written) > 0 {
		response.WriteString("## Written\n\n")
		for _, w := range written {
			fmt.Fprintf(&response, "- ✅ `docs/%s`\n", w)
		}
		response.WriteString("\n")
	}

	if len(skipped) > 0 {
		response.WriteString("## Skipped\n\n")
		for _, s := range skipped {
			fmt.Fprintf(&response, "- ⏭️ `docs/%s`\n", s)
		}
		response.WriteString("\n")
	}

	if len(written) == 0 && len(skipped) > 0 {
		response.WriteString("All artifacts already exist. Nothing to write.\n\n")
	}

	response.WriteString("---\n\n")
	response.WriteString("## Next Steps\n\n")
	response.WriteString("1. **Review** the auto-generated artifacts in `docs/` — they may need refinement\n")
	response.WriteString("2. **Run `sdd_change`** to start making changes with full context awareness\n")
	response.WriteString("3. The `context-check` stage will now have architecture and requirements context\n")

	return mcp.NewToolResultText(response.String()), nil
}
