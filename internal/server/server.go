// Package server wires all MCP components and creates the server instance.
//
// This is the composition root (DIP): it creates concrete implementations
// and injects them into the tools/prompts/resources that depend on abstractions.
// No business logic lives here — only wiring.
package server

import (
	"fmt"
	"log"

	"github.com/HendryAvila/Hoofy/internal/changes"
	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/HendryAvila/Hoofy/internal/memtools"
	"github.com/HendryAvila/Hoofy/internal/prompts"
	"github.com/HendryAvila/Hoofy/internal/resources"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/HendryAvila/Hoofy/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

// Version is set at build time via ldflags.
var Version = "dev"

// New creates and configures the MCP server with all tools, prompts,
// and resources registered. This is the single place where all
// dependencies are resolved.
//
// The returned cleanup function closes the memory store's database
// connection and must be called on shutdown (typically via defer).
// It is always non-nil and safe to call even if memory init failed.
func New() (*server.MCPServer, func(), error) {
	// --- Create shared dependencies ---

	store := config.NewFileStore()

	renderer, err := templates.NewRenderer()
	if err != nil {
		return nil, noop, fmt.Errorf("creating template renderer: %w", err)
	}

	// --- Create the MCP server ---

	s := server.NewMCPServer(
		"hoofy",
		Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
		server.WithInstructions(serverInstructions()),
	)

	// --- Register SDD tools ---

	initTool := tools.NewInitTool(store, renderer)
	s.AddTool(initTool.Definition(), initTool.Handle)

	principlesTool := tools.NewPrinciplesTool(store, renderer)
	s.AddTool(principlesTool.Definition(), principlesTool.Handle)

	charterTool := tools.NewCharterTool(store, renderer)
	s.AddTool(charterTool.Definition(), charterTool.Handle)

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

	businessRulesTool := tools.NewBusinessRulesTool(store, renderer)
	s.AddTool(businessRulesTool.Definition(), businessRulesTool.Handle)

	// --- Register bootstrap & reverse-engineer tools ---
	//
	// These tools work without hoofy.json or an active pipeline.
	// sdd_reverse_engineer is a read-only scanner (no dependencies).
	// sdd_bootstrap writes artifacts using shared rendering functions.

	reverseEngineerTool := tools.NewReverseEngineerTool()
	s.AddTool(reverseEngineerTool.Definition(), reverseEngineerTool.Handle)

	bootstrapTool := tools.NewBootstrapTool(renderer)
	s.AddTool(bootstrapTool.Definition(), bootstrapTool.Handle)

	auditTool := tools.NewAuditTool()
	s.AddTool(auditTool.Definition(), auditTool.Handle)

	// --- Register change pipeline tools ---
	//
	// The change pipeline is independent from the project pipeline —
	// it works without hoofy.json. It uses its own FileStore for
	// persistence under docs/changes/.

	changeStore := changes.NewFileStore()

	changeTool := tools.NewChangeTool(changeStore)
	s.AddTool(changeTool.Definition(), changeTool.Handle)

	changeAdvanceTool := tools.NewChangeAdvanceTool(changeStore)
	s.AddTool(changeAdvanceTool.Definition(), changeAdvanceTool.Handle)

	changeStatusTool := tools.NewChangeStatusTool(changeStore)
	s.AddTool(changeStatusTool.Definition(), changeStatusTool.Handle)

	adrTool := tools.NewADRTool(changeStore)
	s.AddTool(adrTool.Definition(), adrTool.Handle)

	// --- Register memory tools ---
	//
	// Memory is an independent subsystem: if it fails to initialize,
	// SDD tools continue working. We log a warning and skip memory
	// tool registration — the server is still fully functional for
	// spec-driven development.

	cleanup := noop
	memStore, memErr := memory.New(memory.DefaultConfig())

	// Context-check tool registered unconditionally — handles nil memStore
	// internally by skipping memory search (ADR-001: scanner, not analyzer).
	contextCheckTool := tools.NewContextCheckTool(changeStore, memStore)
	s.AddTool(contextCheckTool.Definition(), contextCheckTool.Handle)

	// Suggest-context tool registered unconditionally — standalone tool
	// for ad-hoc sessions, works without active change or hoofy.json.
	suggestContextTool := tools.NewSuggestContextTool(changeStore, memStore)
	s.AddTool(suggestContextTool.Definition(), suggestContextTool.Handle)

	// Unified SDD context facade: get/check/suggest in one entrypoint.
	sddContextTool := tools.NewSDDContextTool(contextTool, contextCheckTool, suggestContextTool)
	s.AddTool(sddContextTool.Definition(), sddContextTool.Handle)

	// Review tool registered unconditionally — standalone spec-aware
	// code review, generates checklists from project specs.
	reviewTool := tools.NewReviewTool(memStore)
	s.AddTool(reviewTool.Definition(), reviewTool.Handle)
	if memErr != nil {
		log.Printf("WARNING: memory subsystem disabled: %v", memErr)
	} else {
		cleanup = func() {
			if err := memStore.Close(); err != nil {
				log.Printf("WARNING: memory store close: %v", err)
			}
		}
		registerMemoryTools(s, memStore)

		// --- Wire SDD-Memory bridge ---
		//
		// When memory is available, SDD stage completions are automatically
		// saved as memory observations with topic_key upserts. This enables
		// cross-session awareness of pipeline state. The bridge is nil-safe:
		// if memory init failed, tools work normally without it.
		bridge := tools.NewMemoryBridge(memStore)
		principlesTool.SetBridge(bridge)
		charterTool.SetBridge(bridge)
		specifyTool.SetBridge(bridge)
		businessRulesTool.SetBridge(bridge)
		clarifyTool.SetBridge(bridge)
		designTool.SetBridge(bridge)
		tasksTool.SetBridge(bridge)
		validateTool.SetBridge(bridge)

		// Wire change pipeline bridge — saves stage completions and ADRs
		// to memory for cross-session awareness.
		changeAdvanceTool.SetBridge(bridge)
		adrTool.SetBridge(bridge)

		// --- Register explore tool (SDD + Memory hybrid) ---
		//
		// sdd_explore is a standalone tool that captures pre-pipeline context.
		// It depends only on memory.Store, not on config or change stores.
		// Registered here because it requires memory to be available.
		exploreTool := tools.NewExploreTool(memStore)
		s.AddTool(exploreTool.Definition(), exploreTool.Handle)
	}

	// --- Register prompts ---

	startPrompt := prompts.NewStartPrompt()
	s.AddPrompt(startPrompt.Definition(), startPrompt.Handle)

	statusPrompt := prompts.NewStatusPrompt()
	s.AddPrompt(statusPrompt.Definition(), statusPrompt.Handle)

	stageGuide := prompts.NewStageGuidePrompt()
	s.AddPrompt(stageGuide.Definition(), stageGuide.Handle)

	memoryGuide := prompts.NewMemoryGuidePrompt()
	s.AddPrompt(memoryGuide.Definition(), memoryGuide.Handle)

	changeGuide := prompts.NewChangeGuidePrompt()
	s.AddPrompt(changeGuide.Definition(), changeGuide.Handle)

	bootstrapGuide := prompts.NewBootstrapGuidePrompt()
	s.AddPrompt(bootstrapGuide.Definition(), bootstrapGuide.Handle)

	// --- Register resources ---

	resourceHandler := resources.NewHandler(store)
	s.AddResource(resourceHandler.StatusResource(), resourceHandler.HandleStatus)

	return s, cleanup, nil
}

// noop is a no-op cleanup function used as the default when memory
// is disabled or hasn't been initialized.
func noop() {}

// registerMemoryTools registers memory MCP tools with the server.
func registerMemoryTools(s *server.MCPServer, ms *memory.Store) {
	// --- Session lifecycle ---
	sessionTool := memtools.NewSessionTool(ms)
	s.AddTool(sessionTool.Definition(), sessionTool.Handle)

	// --- Save & capture ---
	saveTool := memtools.NewSaveTool(ms)
	s.AddTool(saveTool.Definition(), saveTool.Handle)

	// --- Progress tracking ---
	progressTool := memtools.NewProgressTool(ms)
	s.AddTool(progressTool.Definition(), progressTool.Handle)

	// --- Query & retrieval ---
	searchTool := memtools.NewSearchTool(ms)
	s.AddTool(searchTool.Definition(), searchTool.Handle)

	memContext := memtools.NewContextTool(ms)
	s.AddTool(memContext.Definition(), memContext.Handle)

	timelineTool := memtools.NewTimelineTool(ms)
	s.AddTool(timelineTool.Definition(), timelineTool.Handle)

	getObs := memtools.NewGetObservationTool(ms)
	s.AddTool(getObs.Definition(), getObs.Handle)

	// --- Management ---
	deleteTool := memtools.NewDeleteTool(ms)
	s.AddTool(deleteTool.Definition(), deleteTool.Handle)

	updateTool := memtools.NewUpdateTool(ms)
	s.AddTool(updateTool.Definition(), updateTool.Handle)

	suggestKey := memtools.NewSuggestTopicKeyTool()
	s.AddTool(suggestKey.Definition(), suggestKey.Handle)

	// --- Compaction ---
	compactTool := memtools.NewCompactTool(ms)
	s.AddTool(compactTool.Definition(), compactTool.Handle)

	// --- Statistics ---
	statsTool := memtools.NewStatsTool(ms)
	s.AddTool(statsTool.Definition(), statsTool.Handle)

	// --- Knowledge graph (relations) ---
	relateTool := memtools.NewRelateTool(ms)
	s.AddTool(relateTool.Definition(), relateTool.Handle)
}

// serverInstructions returns the system instructions that tell the AI
// how to use Hoofy effectively.
//
// This is the "hot" layer — always loaded, ~160 lines. Detailed instructions
// for specific workflows are served on-demand via MCP prompts (cold layer).
// See: stage_guide.go, memory_guide.go, change_guide.go, bootstrap_guide.go
func serverInstructions() string {
	return `You have access to Hoofy, a Spec-Driven Development MCP server.

## WHEN TO ACTIVATE Hoofy

You MUST proactively suggest using Hoofy when the user:
- Asks to build a new project, app, or system
- Asks to add a new feature or major enhancement
- Describes a vague idea and wants to start coding
- Says things like "I want to build...", "let's create...", "add a feature for..."
- Asks you to plan, architect, or design something

When you detect any of these, say something like:
"Before we start coding, let's use Hoofy to define clear specs.
This prevents hallucinations and ensures we build exactly what you need.
Should I start the SDD pipeline?"

You do NOT need to activate Hoofy for:
- Bug fixes or small patches
- Refactoring existing code without changing behavior
- Questions, explanations, or documentation
- One-liner changes or config tweaks

For bug fixes, refactors, enhancements, and small features, use the
ADAPTIVE CHANGE PIPELINE instead (see below).

For ad-hoc sessions (quick tasks, exploration, debugging), call sdd_suggest_context
with a task description to get relevant specs, memory, and changes to read first.
It works without a pipeline or hoofy.json.

For code review, call sdd_review with a change description to generate a spec-aware
review checklist. Each item references specific spec IDs (FR-XXX, BRC-XXX, ADRs).
It works without a pipeline or hoofy.json.

For spec compliance auditing, call sdd_audit to compare specs/requirements against
actual source code. It scans the codebase and reports discrepancies (unimplemented
requirements, undocumented features, stale specs). It works without a pipeline or hoofy.json.

## What is SDD?

Spec-Driven Development reduces AI hallucinations by forcing clear specifications
BEFORE writing code. Ambiguous requirements are the #1 cause of bad AI-generated code.
(Source: IEEE 29148 — "well-formed requirements" prevent defects downstream)

## CRITICAL: How Tools Work

Hoofy tools are STORAGE tools, not AI tools. They save content YOU generate.
The workflow for each stage is:

1. TALK to the user — understand their idea, ask questions
2. GENERATE the content yourself (proposals, requirements, etc.)
3. CALL the tool with the ACTUAL content as parameters
4. The tool saves it to disk and advances the pipeline

NEVER call a tool with placeholder text like "TBD" or "to be defined".
ALWAYS generate real, substantive content based on your conversation with the user.

## Pipeline

SDD follows a sequential 9-stage pipeline:
1. INIT — Set up the project (call sdd_init_project)
2. PRINCIPLES — Define golden invariants and core beliefs (call sdd_create_principles)
3. CHARTER — Create a structured charter (YOU write it, tool saves it)
4. SPECIFY — Extract formal requirements with IEEE 29148 quality attributes
5. BUSINESS RULES — Extract declarative business rules using BRG taxonomy
6. CLARIFY — The Clarity Gate: resolve ambiguities using EARS patterns
7. DESIGN — Technical architecture document with ADRs (Michael Nygard format)
8. TASKS — Atomic task breakdown with execution wave assignments
9. VALIDATE — Cross-artifact consistency check (YOU analyze, tool saves report)

Before starting any pipeline, use sdd_explore to capture the user's context,
goals, and constraints. It's optional but strongly recommended.

For stage-by-stage details, invoke the /sdd-stage-guide prompt.

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
- After validation, the user's SDD specs are ready for implementation

## PERSISTENT MEMORY

Hoofy includes a persistent memory system for cross-session awareness.
Memory survives between conversations — use it to build project knowledge over time.

### When to Save (call mem_save PROACTIVELY after each of these)
- Architectural decisions or tradeoffs made
- Bug fixes: what was wrong, why, how it was fixed
- New patterns or conventions established
- Configuration changes or environment setup
- Important discoveries, gotchas, or edge cases
- File structure changes or significant refactoring

### Content Format (use this structured format for mem_save content)
**What**: [concise description of what was done]
**Why**: [the reasoning, user request, or problem that drove it]
**Where**: [files/paths affected, e.g. src/auth/middleware.ts]
**Learned**: [gotchas, edge cases, or decisions — omit if none]

### Type Categories
Use the type parameter: decision, architecture, bugfix, pattern, config, discovery, learning

### When to Search (call mem_search)
- At the start of a new session to recover context
- Before making architectural decisions (check if prior decisions exist)
- When encountering familiar errors or patterns
- When the user references something from a previous session

### Session Lifecycle
1. Call mem_session with action="start" at the beginning of each coding session
2. Save observations throughout the session (decisions, fixes, discoveries) using mem_save
3. Optionally include a structured summary in mem_session(action="end", summary=...)
4. Call mem_session with action="end" to close the session

For full memory documentation (progress tracking, compaction, topic keys,
namespace scoping, context budget, knowledge graph, progressive disclosure),
invoke the /sdd-memory-guide prompt.

## ADAPTIVE CHANGE PIPELINE

For ongoing development (features, fixes, refactors, enhancements), use the
adaptive change pipeline instead of the full 9-stage SDD pipeline.

### When to Use Changes vs Full Pipeline
- **Full pipeline** (sdd_init_project): Brand new projects from scratch
- **Change pipeline** (sdd_change): Any modification to an existing codebase

### How It Works
Each change has a TYPE and SIZE that determine the pipeline stages.
ALL flows include a mandatory context-check stage.

**Types**: feature, fix, refactor, enhancement
**Sizes**: small (4 stages), medium (5 stages), large (6-7 stages)

### Stage Flows by Type and Size

**Fix**:
- small: describe → context-check → tasks → verify
- medium: describe → context-check → spec → tasks → verify
- large: describe → context-check → spec → design → tasks → verify

**Feature**:
- small: describe → context-check → tasks → verify
- medium: charter → context-check → spec → tasks → verify
- large: charter → context-check → spec → clarify → design → tasks → verify

**Refactor**:
- small: scope → context-check → tasks → verify
- medium: scope → context-check → design → tasks → verify
- large: scope → context-check → spec → design → tasks → verify

**Enhancement**:
- small: describe → context-check → tasks → verify
- medium: charter → context-check → spec → tasks → verify
- large: charter → context-check → spec → clarify → design → tasks → verify

### Change Pipeline Workflow

1. **Create a change**: Call sdd_change with type, size, and description
   - Only ONE active change at a time
   - The tool creates a directory at docs/changes/<slug>/

2. **Work through stages**: For each stage, generate content and call
   sdd_change_advance with the content
   - The tool writes the content as <stage>.md in the change directory
   - It advances the state machine to the next stage
   - When the final stage (verify) is completed, the change is marked done

3. **Check progress**: Call sdd_change_status to see the current state

4. **Capture decisions**: Call sdd_adr at any time to record an ADR

### Important Rules
- Only ONE active change at a time
- Complete or archive a change before starting a new one
- Generate REAL content for each stage — no placeholders
- All flows end with verify — use it to validate the change
- ADRs can be captured at any time during a change
- Context-check is MANDATORY — never skip it, even for small changes

For detailed change pipeline documentation (context-check heuristics,
structural quality analysis, wave execution orchestration), invoke the
/sdd-change-guide prompt.

## EXISTING PROJECTS

When sdd_change is used on a project with NO existing SDD artifacts,
medium/large changes are BLOCKED until you bootstrap specs.
Small changes proceed with a warning.

For the full bootstrap workflow (sdd_reverse_engineer + sdd_bootstrap),
invoke the /sdd-bootstrap-guide prompt.

## ON-DEMAND INSTRUCTION GUIDES

The following prompts provide detailed instructions for specific workflows.
Invoke them when you need the full reference:

| Prompt | When to Invoke |
|--------|---------------|
| /sdd-stage-guide | Working on any pipeline stage (Principles through Validate) |
| /sdd-memory-guide | Using advanced memory features (compaction, namespaces, graph, budget) |
| /sdd-change-guide | Working on context-check, structural quality, or wave execution |
| /sdd-bootstrap-guide | Bootstrapping an existing project into SDD |`
}
