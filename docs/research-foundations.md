# Research Foundations

Every Hoofy feature is grounded in published research. This document maps each capability to the specific research that informed it — what it recommends, and how Hoofy implements it.

## Anthropic Engineering

### [Building Effective Agents](https://www.anthropic.com/engineering/building-effective-agents) (Dec 2024)

Foundational patterns for agent design. Distinguishes workflows from agents, introduces the concept of Agent-Computer Interface (ACI), and establishes that tool design matters as much as prompt design.

| Recommendation | Hoofy Implementation |
|---|---|
| "Agent-Computer Interface (ACI) is as important as HCI" — tool descriptions and parameters are critical for AI usability | All 38 tools use consistent `sdd_*` and `mem_*` namespacing with self-documenting parameter descriptions |
| "Do the simplest thing that works" — avoid over-engineering agent systems | Adaptive change pipeline selects only the stages needed (4-7 stages based on type x size), instead of forcing a one-size-fits-all workflow |
| Orchestrator-worker pattern for complex tasks | Project pipeline uses sequential orchestration: propose → specify → clarify → design → tasks → validate |
| Evaluator-optimizer pattern for iterative refinement | Clarity Gate blocks pipeline advancement until clarity score meets threshold, forcing iterative requirement refinement |

### [Effective Context Engineering for AI Agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) (Sep 2025)

The most relevant article for Hoofy's memory system. Defines context as a finite resource with diminishing marginal returns, and presents strategies for managing it.

| Recommendation | Hoofy Implementation |
|---|---|
| "Structured note-taking / agentic memory" — agent writes notes persisted outside the context window, pulls them back later | `mem_save` persists observations to SQLite with FTS5 full-text search. `mem_context` and `mem_search` retrieve them in future sessions |
| "Progressive disclosure" — agents discover context layer by layer, keeping only what's necessary | `mem_search` → `mem_timeline` → `mem_get_observation` pattern: search first, drill into timeline, then read full content |
| "Sub-agent architectures" — specialized sub-agents with clean context windows, return condensed summaries | Knowledge graph with `mem_build_context` traverses relations from any observation. `namespace` parameter on 7 memory tools (`mem_save`, `mem_save_prompt`, `mem_session_summary`, `mem_progress`, `mem_search`, `mem_context`, `mem_compact`) enables opt-in isolation — each sub-agent tags observations with its namespace, reads only its own notes, while the orchestrator omits namespace to see everything |
| "Hybrid strategy" — some data retrieved up front, other data explored just-in-time | `mem_context` loads recent history at session start (up front). `mem_search` retrieves specific memories on demand (just-in-time) |
| "Context is a finite resource" — treat it like an attention budget | 5 read-heavy tools support `detail_level: summary | standard | full` to control response verbosity: `sdd_get_context`, `mem_context`, `mem_search`, `mem_timeline`, `sdd_context_check`. Summary-mode responses include a footer hint for progressive disclosure. `sdd_get_context` defaults to `summary` (minimal pipeline overview) to save tokens on the most frequently called pipeline tool |
| "You have to be smart about managing what goes into context" — stale and redundant data degrades performance over time | `mem_compact` identifies stale observations (older than N days) and batch soft-deletes them. Optionally creates a "compaction_summary" observation to preserve key knowledge. Two-step workflow: identify candidates → review → compact with summary |
| "Context is a finite resource" — token budgets must be managed explicitly, not just by verbosity level | 5 read-heavy tools (`mem_context`, `mem_search`, `mem_timeline`, `sdd_get_context`, `sdd_context_check`) accept `max_tokens` to hard-cap response size. Token estimation uses `len(text)/4` heuristic (O(1), no tokenizer dependency). Every response includes a `📏 ~N tokens` footer. Budget-capped responses prepend `⚡ Budget-capped` notice. Complementary to `detail_level` — one controls content type, the other controls total output size |

### [Writing Effective Tools for Agents — with Agents](https://www.anthropic.com/engineering/writing-tools-for-agents) (Sep 2025)

Direct guidance on tool design for AI agents. Covers namespacing, consolidation, response format, truncation, and token efficiency.

| Recommendation | Hoofy Implementation |
|---|---|
| "Namespacing tools with prefixes helps delineate boundaries" | `mem_*` (19 memory tools), `sdd_*` (11 project pipeline + 2 bootstrap tools), `sdd_change*` (5 change tools), standalone `sdd_explore`, `sdd_suggest_context`, `sdd_review` — clear boundaries between systems |
| "Return only high-signal information, avoid cryptic UUIDs" | Tool responses include human-readable summaries, not raw database rows. `detail_level` parameter lets the AI request only the verbosity needed |
| "Tools should be self-contained, robust to error, extremely clear" | Each tool has comprehensive parameter descriptions with examples in the tool definition |
| "Truncate tool responses, but always include total counts" | `mem_search`, `mem_context`, and `mem_timeline` append navigation hints ("📊 Showing X of Y") when results are capped by limit. `NavigationHint()` returns empty string when all results are shown (no noise) |

### [How We Built Our Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system) (Jun 2025)

Architecture lessons from Anthropic's multi-agent Research feature. Key insights on token efficiency, orchestration, and memory management.

| Recommendation | Hoofy Implementation |
|---|---|
| "Long-horizon conversation management: agents summarize completed phases, store in external memory" | `mem_session_summary` captures structured summaries (Goal, Discoveries, Accomplished, Files) at session end for future sessions |
| "Subagents output to filesystem to minimize 'game of telephone'" | All pipeline artifacts are written to `sdd/*.md` files on disk, not passed through conversation history |
| "Each sub-agent works independently with its own context" — parallel agents need memory isolation | `namespace` parameter provides opt-in memory scoping. Sub-agents tag observations with `namespace="subagent/<task-id>"`, reads filter by namespace. Orchestrator omits namespace to see all. Convention: `subagent/<task-id>` or `agent/<role>` |
| "Token usage explains 80% of performance variance" — more tokens does not equal better results | Topic key upsert (`mem_save` with `topic_key`) prevents memory duplication. One observation per topic, always current |

### [Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) (Nov 2025)

Solutions for agents that work across multiple context windows. Introduces the initializer agent pattern, incremental progress, and structured handoffs.

| Recommendation | Hoofy Implementation |
|---|---|
| "Each session: read progress, read git log, run basic test, then start new work" | `mem_progress` persists structured JSON progress docs that survive context compaction. Auto-read at session start, upserted during work. One active progress per project via topic_key. `mem_context` provides recent observations for broader session context |
| "Feature list in JSON (not Markdown) — model less likely to inappropriately change JSON" | Pipeline state persisted in `sdd/sdd.json` (JSON), not markdown. `mem_progress` content is validated JSON — the model is less likely to corrupt structured data than free-form markdown |
| "Agent commits to git with descriptive messages after each feature" | Change pipeline enforces incremental delivery: one active change at a time, verify stage before completion |
| "Initializer agent sets up environment on first run" | `sdd_init_project` creates the `sdd/` directory structure, `sdd.json` config, and templates — environment scaffolding before any work begins |

### [Claude Code: Best Practices for Agentic Coding](https://www.anthropic.com/engineering/claude-code-best-practices) (Apr 2025)

Best practices for getting the most out of AI coding assistants. Covers CLAUDE.md, custom instructions, and structured workflows.

| Recommendation | Hoofy Implementation |
|---|---|
| Use CLAUDE.md for persistent project context | Context-check stage scans `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md` and other convention files for conflicts with the current change |
| Structure specifications before coding | Full greenfield pipeline (propose → specify → business rules → clarity gate → design → tasks → validate) enforces specs before any code is written |

---

## Academic Research

### [Codified Context: Infrastructure for AI Agents in a Complex Codebase](https://arxiv.org/abs/2602.20478v1) (Lulla 2026)

Empirical analysis of meta-infrastructure (AGENTS.md, custom instructions, codified context) for AI coding agents in production codebases. Studies 6,088 SWE tasks and shows that codified context is a first-class engineering artifact, not just documentation.

| Finding | Hoofy Implementation |
|---|---|
| AGENTS.md associated with **29% less runtime** and **17% less token consumption** | Hoofy's `AGENTS.md` is actively scanned by `sdd_context_check` and `sdd_suggest_context` — codified context is used as input, not just documentation |
| Compact constitutions (~660 lines) outperform monolithic instructions | Server instructions reduced from 733 lines to ~160 lines. Detailed guidance moved to 6 on-demand MCP prompts (`/sdd-stage-guide`, `/sdd-memory-guide`, `/sdd-change-guide`, `/sdd-bootstrap-guide`) loaded only when needed |
| 80%+ of agent prompts are ≤100 words — short, focused interactions dominate | `sdd_suggest_context` is designed for short "what should I read?" queries. `sdd_review` takes a brief change description and returns a structured checklist |
| **4.3% overhead** for meta-infrastructure (context files) — small cost for significant gains | SDD artifacts (`sdd/*.md`) and convention files add minimal overhead while preventing hallucinations and rework |
| **24.2% knowledge-to-code ratio** — nearly 1/4 of repo content is context/documentation | Hoofy's pipeline generates specs, business rules, design docs, and task breakdowns as first-class artifacts alongside code |
| Ad-hoc review was more used than formal review stages | `sdd_review` is a standalone tool, not a pipeline stage — can be used at any time without starting a change flow (ADR captured for this decision) |

---

## Industry Research

### Requirements Engineering & Specification

| Source | What it says | Hoofy Implementation |
|---|---|---|
| [METR 2025](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) | Experienced developers were 19% slower with unstructured AI despite feeling 20% faster | Hoofy enforces structured specification — the AI cannot skip specs for non-trivial changes |
| [DORA 2025](https://dora.dev/research/2025/dora-report/) | 7.2% delivery instability increase for every 25% AI adoption without foundational practices | Pipeline stages (context-check, clarity gate, verify) provide the foundational practices DORA identifies as missing |
| [McKinsey 2025](https://www.mckinsey.com/capabilities/mckinsey-digital/our-insights/superagency-in-the-workplace-empowering-people-to-unlock-ais-full-potential-at-work) | Top performers see 16-30% productivity gains only with structured specification and communication | SDD pipeline is structured specification and communication — proposal, requirements, design, tasks |
| [IEEE 720574](https://ieeexplore.ieee.org/document/720574) | Fixing a requirement error in production costs 10-100x more than during requirements phase | Clarity Gate catches ambiguities in the requirements phase, before any code is written |
| IREB & [IEEE 29148](https://www.iso.org/standard/72089.html) | Industry standards for structured requirements elicitation and traceability | Server instructions implement IEEE 29148 Requirements Smells heuristics for the AI to follow during specification |
| [Business Rules Group](https://www.businessrulesgroup.org/brmanifesto.htm) | Business Rules Manifesto — rules are first-class citizens, not buried in code | Business-rules stage uses BRG taxonomy (Definitions, Facts, Constraints, Derivations) to extract declarative rules from requirements |
| [EARS](https://alistairmavin.com/ears/) | Easy Approach to Requirements Syntax — sentence templates that eliminate ambiguity | Server instructions use EARS patterns (When/While/Where/If-Then) for the AI to follow when writing requirements |
| [DDD Ubiquitous Language](https://martinfowler.com/bliki/UbiquitousLanguage.html) | A shared language eliminates translation errors between business and technical domains | Business-rules stage builds a glossary as part of the Ubiquitous Language, used across all pipeline artifacts |

---

*This document is updated as new features are added. Every feature must cite its research source before shipping.*
