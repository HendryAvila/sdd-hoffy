package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/memory"
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
				"Specific stage artifact to read: 'principles', 'charter', 'requirements', 'clarifications', "+
					"'design', 'tasks'. Leave empty to get an overview of all stages.",
			),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail for the overview: "+
					"'summary' (default — stage names + status only — minimal tokens), "+
					"'standard' (pipeline table, artifact sizes, next steps), "+
					"'full' (include complete artifact content inline). "+
					"Defaults to 'summary'. Ignored when 'stage' is set.",
			),
			mcp.Enum("summary", "standard", "full"),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates the response to stay within budget. 0 or omit for no cap."),
		),
	)
}

// Handle processes the sdd_get_context tool call.
func (t *ContextTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stageFilter := req.GetString("stage", "")
	detailLevel := req.GetString("detail_level", "summary")
	maxTokens := intArgTools(req, "max_tokens", 0)

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	cfg, err := t.store.Load(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// If a specific stage was requested, return its content (detail_level ignored).
	if stageFilter != "" {
		result, stageErr := t.readStageContent(cfg, projectRoot, config.Stage(stageFilter))
		if stageErr != nil {
			return nil, stageErr
		}
		return applyBudgetAndFooter(result, maxTokens), nil
	}

	// Route to the appropriate overview builder based on detail level.
	var result *mcp.CallToolResult
	switch detailLevel {
	case "summary":
		result = t.buildSummaryOverview(cfg)
	case "full":
		result, err = t.buildFullOverview(cfg, projectRoot)
		if err != nil {
			return nil, err
		}
	default:
		result, err = t.buildOverview(cfg, projectRoot)
		if err != nil {
			return nil, err
		}
	}

	return applyBudgetAndFooter(result, maxTokens), nil
}

// intArgTools extracts an integer argument from a tool request (JSON numbers are float64).
func intArgTools(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

// applyBudgetAndFooter applies post-hoc budget truncation and appends a token footer.
// Error results are returned as-is (no budget or footer applied).
func applyBudgetAndFooter(result *mcp.CallToolResult, maxTokens int) *mcp.CallToolResult {
	if result == nil || result.IsError {
		return result
	}

	text := getTextContent(result)
	if text == "" {
		return result
	}

	if maxTokens > 0 && memory.EstimateTokens(text) > maxTokens {
		charBudget := maxTokens * 4
		if charBudget < len(text) {
			text = text[:charBudget]
			if lastNL := strings.LastIndex(text, "\n"); lastNL > charBudget/2 {
				text = text[:lastNL]
			}
			text += "\n[...truncated by token budget]"
		}
	}

	text += memory.TokenFooter(memory.EstimateTokens(text))
	return mcp.NewToolResultText(text)
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
// This is the "standard" detail level — the default behavior.
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
		config.StagePrinciples,
		config.StageCharter,
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
		fmt.Fprintf(&sb, "- **%s** (`docs/%s`): %s\n",
			meta.Name, config.StageFilename(stage), exists)
	}

	// Next steps.
	sb.WriteString("\n## Next Steps\n\n")
	sb.WriteString(nextStepGuidance(cfg))

	return mcp.NewToolResultText(sb.String()), nil
}

// buildSummaryOverview creates a minimal overview with stage names and status only.
// Designed for minimal token usage — progressive disclosure pattern.
func (t *ContextTool) buildSummaryOverview(cfg *config.ProjectConfig) *mcp.CallToolResult {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# %s [%s mode]\n\n", cfg.Name, cfg.Mode)

	if cfg.CurrentStage == config.StageClarify {
		fmt.Fprintf(&sb, "Clarity: %d/%d\n\n",
			cfg.ClarityScore, clarityThresholdForMode(cfg.Mode))
	}

	for _, stage := range config.StageOrder {
		meta := config.Stages[stage]
		status := cfg.StageStatus[stage]
		indicator := statusIndicator(status.Status)
		current := ""
		if stage == cfg.CurrentStage {
			current = " ←"
		}
		fmt.Fprintf(&sb, "%s %s%s\n", indicator, meta.Name, current)
	}

	return mcp.NewToolResultText(sb.String())
}

// buildFullOverview creates a comprehensive overview including all artifact content inline.
// This provides all project context in a single call — useful for full-context loading.
func (t *ContextTool) buildFullOverview(cfg *config.ProjectConfig, projectRoot string) (*mcp.CallToolResult, error) {
	// Start with the standard overview.
	standardResult, err := t.buildOverview(cfg, projectRoot)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(getTextContent(standardResult))

	// Append full artifact content for each completed stage.
	artifactStages := []config.Stage{
		config.StagePrinciples,
		config.StageCharter,
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
		if content == "" {
			continue
		}
		meta := config.Stages[stage]
		fmt.Fprintf(&sb, "\n---\n\n## %s Content\n\n%s\n", meta.Name, content)
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// getTextContent extracts the text from an MCP tool result.
func getTextContent(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if textContent, ok := c.(mcp.TextContent); ok {
			return textContent.Text
		}
	}
	return ""
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
	case config.StagePrinciples:
		return "Use `sdd_create_principles` to define your project's golden invariants, coding standards, and domain truths."
	case config.StageCharter:
		return "Use `sdd_create_charter` with your project idea to create a structured charter."
	case config.StageSpecify:
		return "Use `sdd_generate_requirements` to extract formal requirements from the charter."
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
