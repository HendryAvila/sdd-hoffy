package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/mark3labs/mcp-go/mcp"
)

// ValidateTool handles the sdd_validate MCP tool.
// It performs a cross-artifact consistency check across all SDD documents
// and produces a validation report. This is the final stage of the pipeline.
type ValidateTool struct {
	store  config.Store
	bridge StageObserver
}

// NewValidateTool creates a ValidateTool with its dependencies.
func NewValidateTool(store config.Store) *ValidateTool {
	return &ValidateTool{store: store}
}

// SetBridge injects an optional StageObserver that gets notified
// when the validate stage completes. Nil is safe (disables bridge).
func (t *ValidateTool) SetBridge(obs StageObserver) { t.bridge = obs }

// Definition returns the MCP tool definition for registration.
func (t *ValidateTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_validate",
		mcp.WithDescription(
			"Run a cross-artifact consistency check across all SDD documents. "+
				"This is Stage 6 (final) of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST read ALL artifacts "+
				"(principles, charter, requirements, clarifications, design, tasks) using sdd_get_context "+
				"and perform a thorough cross-reference analysis. "+
				"The AI should check: requirement coverage, component coverage, task traceability, "+
				"dependency validity, and identify any gaps or inconsistencies. "+
				"Pass the ACTUAL validation results (not placeholders). "+
				"Requires: sdd_create_tasks must have been run first.",
		),
		mcp.WithString("requirements_coverage",
			mcp.Required(),
			mcp.Description("Analysis of whether every requirement (FR-XXX/NFR-XXX) is covered "+
				"by at least one task. List covered and uncovered requirements. "+
				"Example: '**Covered (12/14)**:\\n- FR-001 → TASK-001, TASK-002\\n"+
				"- FR-002 → TASK-003\\n...\\n\\n"+
				"**Uncovered (2/14)**:\\n- FR-013: No task addresses CSV export\\n"+
				"- NFR-003: No task addresses rate limiting'"),
		),
		mcp.WithString("component_coverage",
			mcp.Required(),
			mcp.Description("Analysis of whether every component in the design has tasks assigned. "+
				"Example: '**Covered**:\\n- AuthModule → TASK-002, TASK-003, TASK-004\\n"+
				"- DatabaseModule → TASK-001\\n\\n"+
				"**Uncovered**:\\n- EmailModule: No tasks create email integration'"),
		),
		mcp.WithString("consistency_issues",
			mcp.Required(),
			mcp.Description("List of inconsistencies found between artifacts. "+
				"Example: '1. **Mismatch**: Design specifies PostgreSQL but TASK-005 mentions MongoDB setup\\n"+
				"2. **Gap**: Requirements mention OAuth login (FR-008) but design only covers email/password auth\\n"+
				"3. **Scope creep**: TASK-011 implements push notifications which is listed as out-of-scope in charter'"),
		),
		mcp.WithString("risk_assessment",
			mcp.Description("Identified risks and their mitigation strategies. "+
				"Example: '1. **High**: No tasks for database migration strategy — could cause data loss in production\\n"+
				"2. **Medium**: OAuth integration (FR-008) has no design details — may require design revision\\n"+
				"3. **Low**: No monitoring/observability tasks — acceptable for MVP'"),
		),
		mcp.WithString("verdict",
			mcp.Required(),
			mcp.Description("Overall validation result: 'PASS', 'PASS_WITH_WARNINGS', or 'FAIL'. "+
				"PASS: All requirements covered, no consistency issues. "+
				"PASS_WITH_WARNINGS: Minor gaps or low-risk issues that don't block implementation. "+
				"FAIL: Critical gaps, missing requirement coverage, or major inconsistencies that "+
				"require revision of previous stages."),
		),
		mcp.WithString("recommendations",
			mcp.Description("Specific actionable recommendations. "+
				"For FAIL: which stage to revisit and what to fix. "+
				"For PASS_WITH_WARNINGS: issues to track during implementation. "+
				"Example: '1. Revisit design to add EmailModule architecture\\n"+
				"2. Add TASK for database migration strategy\\n"+
				"3. Track NFR-003 (rate limiting) as tech debt for v1.1'"),
		),
		mcp.WithString("design_quality",
			mcp.Description("Structural quality verification of the design. "+
				"Cross-check the design's Structural Quality Analysis section against the task breakdown. "+
				"Verify: (1) SOLID compliance — does each task maintain SRP? Does a single requirement "+
				"change touch more than 2 components (Shotgun Surgery risk)? "+
				"(2) Coupling — do tasks introduce new inter-component dependencies not documented in design? "+
				"(3) Smell propagation — do the tasks mitigate or amplify the smells identified in the design? "+
				"Reference Martin Fowler's Refactoring catalog for smell definitions."),
		),
	)
}

// Handle processes the sdd_validate tool call.
func (t *ValidateTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reqCoverage := req.GetString("requirements_coverage", "")
	compCoverage := req.GetString("component_coverage", "")
	consistencyIssues := req.GetString("consistency_issues", "")
	riskAssessment := req.GetString("risk_assessment", "")
	verdict := req.GetString("verdict", "")
	recommendations := req.GetString("recommendations", "")
	designQuality := req.GetString("design_quality", "")

	// Validate required fields.
	if reqCoverage == "" {
		return mcp.NewToolResultError("'requirements_coverage' is required — analyze requirement-to-task traceability"), nil
	}
	if compCoverage == "" {
		return mcp.NewToolResultError("'component_coverage' is required — analyze component-to-task coverage"), nil
	}
	if consistencyIssues == "" {
		return mcp.NewToolResultError("'consistency_issues' is required — list cross-artifact inconsistencies (or '_None found._')"), nil
	}
	if verdict == "" {
		return mcp.NewToolResultError("'verdict' is required — must be 'PASS', 'PASS_WITH_WARNINGS', or 'FAIL'"), nil
	}

	// Validate verdict value.
	verdictUpper := strings.ToUpper(strings.TrimSpace(verdict))
	if verdictUpper != "PASS" && verdictUpper != "PASS_WITH_WARNINGS" && verdictUpper != "FAIL" {
		return mcp.NewToolResultError(
			"'verdict' must be 'PASS', 'PASS_WITH_WARNINGS', or 'FAIL' — got: " + verdict,
		), nil
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
	if err := pipeline.RequireStage(cfg, config.StageValidate); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Verify all previous artifacts exist.
	for _, stage := range []config.Stage{
		config.StagePrinciples,
		config.StageCharter,
		config.StageSpecify,
		config.StageClarify,
		config.StageDesign,
		config.StageTasks,
	} {
		path := config.StagePath(projectRoot, stage)
		content, err := readStageFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s artifact: %w", stage, err)
		}
		if content == "" {
			return mcp.NewToolResultError(
				fmt.Sprintf("%s is empty — all previous stages must be completed before validation", config.StageFilename(stage)),
			), nil
		}
	}

	pipeline.MarkInProgress(cfg)

	// Fill optional fields with defaults.
	if riskAssessment == "" {
		riskAssessment = "_No specific risks identified._"
	}
	if recommendations == "" {
		recommendations = "_No additional recommendations._"
	}
	if designQuality == "" {
		designQuality = "_No structural quality verification provided._"
	}

	// Build the validation report.
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — Validation Report\n\n", cfg.Name)
	sb.WriteString("> Generated by [SDD-Hoffy](https://github.com/HendryAvila/Hoofy) | Stage 6: Validate\n\n")
	fmt.Fprintf(&sb, "## Verdict: %s\n\n", verdictUpper)
	sb.WriteString("---\n\n")
	sb.WriteString("## Requirements Coverage\n\n")
	sb.WriteString(reqCoverage)
	sb.WriteString("\n\n## Component Coverage\n\n")
	sb.WriteString(compCoverage)
	sb.WriteString("\n\n## Consistency Issues\n\n")
	sb.WriteString(consistencyIssues)
	sb.WriteString("\n\n## Risk Assessment\n\n")
	sb.WriteString(riskAssessment)
	sb.WriteString("\n\n## Design Quality\n\n")
	sb.WriteString(designQuality)
	sb.WriteString("\n\n## Recommendations\n\n")
	sb.WriteString(recommendations)

	content := sb.String()

	// Write the validation report.
	validatePath := config.StagePath(projectRoot, config.StageValidate)
	if err := writeStageFile(validatePath, content); err != nil {
		return nil, fmt.Errorf("writing validation report: %w", err)
	}

	// Mark the final stage as completed (no Advance — this IS the last stage).
	st := cfg.StageStatus[config.StageValidate]
	st.Status = "completed"
	st.CompletedAt = pipeline.Now()
	cfg.StageStatus[config.StageValidate] = st

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	notifyObserver(t.bridge, cfg.Name, config.StageValidate, content)

	// Build response based on verdict.
	var nextStep string
	switch verdictUpper {
	case "PASS":
		nextStep = "## 🎉 SDD Pipeline Complete!\n\n" +
			"All specifications are consistent and ready for implementation.\n\n" +
			"**Your SDD artifacts:**\n" +
			"- `docs/principles.md` — Golden invariants and coding standards\n" +
			"- `docs/charter.md` — What we're building and why\n" +
			"- `docs/requirements.md` — Formal requirements (MoSCoW)\n" +
			"- `docs/clarifications.md` — Resolved ambiguities\n" +
			"- `docs/design.md` — Technical architecture\n" +
			"- `docs/tasks.md` — Implementation task breakdown\n" +
			"- `docs/validation.md` — This consistency report\n\n" +
			"**Next:** Use these specs with your AI coding tool's `/plan mode` to start implementation. " +
			"The specs will dramatically reduce hallucinations because every requirement is clear, " +
			"traced to a task, and architecturally grounded."
	case "PASS_WITH_WARNINGS":
		nextStep = "## ⚠️ SDD Pipeline Complete (with warnings)\n\n" +
			"Specifications are usable but have minor gaps. " +
			"Track the warnings during implementation.\n\n" +
			"**Recommendations:**\n\n" + recommendations + "\n\n" +
			"**Next:** You can proceed to implementation, but keep an eye on the flagged issues."
	case "FAIL":
		nextStep = "## ❌ Validation Failed\n\n" +
			"Critical gaps or inconsistencies were found. " +
			"Implementation would likely produce incorrect results.\n\n" +
			"**Required actions:**\n\n" + recommendations + "\n\n" +
			"**Next:** Revisit the stages mentioned above to fix the issues, " +
			"then re-run validation."
	}

	response := fmt.Sprintf(
		"# Validation Report\n\n"+
			"**Verdict:** %s\n\n"+
			"Saved to `docs/validation.md`\n\n"+
			"## Summary\n\n%s\n\n"+
			"---\n\n"+
			"%s",
		verdictUpper, content, nextStep,
	)

	return mcp.NewToolResultText(response), nil
}
