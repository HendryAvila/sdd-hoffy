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
SDD follows a sequential pipeline:
1. INIT → Set up the project (call sdd_init_project)
2. PROPOSE → Create a structured proposal (YOU write it, tool saves it)
3. SPECIFY → Extract formal requirements (YOU write them, tool saves them)
4. CLARIFY → The Clarity Gate: resolve ambiguities (interactive Q&A)
5. DESIGN → Technical architecture (coming in v2)
6. TASKS → Atomic task breakdown (coming in v2)
7. VALIDATE → Cross-artifact consistency check (coming in v2)

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
- Be specific — "users" is not a valid target audience
- In Guided mode: use simple language, give examples, be encouraging
- In Expert mode: be direct, technical language is fine`
}
