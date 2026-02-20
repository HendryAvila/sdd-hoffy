package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/HendryAvila/sdd-hoffy/internal/pipeline"
	"github.com/HendryAvila/sdd-hoffy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// ClarifyTool handles the sdd_clarify MCP tool.
// This is the CORE of SDD-Hoffy: the Clarity Gate that forces
// disambiguation before proceeding to implementation.
type ClarifyTool struct {
	store    config.Store
	renderer templates.Renderer
}

// NewClarifyTool creates a ClarifyTool with its dependencies.
func NewClarifyTool(store config.Store, renderer templates.Renderer) *ClarifyTool {
	return &ClarifyTool{store: store, renderer: renderer}
}

// Definition returns the MCP tool definition for registration.
func (t *ClarifyTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_clarify",
		mcp.WithDescription(
			"Run the Clarity Gate analysis on current requirements. "+
				"This is Stage 3 of the SDD pipeline — the MOST IMPORTANT stage. "+
				"It analyzes requirements for ambiguities across 8 dimensions "+
				"(target users, core functionality, data model, integrations, edge cases, "+
				"security, scale, scope boundaries). "+
				"\n\nUSAGE: "+
				"\n- Call WITHOUT 'answers' to get the analysis framework and dimensions. "+
				"The AI should then analyze the requirements, generate 3-5 specific questions, "+
				"and present them to the user. "+
				"\n- Call WITH 'answers' and 'dimension_scores' after the user answers the questions. "+
				"The AI should assess each dimension based on the requirements + answers. "+
				"\n\nThe pipeline cannot advance until the clarity score meets the threshold. "+
				"Requires: sdd_generate_requirements must have been run first.",
		),
		mcp.WithString("answers",
			mcp.Description(
				"The user's answers to clarity questions, combined with the AI's analysis. "+
					"Include both the questions asked and the answers received. "+
					"Format as markdown. Leave empty to start a new analysis round.",
			),
		),
		mcp.WithString("dimension_scores",
			mcp.Description(
				"AI-assessed scores for each clarity dimension after analyzing requirements + answers. "+
					"Comma-separated list of dimension_name:score pairs (score 0-100). "+
					"ALL 8 dimensions should be scored: "+
					"target_users, core_functionality, data_model, integrations, "+
					"edge_cases, security, scale_performance, scope_boundaries. "+
					"Example: 'target_users:80,core_functionality:90,data_model:60,integrations:50,"+
					"edge_cases:55,security:70,scale_performance:60,scope_boundaries:85'",
			),
		),
	)
}

// Handle processes the sdd_clarify tool call.
func (t *ClarifyTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	answers := req.GetString("answers", "")
	dimensionScores := req.GetString("dimension_scores", "")

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	cfg, err := t.store.Load(projectRoot)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate pipeline stage.
	if err := pipeline.RequireStage(cfg, config.StageClarify); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Read requirements for analysis.
	reqPath := config.StagePath(projectRoot, config.StageSpecify)
	requirements, err := readStageFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("reading requirements: %w", err)
	}
	if requirements == "" {
		return mcp.NewToolResultError("requirements.md is empty — run sdd_generate_requirements first"), nil
	}

	pipeline.MarkInProgress(cfg)

	threshold := pipeline.ClarityThreshold(cfg.Mode)

	// Branch: generating questions vs processing answers.
	if answers == "" {
		return t.generateQuestions(cfg, requirements, projectRoot, threshold)
	}

	return t.processAnswers(cfg, requirements, answers, dimensionScores, projectRoot, threshold)
}

// generateQuestions analyzes requirements and produces the clarity analysis framework.
func (t *ClarifyTool) generateQuestions(
	cfg *config.ProjectConfig,
	requirements string,
	projectRoot string,
	threshold int,
) (*mcp.CallToolResult, error) {
	dimensions := pipeline.DefaultDimensions()

	var sb strings.Builder
	sb.WriteString("# Clarity Gate Analysis\n\n")
	sb.WriteString(fmt.Sprintf("**Mode:** %s | **Threshold:** %d/100\n\n", cfg.Mode, threshold))
	sb.WriteString("## Requirements Under Analysis\n\n")
	sb.WriteString(requirements)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString("## Clarity Dimensions\n\n")
	sb.WriteString("Analyze the requirements above across these 8 dimensions. ")
	sb.WriteString("For each dimension with gaps, generate 1-2 specific, answerable questions.\n\n")

	for _, d := range dimensions {
		sb.WriteString(fmt.Sprintf("### %s (weight: %d/10)\n", d.Name, d.Weight))
		sb.WriteString(fmt.Sprintf("%s\n\n", d.Description))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("## What To Do Next\n\n")
	sb.WriteString("1. Analyze the requirements for gaps in each dimension\n")
	sb.WriteString("2. Generate 3-5 total questions targeting the WEAKEST dimensions\n")
	sb.WriteString("3. Present the questions to the user and collect their answers\n")
	sb.WriteString("4. After receiving answers, call `sdd_clarify` again with:\n")
	sb.WriteString("   - `answers`: the Q&A from this round (as markdown)\n")
	sb.WriteString("   - `dimension_scores`: your assessment of each dimension (0-100)\n")

	// Read existing clarifications to show history.
	clarifyPath := config.StagePath(projectRoot, config.StageClarify)
	existing, _ := readStageFile(clarifyPath)
	if existing != "" {
		sb.WriteString("\n---\n\n## Previous Clarification Rounds\n\n")
		sb.WriteString(existing)
		sb.WriteString("\n\n_Build on previous rounds. Don't re-ask answered questions._\n")
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// processAnswers records answers, updates clarity score, and checks the gate.
func (t *ClarifyTool) processAnswers(
	cfg *config.ProjectConfig,
	requirements, answers, dimensionScores string,
	projectRoot string,
	threshold int,
) (*mcp.CallToolResult, error) {
	// Parse dimension scores if provided.
	dimensions := pipeline.DefaultDimensions()
	if dimensionScores != "" {
		parseDimensionScores(dimensionScores, dimensions)
	}

	// Calculate new clarity score.
	newScore := pipeline.CalculateScore(dimensions)
	cfg.ClarityScore = newScore

	// Read existing clarifications and append this round.
	clarifyPath := config.StagePath(projectRoot, config.StageClarify)
	existing, _ := readStageFile(clarifyPath)

	iteration := cfg.StageStatus[config.StageClarify].Iterations
	roundContent := fmt.Sprintf(
		"\n### Round %d\n\n%s\n\n**Clarity Score after this round:** %d/100\n",
		iteration, answers, newScore,
	)

	updatedContent := existing + roundContent
	if err := writeStageFile(clarifyPath, updatedContent); err != nil {
		return nil, fmt.Errorf("writing clarifications: %w", err)
	}

	// Render the full clarifications document.
	status := "IN PROGRESS"
	if newScore >= threshold {
		status = "PASSED"
	}

	fullDoc, err := t.renderer.Render(templates.Clarifications, templates.ClarificationsData{
		Name:         cfg.Name,
		ClarityScore: newScore,
		Mode:         string(cfg.Mode),
		Threshold:    threshold,
		Status:       status,
		Rounds:       updatedContent,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering clarifications: %w", err)
	}

	if err := writeStageFile(clarifyPath, fullDoc); err != nil {
		return nil, fmt.Errorf("writing clarifications: %w", err)
	}

	// Check if we passed the gate.
	var response string
	if newScore >= threshold {
		// Gate passed! Advance pipeline.
		if err := pipeline.Advance(cfg); err != nil {
			return nil, fmt.Errorf("advancing pipeline: %w", err)
		}

		response = fmt.Sprintf(
			"# Clarity Gate PASSED\n\n"+
				"**Score:** %d/100 (threshold: %d)\n\n"+
				"Your requirements are now clear enough to proceed.\n\n"+
				"## Next Step\n\n"+
				"Pipeline advanced to **Stage 4: Design**.\n\n"+
				"The AI can now create a technical design based on these well-defined requirements. "+
				"Use `sdd_get_context` to review all artifacts before proceeding.",
			newScore, threshold,
		)
	} else {
		// Need more clarification.
		uncovered := pipeline.UncoveredDimensions(dimensions)
		var uncoveredNames []string
		for _, d := range uncovered {
			uncoveredNames = append(uncoveredNames, d.Name)
		}

		response = fmt.Sprintf(
			"# Clarity Gate: More Clarification Needed\n\n"+
				"**Score:** %d/100 (need %d to pass)\n\n"+
				"## Weak Areas\n\n"+
				"These dimensions still need attention: %s\n\n"+
				"## What to Do\n\n"+
				"Call `sdd_clarify` again (without answers) to get the next round of questions "+
				"targeting these weak areas.",
			newScore, threshold, strings.Join(uncoveredNames, ", "),
		)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	return mcp.NewToolResultText(response), nil
}

// parseDimensionScores parses "name:score,name:score" format into dimensions.
func parseDimensionScores(input string, dimensions []pipeline.ClarityDimension) {
	pairs := strings.Split(input, ",")
	scoreMap := make(map[string]int)

	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		var score int
		if _, err := fmt.Sscanf(parts[1], "%d", &score); err == nil {
			if score < 0 {
				score = 0
			}
			if score > 100 {
				score = 100
			}
			scoreMap[name] = score
		}
	}

	for i := range dimensions {
		if score, ok := scoreMap[dimensions[i].Name]; ok {
			dimensions[i].Score = score
			dimensions[i].Covered = score > 30 // Consider "covered" if score > 30
		}
	}
}
