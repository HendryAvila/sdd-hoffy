// Package server wires all MCP components and creates the server instance.
//
// This is the composition root (DIP): it creates concrete implementations
// and injects them into the tools/prompts/resources that depend on abstractions.
// No business logic lives here — only wiring.
package server

import (
	"fmt"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
	"github.com/HendryAvila/sdd-hoffy/internal/prompts"
	"github.com/HendryAvila/sdd-hoffy/internal/resources"
	"github.com/HendryAvila/sdd-hoffy/internal/templates"
	"github.com/HendryAvila/sdd-hoffy/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

// Version is set at build time via ldflags.
var Version = "dev"

// New creates and configures the MCP server with all tools, prompts,
// and resources registered. This is the single place where all
// dependencies are resolved.
func New() (*server.MCPServer, error) {
	// --- Create shared dependencies ---

	store := config.NewFileStore()

	renderer, err := templates.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("creating template renderer: %w", err)
	}

	// --- Create the MCP server ---

	s := server.NewMCPServer(
		"sdd-hoffy",
		Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
		server.WithInstructions(serverInstructions()),
	)

	// --- Register tools ---

	initTool := tools.NewInitTool(store)
	s.AddTool(initTool.Definition(), initTool.Handle)

	proposeTool := tools.NewProposeTool(store, renderer)
	s.AddTool(proposeTool.Definition(), proposeTool.Handle)

	specifyTool := tools.NewSpecifyTool(store, renderer)
	s.AddTool(specifyTool.Definition(), specifyTool.Handle)

	clarifyTool := tools.NewClarifyTool(store, renderer)
	s.AddTool(clarifyTool.Definition(), clarifyTool.Handle)

	designTool := tools.NewDesignTool(store, renderer)
	s.AddTool(designTool.Definition(), designTool.Handle)

	tasksTool := tools.NewTasksTool(store, renderer)
	s.AddTool(tasksTool.Definition(), tasksTool.Handle)

	validateTool := tools.NewValidateTool(store)
	s.AddTool(validateTool.Definition(), validateTool.Handle)

	contextTool := tools.NewContextTool(store)
	s.AddTool(contextTool.Definition(), contextTool.Handle)

	// --- Register prompts ---

	startPrompt := prompts.NewStartPrompt()
	s.AddPrompt(startPrompt.Definition(), startPrompt.Handle)

	statusPrompt := prompts.NewStatusPrompt()
	s.AddPrompt(statusPrompt.Definition(), statusPrompt.Handle)

	// --- Register resources ---

	resourceHandler := resources.NewHandler(store)
	s.AddResource(resourceHandler.StatusResource(), resourceHandler.HandleStatus)

	return s, nil
}

// serverInstructions returns the system instructions that tell the AI
// how to use SDD-Hoffy effectively.
func serverInstructions() string {
	return `You have access to SDD-Hoffy, a Spec-Driven Development MCP server.

## WHEN TO ACTIVATE SDD-Hoffy

You MUST proactively suggest using SDD-Hoffy when the user:
- Asks to build a new project, app, or system
- Asks to add a new feature or major enhancement
- Describes a vague idea and wants to start coding
- Says things like "I want to build...", "let's create...", "add a feature for..."
- Asks you to plan, architect, or design something

When you detect any of these, say something like:
"Before we start coding, let's use SDD-Hoffy to define clear specs.
This prevents hallucinations and ensures we build exactly what you need.
Should I start the SDD pipeline?"

You do NOT need to activate SDD-Hoffy for:
- Bug fixes or small patches
- Refactoring existing code without changing behavior
- Questions, explanations, or documentation
- One-liner changes or config tweaks

## What is SDD?
Spec-Driven Development reduces AI hallucinations by forcing clear specifications 
BEFORE writing code. Ambiguous requirements are the #1 cause of bad AI-generated code.

## CRITICAL: How Tools Work
SDD-Hoffy tools are STORAGE tools, not AI tools. They save content YOU generate.
The workflow for each stage is:

1. TALK to the user → understand their idea, ask questions
2. GENERATE the content yourself (proposals, requirements, etc.)
3. CALL the tool with the ACTUAL content as parameters
4. The tool saves it to disk and advances the pipeline

NEVER call a tool with placeholder text like "TBD" or "to be defined".
ALWAYS generate real, substantive content based on your conversation with the user.

## Pipeline
SDD follows a sequential 7-stage pipeline:
1. INIT → Set up the project (call sdd_init_project)
2. PROPOSE → Create a structured proposal (YOU write it, tool saves it)
3. SPECIFY → Extract formal requirements (YOU write them, tool saves them)
4. CLARIFY → The Clarity Gate: resolve ambiguities (interactive Q&A)
5. DESIGN → Technical architecture document (YOU design it, tool saves it)
6. TASKS → Atomic implementation task breakdown (YOU break it down, tool saves it)
7. VALIDATE → Cross-artifact consistency check (YOU analyze, tool saves report)

## Stage-by-Stage Workflow

### Stage 1: Propose
1. Ask the user about their project idea
2. Ask follow-up questions to understand the problem, users, and goals
3. Based on the conversation, generate content for ALL sections:
   - problem_statement: The core problem (2-3 sentences)
   - target_users: 2-3 specific user personas with needs
   - proposed_solution: High-level description (NO tech details)
   - out_of_scope: 3-5 explicit exclusions
   - success_criteria: 2-4 measurable outcomes
   - open_questions: Remaining unknowns
4. Call sdd_create_proposal with all sections filled in

### Stage 2: Specify
1. Read the proposal from sdd/proposal.md (use sdd_get_context if needed)
2. Extract formal requirements using MoSCoW prioritization
3. Each requirement gets a unique ID (FR-001 for functional, NFR-001 for non-functional)
4. Call sdd_generate_requirements with real requirements content

### Stage 3: Clarify (Clarity Gate)
1. Call sdd_clarify WITHOUT answers to get the analysis framework
2. Analyze the requirements across all 8 dimensions
3. Generate 3-5 specific questions targeting the weakest areas
4. Present questions to the user and collect answers
5. Call sdd_clarify WITH answers and your dimension_scores assessment
6. If score < threshold, repeat from step 1

### Stage 4: Design
1. Read ALL previous artifacts (use sdd_get_context for proposal, requirements, clarifications)
2. Design the technical architecture addressing ALL requirements
3. Choose tech stack with rationale, define components, data model, API contracts
4. Document key architectural decisions (ADRs) with alternatives considered
5. Call sdd_create_design with the complete architecture document

### Stage 5: Tasks
1. Read the design document (use sdd_get_context stage=design)
2. Break the design into atomic, AI-ready implementation tasks
3. Each task must have: unique ID (TASK-001), clear scope, requirements covered,
   component affected, dependencies, and acceptance criteria
4. Define the dependency graph (what can be parallelized)
5. Call sdd_create_tasks with the complete task breakdown

### Stage 6: Validate
1. Read ALL artifacts (proposal, requirements, clarifications, design, tasks)
2. Cross-reference every requirement against tasks (coverage analysis)
3. Cross-reference every component against tasks (component coverage)
4. Check for inconsistencies between artifacts
5. Assess risks and provide recommendations
6. Call sdd_validate with the full analysis and verdict (PASS/PASS_WITH_WARNINGS/FAIL)

## Modes
- Guided: More questions, examples, encouragement. For non-technical users.
  Clarity threshold: 70/100.
- Expert: Direct, concise, technical. For experienced developers.
  Clarity threshold: 50/100.

## Important Rules
- NEVER skip the Clarity Gate
- ALWAYS follow the pipeline order
- NEVER pass placeholder text to tools — generate REAL content
- Each requirement must have a unique ID (FR-001, NFR-001)
- Each task must have a unique ID (TASK-001) and trace to requirements
- Be specific — "users" is not a valid target audience
- In Guided mode: use simple language, give examples, be encouraging
- In Expert mode: be direct, technical language is fine
- After validation, the user's SDD specs are ready for implementation with /plan mode`
}
