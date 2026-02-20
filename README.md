# SDD-Hoffy

**Spec-Driven Development for the AI era.** An MCP server that guides you from a vague idea to clear, actionable specifications — so your AI coder builds what you actually want.

[![CI](https://github.com/HendryAvila/sdd-hoffy/actions/workflows/ci.yml/badge.svg)](https://github.com/HendryAvila/sdd-hoffy/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-Compatible-purple)](https://modelcontextprotocol.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/HendryAvila/sdd-hoffy?include_prereleases)](https://github.com/HendryAvila/sdd-hoffy/releases)

---

## The Problem

AI coding tools are powerful but they hallucinate. A lot.

The METR 2025 study found that developers are [19% slower with AI](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) despite _feeling_ 20% faster. The DORA 2025 State of DevOps Report shows a [7.2% delivery instability increase](https://dora.dev/research/2025/dora-report/) for every 25% AI adoption — without foundational systems. McKinsey 2025 reports that top performers see [16-30% productivity gains](https://www.mckinsey.com/capabilities/mckinsey-digital/our-insights/superagency-in-the-workplace-empowering-people-to-unlock-ais-full-potential-at-work) only when they communicate structured requirements.

The pattern is clear: **AI without clear specifications makes things worse, not better.**

The root cause? Ambiguity. When you tell an AI "build me an app", it fills in the blanks with assumptions. Those assumptions are hallucinations.

## The Solution: Spec-Driven Development

SDD-Hoffy solves this with a complete 7-stage pipeline:

```
Vague Idea → Proposal → Requirements → Clarity Gate → Architecture → Tasks → Validation → Ready for AI
```

The **Clarity Gate** is the core innovation. It analyzes your requirements across 8 dimensions and blocks progress until ambiguities are resolved. The AI can't skip ahead to architecture until your specs are clear enough.

After the gate, SDD-Hoffy continues into **technical design**, **atomic task breakdown**, and a **cross-artifact validation** that catches inconsistencies before a single line of code is written.

---

## SDD-Hoffy vs Plan Mode

If you use Cursor, Claude Code, Codex, or similar tools, you've probably used `/plan mode` or something like it. So what's the difference?

**They operate at different layers:**

```
┌─────────────────────────────────────────────────────────┐
│                   WHAT to build                         │
│                                                         │
│  SDD-Hoffy (Requirements Engineering Layer)             │
│  ─────────────────────────────────────────               │
│  "WHO are the users? WHAT must the system do?           │
│   What's out of scope? What are the NFRs?               │
│   What architecture addresses these requirements?       │
│   What tasks cover the full design?"                    │
│                                                         │
├─────────────────────────────────────────────────────────┤
│                   HOW to build it                       │
│                                                         │
│  /plan mode (Implementation Layer)                      │
│  ─────────────────────────────────                      │
│  "What files do I create? What functions do I write?    │
│   What tests do I need? What's the PR structure?"       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**SDD-Hoffy is the architect. Plan mode is the contractor.**

An architect decides _what_ the building needs: load-bearing walls, plumbing routes, electrical capacity, fire exits. A contractor decides _how_ to build it: which tools, what order, which crew. You wouldn't hire a contractor without blueprints — and you shouldn't use plan mode without specifications.

### What each one does

| Concern | SDD-Hoffy | Plan Mode |
|---|---|---|
| **"Who are the users?"** | Defines 2-3 personas with needs | Doesn't ask |
| **"What's out of scope?"** | Explicit exclusion list | Doesn't track |
| **"Is this requirement ambiguous?"** | Clarity Gate with 8-dimension scoring | No check |
| **"What architecture fits?"** | Full design doc with ADRs and rationale | Infers from code |
| **"Are all requirements covered?"** | Cross-artifact validation report | No traceability |
| **"What files should I change?"** | Not its job | This is what it does |
| **"What's the implementation order?"** | Not its job | This is what it does |

### The workflow together

```
1. Run SDD-Hoffy    →  Produce: proposal.md, requirements.md, design.md, tasks.md
2. Open plan mode   →  Feed it the SDD artifacts as context
3. Execute          →  AI implements with clear specs, minimal hallucination
```

SDD-Hoffy doesn't replace plan mode. It makes plan mode **dramatically better** by giving it unambiguous input.

---

## What Makes SDD-Hoffy Different

| Feature | SDD-Hoffy | Traditional CLI Tools |
|---|---|---|
| **Interface** | MCP server (works in ANY AI tool) | Standalone CLI |
| **LLM config** | Zero — your AI tool handles it | API keys, env vars |
| **Compatibility** | Claude Code, Gemini CLI, Codex, Cursor, VS Code Copilot, OpenCode | Single tool only |
| **Install** | One-liner install + auto-update | npm/pip, dependencies |
| **Dual mode** | Guided (beginners) + Expert (devs) | One size fits all |
| **Full pipeline** | Idea → Specs → Architecture → Tasks → Validation | Specs only |

---

## Quick Start

### 1. Install

**One-liner (recommended):**

```bash
curl -sSL https://raw.githubusercontent.com/HendryAvila/sdd-hoffy/main/install.sh | bash
```

This detects your OS/architecture, downloads the latest binary, and installs it — no Go required.

<details>
<summary><strong>Other installation methods</strong></summary>

```bash
# Go install (requires Go 1.25+)
go install github.com/HendryAvila/sdd-hoffy/cmd/sdd-hoffy@latest

# Build from source
git clone https://github.com/HendryAvila/sdd-hoffy.git
cd sdd-hoffy
make build

# Manual download (pick your platform)
# https://github.com/HendryAvila/sdd-hoffy/releases
```
</details>

### 2. Configure your AI tool

Add SDD-Hoffy to your tool's MCP configuration:

<details>
<summary><strong>Claude Code</strong></summary>

Add to `.claude/settings.json` or project settings:
```json
{
  "mcpServers": {
    "sdd-hoffy": {
      "command": "sdd-hoffy",
      "args": ["serve"]
    }
  }
}
```
</details>

<details>
<summary><strong>VS Code Copilot</strong></summary>

Add to `.vscode/mcp.json`:
```json
{
  "servers": {
    "sdd-hoffy": {
      "type": "stdio",
      "command": "sdd-hoffy",
      "args": ["serve"]
    }
  }
}
```
</details>

<details>
<summary><strong>Cursor</strong></summary>

Add to your MCP configuration:
```json
{
  "mcpServers": {
    "sdd-hoffy": {
      "command": "sdd-hoffy",
      "args": ["serve"]
    }
  }
}
```
</details>

<details>
<summary><strong>OpenCode</strong></summary>

Add to `~/.config/opencode/opencode.json` (inside the `"mcp"` key):
```json
{
  "mcp": {
    "sdd-hoffy": {
      "type": "local",
      "command": ["sdd-hoffy", "serve"],
      "enabled": true
    }
  }
}
```
</details>

<details>
<summary><strong>Gemini CLI</strong></summary>

Add to your MCP settings:
```json
{
  "mcpServers": {
    "sdd-hoffy": {
      "command": "sdd-hoffy",
      "args": ["serve"]
    }
  }
}
```
</details>

### 3. Use it

Just talk to your AI tool:

> "I want to start a new SDD project for a task management app"

The AI will use SDD-Hoffy's tools to guide you through the full pipeline. Or use the prompt directly:

> `/sdd-start`

### 4. Keep it updated

SDD-Hoffy checks for updates automatically when you start the server and shows a non-intrusive notice if a new version is available. To update:

```bash
sdd-hoffy update
```

No auto-restarts, no surprises — you decide when to update.

### 5. Make it proactive (optional but recommended)

SDD-Hoffy already includes built-in instructions that tell your AI when to suggest using the SDD pipeline. But if you want to **reinforce** this behavior, add the following to your project's agent instructions file:

<details>
<summary><strong>Claude Code</strong> — add to <code>CLAUDE.md</code></summary>

```markdown
## Spec-Driven Development

When the user asks to build a new project, add a feature, or design something,
ALWAYS suggest using SDD-Hoffy first to create clear specs before writing code.
Use the /sdd-start prompt or call sdd_init_project to begin the pipeline.
Do NOT start coding without specs for any non-trivial change.
```
</details>

<details>
<summary><strong>Cursor</strong> — add to <code>.cursor/rules/sdd.md</code></summary>

```markdown
## Spec-Driven Development

When the user asks to build a new project, add a feature, or design something,
ALWAYS suggest using SDD-Hoffy first to create clear specs before writing code.
Use the /sdd-start prompt or call sdd_init_project to begin the pipeline.
Do NOT start coding without specs for any non-trivial change.
```
</details>

<details>
<summary><strong>OpenCode</strong> — add to <code>AGENTS.md</code></summary>

```markdown
## Spec-Driven Development

When the user asks to build a new project, add a feature, or design something,
ALWAYS suggest using SDD-Hoffy first to create clear specs before writing code.
Use the /sdd-start prompt or call sdd_init_project to begin the pipeline.
Do NOT start coding without specs for any non-trivial change.
```
</details>

<details>
<summary><strong>VS Code Copilot</strong> — add to <code>.github/copilot-instructions.md</code></summary>

```markdown
## Spec-Driven Development

When the user asks to build a new project, add a feature, or design something,
ALWAYS suggest using SDD-Hoffy first to create clear specs before writing code.
Use the /sdd-start prompt or call sdd_init_project to begin the pipeline.
Do NOT start coding without specs for any non-trivial change.
```
</details>

<details>
<summary><strong>Gemini CLI</strong> — add to <code>GEMINI.md</code></summary>

```markdown
## Spec-Driven Development

When the user asks to build a new project, add a feature, or design something,
ALWAYS suggest using SDD-Hoffy first to create clear specs before writing code.
Use the /sdd-start prompt or call sdd_init_project to begin the pipeline.
Do NOT start coding without specs for any non-trivial change.
```
</details>

---

## The SDD Pipeline

SDD-Hoffy follows a sequential 7-stage pipeline. Each stage builds on the previous one, and you can't skip ahead until the current stage is complete.

```
┌──────────────────────────────────────────────────────────┐
│                     SDD Pipeline                          │
│                                                           │
│  ┌──────┐   ┌─────────┐   ┌─────────┐   ┌────────────┐  │
│  │ INIT │──▶│ PROPOSE │──▶│ SPECIFY │──▶│  CLARIFY   │  │
│  └──────┘   └─────────┘   └─────────┘   └─────┬──────┘  │
│                                                │          │
│                                        ┌───────┘          │
│                                        ▼                  │
│                                 ┌────────────┐            │
│                                 │Clarity Gate│            │
│                                 │Score >= 70?│            │
│                                 └──────┬─────┘            │
│                               No │     │ Yes              │
│                                  │     ▼                  │
│                               ◀──┘  ┌────────┐           │
│                                     │ DESIGN │           │
│                                     └────┬───┘           │
│                                          ▼               │
│                                     ┌────────┐           │
│                                     │ TASKS  │           │
│                                     └────┬───┘           │
│                                          ▼               │
│                                     ┌──────────┐         │
│                                     │ VALIDATE │         │
│                                     └──────────┘         │
│                                          │               │
│                                          ▼               │
│                                  Ready for /plan         │
│                                  mode & coding           │
└──────────────────────────────────────────────────────────┘
```

### Stage 0: Init
Set up the SDD project structure. Creates a `sdd/` directory with configuration.

### Stage 1: Propose
Transform your vague idea into a structured proposal with:
- **Problem Statement** — What pain point does this solve?
- **Target Users** — Who specifically will use this?
- **Proposed Solution** — What are we building? (high level, no tech details)
- **Out of Scope** — What this project does NOT do
- **Success Criteria** — How do we know it worked?
- **Open Questions** — Things we're still unsure about

### Stage 2: Specify
Extract formal requirements from the proposal using [MoSCoW prioritization](https://en.wikipedia.org/wiki/MoSCoW_method):
- **Must Have** — Non-negotiable for launch
- **Should Have** — Important but not blocking
- **Could Have** — Nice to have, can wait
- **Won't Have** — Explicitly excluded from this version

Each requirement gets a unique ID (FR-001, NFR-001) for traceability.

### Stage 3: Clarify (The Clarity Gate)
The core innovation. SDD-Hoffy analyzes your requirements across **8 dimensions**:

| Dimension | What it checks |
|---|---|
| Target Users | Are user personas clearly defined? |
| Core Functionality | Are main features unambiguous? |
| Data Model | Are entities and relationships clear? |
| Integrations | Are external system interfaces defined? |
| Edge Cases | Are error scenarios addressed? |
| Security | Are auth and data protection requirements clear? |
| Scale & Performance | Are performance expectations defined? |
| Scope Boundaries | Is it clear what the system does NOT do? |

The pipeline **blocks** until the clarity score meets the threshold:
- **Guided mode:** 70/100 (more thoroughness for non-technical users)
- **Expert mode:** 50/100 (developers can handle more ambiguity)

### Stage 4: Design
Create a technical architecture document grounded in your validated requirements:
- **Architecture Overview** — Pattern choice (monolith, microservices, serverless) with rationale
- **Tech Stack** — Every technology choice justified against requirements
- **Components** — Modules with responsibilities, boundaries, and requirement traceability (FR-XXX)
- **API Contracts** — Endpoint definitions, schemas, error codes
- **Data Model** — Schema, relationships, constraints, indexes
- **Infrastructure** — Deployment strategy, CI/CD, environments
- **Security** — Auth strategy, encryption, rate limiting (addressing NFRs)
- **Design Decisions** — ADRs with alternatives considered and why they were rejected

### Stage 5: Tasks
Break the design into atomic, AI-ready implementation tasks:
- **Each task** has a unique ID (TASK-001), maps to requirements (FR-XXX), identifies affected components, lists dependencies, and includes acceptance criteria
- **Dependency graph** shows what can be parallelized and what's sequential
- **Effort estimate** for the full implementation
- **Global acceptance criteria** — project-wide quality gates (coverage, linting, testing)

### Stage 6: Validate
Cross-artifact consistency check — the final quality gate:
- **Requirements coverage** — Is every FR-XXX/NFR-XXX covered by at least one task?
- **Component coverage** — Does every component in the design have tasks assigned?
- **Consistency check** — Do tasks match the design? Does the design match the requirements?
- **Risk assessment** — What could go wrong during implementation?
- **Verdict** — PASS, PASS_WITH_WARNINGS, or FAIL (with actionable recommendations)

If validation fails, SDD-Hoffy tells you exactly which stage to revisit and what to fix.

---

## Dual Mode

SDD-Hoffy adapts to your experience level:

### Guided Mode (for vibe coders and non-technical users)
- Step-by-step instructions with examples
- More clarifying questions
- Simpler language, no jargon
- Higher clarity threshold (70/100)
- Templates include example content

### Expert Mode (for experienced developers)
- Streamlined flow, fewer questions
- Accepts technical input directly
- Lower clarity threshold (50/100)
- Lean templates, just section headers

Set the mode when initializing:
```
sdd_init_project(name: "my-app", description: "...", mode: "guided")
```

---

## Available Tools

| Tool | Stage | Description |
|---|---|---|
| `sdd_init_project` | 0 | Initialize SDD project structure |
| `sdd_create_proposal` | 1 | Save a structured proposal from your idea |
| `sdd_generate_requirements` | 2 | Save formal requirements from the proposal |
| `sdd_clarify` | 3 | Run the Clarity Gate analysis |
| `sdd_create_design` | 4 | Save the technical architecture document |
| `sdd_create_tasks` | 5 | Save the atomic implementation task breakdown |
| `sdd_validate` | 6 | Run cross-artifact consistency check |
| `sdd_get_context` | Any | View current project state and artifacts |

## Available Prompts

| Prompt | Description |
|---|---|
| `/sdd-start` | Start a new SDD project (guided workflow) |
| `/sdd-status` | Check current pipeline status |

---

## What Gets Generated

SDD-Hoffy creates a `sdd/` directory in your project:

```
sdd/
├── sdd.json              # Project config (mode, stage, clarity score)
├── proposal.md           # Stage 1: Structured proposal
├── requirements.md       # Stage 2: Formal requirements with MoSCoW IDs
├── clarifications.md     # Stage 3: Clarity Gate Q&A log
├── design.md             # Stage 4: Technical architecture & ADRs
├── tasks.md              # Stage 5: Atomic implementation tasks
├── validation.md         # Stage 6: Cross-artifact consistency report
└── history/              # Archived completed changes
```

These files are designed to be:
- **Human-readable** — Clear markdown that anyone can understand
- **AI-consumable** — Structured enough that AI tools can parse and follow them
- **Version-controllable** — Plain text files that live in your repo
- **Feedable to plan mode** — Drop them as context into any AI coding tool

---

## The Research Behind SDD

SDD-Hoffy isn't built on opinions. It's built on research:

- **METR 2025**: In a randomized controlled trial, experienced developers were [19% slower with AI](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/) despite feeling 20% faster — because unstructured AI usage introduces debugging overhead and false confidence.

- **DORA 2025 State of DevOps**: Organizations see a [7.2% increase in delivery instability](https://dora.dev/research/2025/dora-report/) for every 25% increase in AI adoption — when adopted without foundational systems and practices.

- **McKinsey 2025**: Top-performing teams achieve [16-30% productivity gains with AI](https://www.mckinsey.com/capabilities/mckinsey-digital/our-insights/superagency-in-the-workplace-empowering-people-to-unlock-ais-full-potential-at-work) — but only when they invest in structured specification and communication.

- **Requirements Engineering (IEEE)**: The cost of fixing a requirement error found in production is [10-100x more expensive](https://ieeexplore.ieee.org/document/720574) than fixing it during the requirements phase. This multiplier is even worse with AI-generated code, where hallucinated requirements propagate through entire codebases.

The common thread: **structure beats speed**. SDD-Hoffy forces that structure before a single line of code is written.

---

## Contributing

SDD-Hoffy is open source and we welcome contributions!

### Development

```bash
# Clone the repo
git clone https://github.com/HendryAvila/sdd-hoffy.git
cd sdd-hoffy

# Build
make build

# Run
./bin/sdd-hoffy serve

# Run tests
make test

# Lint
make lint
```

### Areas for Contribution

- **More clarity dimensions** — Domain-specific dimensions (mobile, API, data pipeline, etc.)
- **Template improvements** — Better guided mode templates and examples
- **Host-specific guides** — Documentation for configuring in specific tools
- **Streamable HTTP transport** — Remote server deployment support
- **Template customization** — Bring your own templates
- **Project presets** — Web app, API, CLI, mobile starter configs
- **Export formats** — Jira, Linear, GitHub Issues integration
- **i18n** — Support for non-English specs
- **Tests** — Unit and integration tests

---

## Roadmap

- [x] Full 7-stage pipeline: Init, Propose, Specify, Clarify, Design, Tasks, Validate
- [x] Dual mode (Guided/Expert)
- [x] MCP server with stdio transport
- [x] CI/CD pipeline (GitHub Actions + GoReleaser)
- [x] Multi-platform binary releases (Linux, macOS, Windows)
- [x] Cross-artifact validation with PASS/WARN/FAIL verdicts
- [x] One-liner install script
- [x] Self-update system (`sdd-hoffy update`)
- [ ] Streamable HTTP transport (for remote server deployment)
- [ ] Template customization (bring your own templates)
- [ ] Project presets (web app, API, CLI, mobile, etc.)
- [ ] Export to external formats (Jira, Linear, GitHub Issues)

---

## License

[MIT](LICENSE) — Use it, fork it, build on it. That's what open source is for.

---

<p align="center">
  <strong>Stop prompting. Start specifying.</strong><br>
  Built with care by the SDD-Hoffy community.
</p>
