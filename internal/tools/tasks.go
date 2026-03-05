package tools

import (
	"context"
	"fmt"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/pipeline"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// TasksTool handles the sdd_create_tasks MCP tool.
// It saves an implementation task breakdown with content provided by the AI.
type TasksTool struct {
	store    config.Store
	renderer templates.Renderer
	bridge   StageObserver
}

// NewTasksTool creates a TasksTool with its dependencies.
func NewTasksTool(store config.Store, renderer templates.Renderer) *TasksTool {
	return &TasksTool{store: store, renderer: renderer}
}

// SetBridge injects an optional StageObserver that gets notified
// when the tasks stage completes. Nil is safe (disables bridge).
func (t *TasksTool) SetBridge(obs StageObserver) { t.bridge = obs }

// Definition returns the MCP tool definition for registration.
func (t *TasksTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_create_tasks",
		mcp.WithDescription(
			"Save an implementation task breakdown for the SDD project. "+
				"This is Stage 5 of the SDD pipeline. "+
				"IMPORTANT: Before calling this tool, the AI MUST read the design document "+
				"(use sdd_get_context stage=design) and break it into atomic, AI-ready tasks. "+
				"Each task should be small enough for a single commit, have clear acceptance criteria, "+
				"and reference the requirements (FR-XXX/NFR-XXX) and components it implements. "+
				"Pass the ACTUAL task content (not placeholders). "+
				"Requires: sdd_create_design must have been run first.",
		),
		mcp.WithString("total_tasks",
			mcp.Required(),
			mcp.Description("Total number of tasks in the breakdown. "+
				"Example: '12'"),
		),
		mcp.WithString("estimated_effort",
			mcp.Required(),
			mcp.Description("High-level effort estimate for the full implementation. "+
				"Example: '3-4 days for a single developer' or '2 sprints (4 weeks) for a 3-person team'"),
		),
		mcp.WithString("tasks",
			mcp.Required(),
			mcp.Description("The ordered list of implementation tasks. Each task MUST include: "+
				"a unique ID (TASK-001), title, description, requirements covered (FR-XXX), "+
				"component(s) affected, dependencies on other tasks, and acceptance criteria. "+
				"Use markdown format. "+
				"Example: '### TASK-001: Set up project scaffolding\\n"+
				"**Component**: ProjectSetup\\n"+
				"**Covers**: Infrastructure\\n"+
				"**Dependencies**: None\\n"+
				"**Description**: Initialize the project with the chosen tech stack...\\n"+
				"**Acceptance Criteria**:\\n"+
				"- [ ] Project builds and runs locally\\n"+
				"- [ ] Linter and formatter configured\\n"+
				"- [ ] CI pipeline runs on push'"),
		),
		mcp.WithString("dependency_graph",
			mcp.Description("Visual or textual representation of task dependencies. "+
				"Shows which tasks can be parallelized and which must be sequential. "+
				"Example: 'TASK-001 → TASK-002 → TASK-003\\n"+
				"TASK-001 → TASK-004 (can parallel with TASK-002)\\n"+
				"TASK-003 + TASK-004 → TASK-005'"),
		),
		mcp.WithString("wave_assignments",
			mcp.Description("Parallel execution wave groupings derived from the dependency graph. "+
				"Group tasks into waves where all tasks in the same wave can execute in parallel. "+
				"Wave N+1 starts only after all wave N tasks complete. "+
				"Algorithm: tasks with no dependencies = Wave 1, tasks depending only on Wave 1 = Wave 2, etc. "+
				"Use markdown format. "+
				"Example: '**Wave 1** (parallel — no dependencies):\\n"+
				"- TASK-001: Project scaffolding\\n"+
				"- TASK-002: Database schema\\n\\n"+
				"**Wave 2** (parallel — depends on Wave 1):\\n"+
				"- TASK-003: API endpoints\\n"+
				"- TASK-004: Auth module\\n\\n"+
				"**Wave 3** (sequential — depends on Wave 2):\\n"+
				"- TASK-005: Integration tests'"),
		),
		mcp.WithString("acceptance_criteria",
			mcp.Description("Global acceptance criteria that apply across ALL tasks. "+
				"These are the project-wide quality gates. "+
				"Example: '- All code must pass linting with zero warnings\\n"+
				"- Test coverage must be ≥ 80%\\n"+
				"- All API endpoints must have integration tests'"),
		),
	)
}

// Handle processes the sdd_create_tasks tool call.
func (t *TasksTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	totalTasks := req.GetString("total_tasks", "")
	estimatedEffort := req.GetString("estimated_effort", "")
	tasks := req.GetString("tasks", "")
	dependencyGraph := req.GetString("dependency_graph", "")
	waveAssignments := req.GetString("wave_assignments", "")
	acceptanceCriteria := req.GetString("acceptance_criteria", "")

	// Validate required fields.
	if totalTasks == "" {
		return mcp.NewToolResultError("'total_tasks' is required — how many tasks in the breakdown?"), nil
	}
	if estimatedEffort == "" {
		return mcp.NewToolResultError("'estimated_effort' is required — what's the estimated effort?"), nil
	}
	if tasks == "" {
		return mcp.NewToolResultError("'tasks' is required — provide the ordered list of implementation tasks"), nil
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
	if err := pipeline.RequireStage(cfg, config.StageTasks); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Verify the design document exists.
	designPath := config.StagePath(projectRoot, config.StageDesign)
	design, err := readStageFile(designPath)
	if err != nil {
		return nil, fmt.Errorf("reading design: %w", err)
	}
	if design == "" {
		return mcp.NewToolResultError("design.md is empty — run sdd_create_design first"), nil
	}

	pipeline.MarkInProgress(cfg)

	// Fill optional fields with defaults.
	if dependencyGraph == "" {
		dependencyGraph = "_No explicit dependency graph defined. Tasks should be executed in order._"
	}
	if acceptanceCriteria == "" {
		acceptanceCriteria = "_No global acceptance criteria defined. See individual task criteria._"
	}

	// Build tasks document with REAL content from the AI.
	data := templates.TasksData{
		Name:               cfg.Name,
		TotalTasks:         totalTasks,
		EstimatedEffort:    estimatedEffort,
		Tasks:              tasks,
		DependencyGraph:    dependencyGraph,
		WaveAssignments:    waveAssignments,
		AcceptanceCriteria: acceptanceCriteria,
	}

	content, err := t.renderer.Render(templates.Tasks, data)
	if err != nil {
		return nil, fmt.Errorf("rendering tasks: %w", err)
	}

	// Write the tasks file.
	tasksPath := config.StagePath(projectRoot, config.StageTasks)
	if err := writeStageFile(tasksPath, content); err != nil {
		return nil, fmt.Errorf("writing tasks: %w", err)
	}

	// Advance pipeline to next stage.
	if err := pipeline.Advance(cfg); err != nil {
		return nil, fmt.Errorf("advancing pipeline: %w", err)
	}

	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	notifyObserver(t.bridge, cfg.Name, config.StageTasks, content)

	response := fmt.Sprintf(
		"# Implementation Tasks Created\n\n"+
			"Saved to `docs/tasks.md`\n\n"+
			"## Content\n\n%s\n\n"+
			"---\n\n"+
			"## Next Step\n\n"+
			"Pipeline advanced to **Stage 6: Validate**.\n\n"+
			"Now run a cross-artifact consistency check to verify:\n"+
			"- Every requirement (FR-XXX/NFR-XXX) is covered by at least one task\n"+
			"- Every component in the design has tasks assigned to it\n"+
			"- Task dependencies are valid (no circular dependencies)\n"+
			"- No orphaned tasks (tasks that don't trace to any requirement)\n\n"+
			"Call `sdd_validate` with your validation analysis.",
		content,
	)

	return mcp.NewToolResultText(response), nil
}
