# Tool Reference

Hoofy exposes MCP tools and on-demand prompts across five systems. The AI uses them proactively based on built-in server instructions — you don't need to call them manually.

---

## Memory

Persistent context across sessions. SQLite + FTS5 full-text search with a knowledge graph for connecting observations.

| Tool | Description |
|---|---|
| `mem_save` | Unified save interface. Supports `save_type: observation \| prompt \| passive` (default: observation), plus observation options (`upsert`, `relate_to`, `topic_key`) and `namespace` |
| `mem_session` | Unified session lifecycle: `action: start \| end` |
| `mem_search` | Full-text search across all sessions. Supports `namespace` to filter by sub-agent. Supports `max_tokens` to cap response size |
| `mem_context` | Recent observations for session startup. Supports `namespace` to filter by sub-agent. Supports `max_tokens` to cap response size |
| `mem_timeline` | Chronological context around a specific event. Supports `max_tokens` to cap response size |
| `mem_get` | Full content of a specific observation (includes direct relations) and optional graph traversal via `depth` |
| `mem_relate` | Create a typed directional relation between two observations (`relates_to`, `depends_on`, `caused_by`, `implements`, `supersedes`, `part_of`) |
| `mem_relate` | Create or remove typed relations (`action: add \| remove`) |
| `mem_stats` | Memory system statistics |
| `mem_delete` | Remove an observation |
| `mem_update` | Update an existing observation |
| `mem_suggest_topic_key` | Suggest stable key for upserts (evolving knowledge) |
| `mem_progress` | Read/write structured JSON progress doc for long-running sessions (one per project, auto-upserted). Supports `namespace` — scoped progress becomes `progress/<namespace>/<project>` |
| `mem_compact` | Identify and compact stale observations. Dual behavior: without `compact_ids` lists candidates, with `compact_ids` batch soft-deletes and optionally creates a summary observation. Supports `namespace` to scope compaction |

## Change Pipeline (5 tools)

Adaptive workflow for ongoing development. Includes mandatory `sdd_context_check` for conflict scanning.

| Tool | Description |
|---|---|
| `sdd_change` | Create a new change (feature, fix, refactor, enhancement) with size (small, medium, large). One active change at a time. Artifacts stored in `docs/changes/<slug>/` |
| `sdd_context_check` | Mandatory conflict scanner — scans existing specs, completed changes, memory observations, and convention files (`CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, etc.) for ambiguities and conflicts. Runs as a stage in every change flow. Zero issues = advance. Issues found = must resolve. Supports `max_tokens` to cap response size |
| `sdd_change_advance` | Save stage content and advance to next stage |
| `sdd_change_status` | View current change status, stage progress, and artifacts |
| `sdd_adr` | Capture Architecture Decision Records (context, decision, rationale, rejected alternatives). Stored in `docs/adrs/` with sequential `NNN-slug.md` naming |

## Bootstrap (2 tools)

Reverse-engineer existing codebases into SDD artifacts. Scan first, then bootstrap — no pipeline guards required.

| Tool | Description |
|---|---|
| `sdd_reverse_engineer` | Scan an existing codebase and produce a structured evidence report (project overview, tech stack, architecture, conventions, data model, API, prior decisions, tests, business logic). Read-only — generates no files. Supports `detail_level`, `max_tokens`, `scan_path`, `max_depth` |
| `sdd_bootstrap` | Write SDD artifacts (`requirements.md`, `business-rules.md`, `design.md`) from AI-generated content — no pipeline guards. Only generates missing artifacts. Auto-marks output with `Auto-generated` header for review |

## Standalone (4 tools)

Tools that work without an active pipeline or `hoofy.json`. Useful for ad-hoc sessions, quick context gathering, spec-aware code reviews, and spec-vs-code auditing.

| Tool | Description |
|---|---|
| `sdd_explore` | Pre-pipeline context capture — saves goals, constraints, tech preferences, unknowns, and decisions to memory. Upserts via topic key (call multiple times as thinking evolves). Suggests change type/size based on keywords. Use before `sdd_change` or `sdd_init_project` |
| `sdd_suggest_context` | Recommend relevant specs, memory observations, and completed changes for a task description. Scans artifacts, completed changes, memory, and conventions. Returns a prioritized, actionable list of context to read. Supports `detail_level`, `max_tokens`, `project_name` |
| `sdd_review` | Generate a spec-aware code review checklist for a change. Parses requirements (FR-XXX), business rules (BRC-XXX constraints), design decisions, and ADRs from memory. Returns verification items that reference specific spec IDs. Supports `detail_level`, `max_tokens`, `project_name` |
| `sdd_audit` | Compare specifications against actual source code and report discrepancies: missing implementations, stale specs, and inconsistencies. Read-only scanner — produces a structured report for the AI to analyze. Works standalone without an active pipeline |

## Project Pipeline (10 tools)

Full greenfield specification — from vague idea to validated architecture. 9 sequential stages with principles declaration, business rules extraction, and the Clarity Gate. Artifacts stored in `docs/`.

| Tool | Stage | Description |
|---|---|---|
| `sdd_init_project` | Init | Initialize project structure (`docs/` directory, `hoofy.json`). Auto-generates an SDD section in `CLAUDE.md`/`AGENTS.md` (idempotent) |
| `sdd_create_principles` | Principles | Capture golden invariants — project principles, coding standards, and domain truths that anchor all subsequent stages |
| `sdd_create_charter` | Charter | Save project charter — enterprise-grade project definition with domain context, stakeholders, vision, boundaries, success criteria, existing systems, and constraints. Four required + six optional fields |
| `sdd_generate_requirements` | Specify | Save formal requirements with MoSCoW prioritization (Must/Should/Could/Won't Have + Non-Functional) |
| `sdd_create_business_rules` | Business Rules | Extract declarative business rules from requirements using BRG taxonomy (Definitions, Facts, Constraints, Derivations) and DDD Ubiquitous Language |
| `sdd_clarify` | Clarify | Run the Clarity Gate — 8-dimension ambiguity analysis. Blocks until score meets threshold (guided: 70, expert: 50) |
| `sdd_create_design` | Design | Save technical architecture (components, data model, APIs, security, infrastructure, structural quality analysis) |
| `sdd_create_tasks` | Tasks | Save implementation task breakdown with dependency graph and optional wave assignments for parallel execution |
| `sdd_validate` | Validate | Cross-artifact consistency check (requirements <-> design <-> tasks). Includes structural quality verification |
| `sdd_get_context` | — | View project state, pipeline status, and stage artifacts. Supports `detail_level`, `max_tokens` |

### Pipeline Order

```
init → principles → charter → specify → business-rules → clarify → design → tasks → validate
```

## Prompts (6 on-demand guides)

Detailed guidance loaded on-demand to reduce base instruction size. The AI requests the right prompt when it needs workflow-specific instructions.

| Prompt | Description |
|---|---|
| `/sdd-start` | Start a new SDD project (guided conversation) |
| `/sdd-status` | Check current pipeline status |
| `/sdd-stage-guide` | Detailed instructions for the current pipeline stage (how to generate content, what to check) |
| `/sdd-memory-guide` | Best practices for memory operations (when to save, search patterns, topic keys, relations) |
| `/sdd-change-guide` | Complete guide for the change pipeline (all 12 flow variants, stage descriptions, artifact guards) |
| `/sdd-bootstrap-guide` | Instructions for bootstrapping existing projects (reverse-engineer -> analyze -> bootstrap workflow) |
