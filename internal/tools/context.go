package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// ContextTool handles the sdd_get_context MCP tool.
// It provides a read-only view of the current SDD project state.
type ContextTool struct {
	store config.Store
}

// NewContextTool creates a ContextTool with its dependencies.
func NewContextTool(store config.Store) *ContextTool {
	return &ContextTool{store: store}
}

// Definition returns the MCP tool definition for registration.
func (t *ContextTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_get_context",
		mcp.WithDescription(
			"Read the current state of the SDD project. "+
				"Returns pipeline status, current stage, clarity score, and optionally "+
				"the content of specific stage artifacts. "+
				"Use this to understand where the project is in the SDD pipeline.",
		),
		mcp.WithString("stage",
			mcp.Description(
				"Specific stage artifact to read: 'proposal', 'requirements', 'clarifications', "+
					"'design', 'tasks'. Leave empty to get an overview of all stages.",
			),
		),
	)
}

// Handle processes the sdd_get_context tool call.
func (t *ContextTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stageFilter := req.GetString("stage", "")

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	cfg, err := t.store.Load(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// If a specific stage was requested, return its content.
	if stageFilter != "" {
		return t.readStageContent(cfg, projectRoot, config.Stage(stageFilter))
	}

	// Otherwise, return the full project overview.
	return t.buildOverview(cfg, projectRoot)
}

// readStageContent returns the markdown content for a specific stage.
func (t *ContextTool) readStageContent(cfg *config.ProjectConfig, projectRoot string, stage config.Stage) (*mcp.CallToolResult, error) {
	path := config.StagePath(projectRoot, stage)
	if path == "" {
		return mcp.NewToolResultError(fmt.Sprintf("unknown stage: %s", stage)), nil
	}

	content, err := readStageFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading stage %s: %w", stage, err)
	}

	if content == "" {
		meta := config.Stages[stage]
		return mcp.NewToolResultText(fmt.Sprintf(
			"# Stage: %s\n\n**Status:** Not yet completed\n\n_%s_",
			meta.Name, meta.Description,
		)), nil
	}

	return mcp.NewToolResultText(content), nil
}

// buildOverview creates a summary of the entire SDD project state.
func (t *ContextTool) buildOverview(cfg *config.ProjectConfig, projectRoot string) (*mcp.CallToolResult, error) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# SDD Project: %s\n\n", cfg.Name)
	fmt.Fprintf(&sb, "**Description:** %s\n", cfg.Description)
	fmt.Fprintf(&sb, "**Mode:** %s\n", cfg.Mode)
	fmt.Fprintf(&sb, "**Created:** %s\n", cfg.CreatedAt)
	fmt.Fprintf(&sb, "**Last Updated:** %s\n\n", cfg.UpdatedAt)

	// Pipeline status.
	currentMeta := config.Stages[cfg.CurrentStage]
	fmt.Fprintf(&sb, "## Current Stage: %s (%s)\n\n", currentMeta.Name, cfg.CurrentStage)
	fmt.Fprintf(&sb, "_%s_\n\n", currentMeta.Description)

	if cfg.CurrentStage == config.StageClarify {
		fmt.Fprintf(&sb, "**Clarity Score:** %d/100 (need %d for %s mode)\n\n",
			cfg.ClarityScore, clarityThresholdForMode(cfg.Mode), cfg.Mode)
	}

	// Stage overview table.
	sb.WriteString("## Pipeline Progress\n\n")
	sb.WriteString("| Stage | Status | Iterations |\n")
	sb.WriteString("|-------|--------|------------|\n")

	for _, stage := range config.StageOrder {
		meta := config.Stages[stage]
		status := cfg.StageStatus[stage]
		indicator := statusIndicator(status.Status)
		current := ""
		if stage == cfg.CurrentStage {
			current = " **← current**"
		}
		fmt.Fprintf(&sb, "| %s %s | %s%s | %d |\n",
			indicator, meta.Name, status.Status, current, status.Iterations)
	}

	// Artifacts summary.
	sb.WriteString("\n## Artifacts\n\n")
	artifactStages := []config.Stage{
		config.StagePropose,
		config.StageSpecify,
		config.StageClarify,
		config.StageDesign,
		config.StageTasks,
		config.StageValidate,
	}
	for _, stage := range artifactStages {
		path := config.StagePath(projectRoot, stage)
		if path == "" {
			continue
		}
		content, _ := readStageFile(path)
		exists := "not created"
		if content != "" {
			lines := strings.Count(content, "\n")
			exists = fmt.Sprintf("%d lines", lines)
		}
		meta := config.Stages[stage]
		fmt.Fprintf(&sb, "- **%s** (`sdd/%s`): %s\n",
			meta.Name, config.StageFilename(stage), exists)
	}

	// Next steps.
	sb.WriteString("\n## Next Steps\n\n")
	sb.WriteString(nextStepGuidance(cfg))

	return mcp.NewToolResultText(sb.String()), nil
}

// statusIndicator returns an emoji for the given status.
func statusIndicator(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "in_progress":
		return "🔄"
	case "skipped":
		return "⏭️"
	default:
		return "⬜"
	}
}

// nextStepGuidance returns mode-appropriate guidance for the current stage.
func nextStepGuidance(cfg *config.ProjectConfig) string {
	switch cfg.CurrentStage {
	case config.StagePropose:
		return "Use `sdd_create_proposal` with your project idea to create a structured proposal."
	case config.StageSpecify:
		return "Use `sdd_generate_requirements` to extract formal requirements from the proposal."
	case config.StageClarify:
		return fmt.Sprintf(
			"Use `sdd_clarify` to run the Clarity Gate. Current score: %d/%d needed.",
			cfg.ClarityScore, clarityThresholdForMode(cfg.Mode),
		)
	case config.StageDesign:
		return "Use `sdd_create_design` to create the technical architecture document. " +
			"Read all previous artifacts first (use `sdd_get_context`), then design the system " +
			"addressing ALL requirements. Include tech stack, components, data model, and key design decisions."
	case config.StageTasks:
		return "Use `sdd_create_tasks` to break the design into atomic implementation tasks. " +
			"Read the design document first (use `sdd_get_context stage=design`). " +
			"Each task should have a unique ID, clear scope, requirements covered, and acceptance criteria."
	case config.StageValidate:
		return "Use `sdd_validate` to run a cross-artifact consistency check. " +
			"Read ALL artifacts and verify: requirement coverage, component coverage, " +
			"consistency between documents, and identify any gaps or risks."
	default:
		return "Use `sdd_init_project` to start a new SDD project."
	}
}

// clarityThresholdForMode returns the clarity threshold. This is a thin
// wrapper to avoid importing the pipeline package (keeps ContextTool
// lightweight — it only needs config).
func clarityThresholdForMode(mode config.Mode) int {
	if mode == config.ModeExpert {
		return 50
	}
	return 70
}
