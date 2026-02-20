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

The METR 2025 study found that developers are [19% slower with AI](https://metr.org/blog/2025-07-10-early-2025-ai-developer-study/) despite _feeling_ 20% faster. The DORA 2025 State of DevOps Report shows a [7.2% delivery instability increase](https://dora.dev/research/) for every 25% AI adoption — without foundational systems. McKinsey 2025 reports that top performers see [16-30% productivity gains](https://www.mckinsey.com/capabilities/mckinsey-digital/our-insights/superagency-in-the-workplace-empowering-people-to-unlock-ais-full-potential-at-work) only when they communicate structured requirements.

The pattern is clear: **AI without clear specifications makes things worse, not better.**

The root cause? Ambiguity. When you tell an AI "build me an app", it fills in the blanks with assumptions. Those assumptions are hallucinations.

## The Solution: Spec-Driven Development

SDD-Hoffy solves this with a simple pipeline:

```
Vague Idea → Structured Proposal → Formal Requirements → Clarity Gate → Ready for AI
```

The **Clarity Gate** is the core innovation. It's a quality check that analyzes your requirements across 8 dimensions and blocks progress until ambiguities are resolved. The AI can't skip ahead to coding until your specs are clear enough.

## What Makes SDD-Hoffy Different

| Feature | SDD-Hoffy | Traditional CLI Tools |
|---|---|---|
| **Interface** | MCP server (works in ANY AI tool) | Standalone CLI |
| **LLM config** | Zero — your AI tool handles it | API keys, env vars |
| **Compatibility** | Claude Code, Gemini CLI, Codex, Cursor, VS Code Copilot, OpenCode | Single tool only |
| **Install** | One binary, 3 lines of config | npm/pip, dependencies |
| **Dual mode** | Guided (beginners) + Expert (devs) | One size fits all |

---

## Quick Start

### 1. Install

```bash
# Option A: Go install (requires Go 1.25+)
go install github.com/HendryAvila/sdd-hoffy/cmd/sdd-hoffy@latest

# Option B: Download binary (no Go required)
# Download the latest release for your platform from:
# https://github.com/HendryAvila/sdd-hoffy/releases
#
# Linux (amd64):
curl -Lo sdd-hoffy.tar.gz https://github.com/HendryAvila/sdd-hoffy/releases/latest/download/sdd-hoffy_linux_amd64.tar.gz
tar xzf sdd-hoffy.tar.gz
sudo mv sdd-hoffy /usr/local/bin/

# macOS (Apple Silicon):
curl -Lo sdd-hoffy.tar.gz https://github.com/HendryAvila/sdd-hoffy/releases/latest/download/sdd-hoffy_darwin_arm64.tar.gz
tar xzf sdd-hoffy.tar.gz
sudo mv sdd-hoffy /usr/local/bin/

# Option C: Build from source
git clone https://github.com/HendryAvila/sdd-hoffy.git
cd sdd-hoffy
make build
```

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

Add to `.opencode.json`:
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

The AI will use SDD-Hoffy's tools to guide you through the pipeline. Or use the prompt directly:

> `/sdd-start`

---

## The SDD Pipeline

SDD-Hoffy follows a sequential pipeline. Each stage builds on the previous one, and you can't skip ahead until the current stage is complete.

```
┌─────────────────────────────────────────────────────┐
│                    SDD Pipeline                      │
│                                                      │
│  ┌──────┐   ┌─────────┐   ┌─────────┐   ┌────────┐ │
│  │ INIT │──▶│ PROPOSE │──▶│ SPECIFY │──▶│CLARIFY │ │
│  └──────┘   └─────────┘   └─────────┘   └────────┘ │
│                                             │        │
│                                     ┌───────┘        │
│                                     ▼                │
│                              ┌────────────┐          │
│                              │Clarity Gate│          │
│                              │Score >= 70?│          │
│                              └──────┬─────┘          │
│                            No │     │ Yes            │
│                               │     ▼                │
│                            ◀──┘  ┌────────┐          │
│                                  │ DESIGN │ (v2)     │
│                                  └────┬───┘          │
│                                       ▼              │
│                                  ┌────────┐          │
│                                  │ TASKS  │ (v2)     │
│                                  └────┬───┘          │
│                                       ▼              │
│                                  ┌──────────┐        │
│                                  │ VALIDATE │ (v2)   │
│                                  └──────────┘        │
└─────────────────────────────────────────────────────┘
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
This is where the magic happens. SDD-Hoffy analyzes your requirements across **8 dimensions**:

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

### Stages 4-6: Design, Tasks, Validate (Coming in v2)
Technical architecture, atomic task breakdown, and cross-artifact consistency checks.

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

| Tool | Description |
|---|---|
| `sdd_init_project` | Initialize SDD project structure |
| `sdd_create_proposal` | Generate a structured proposal from your idea |
| `sdd_generate_requirements` | Extract formal requirements from the proposal |
| `sdd_clarify` | Run the Clarity Gate analysis |
| `sdd_get_context` | View current project state and artifacts |

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
├── proposal.md           # Your structured proposal
├── requirements.md       # Formal requirements with IDs
├── clarifications.md     # Clarity Gate Q&A log
└── history/              # Archived completed changes
```

These files are designed to be:
- **Human-readable** — Clear markdown that anyone can understand
- **AI-consumable** — Structured enough that AI tools can parse and follow them
- **Version-controllable** — Plain text files that live in your repo

---

## The Research Behind SDD

SDD-Hoffy isn't built on opinions. It's built on research:

- **METR 2025**: In a randomized controlled trial, experienced developers were [19% slower with AI](https://metr.org/blog/2025-07-10-early-2025-ai-developer-study/) despite feeling 20% faster — because unstructured AI usage introduces debugging overhead and false confidence.

- **DORA 2025 State of DevOps**: Organizations see a [7.2% increase in delivery instability](https://dora.dev/research/) for every 25% increase in AI adoption — when adopted without foundational systems and practices.

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

- **Stage 4-6 implementation** — Design, Tasks, and Validate stages
- **More clarity dimensions** — Domain-specific dimensions (mobile, API, data pipeline, etc.)
- **Template improvements** — Better guided mode templates and examples
- **Host-specific guides** — Documentation for configuring in specific tools
- **i18n** — Support for non-English specs
- **Tests** — Unit and integration tests

---

## Roadmap

- [x] MVP: Init, Propose, Specify, Clarify pipeline
- [x] Dual mode (Guided/Expert)
- [x] MCP server with stdio transport
- [x] CI/CD pipeline (GitHub Actions + GoReleaser)
- [x] Multi-platform binary releases (Linux, macOS, Windows)
- [ ] Stage 4: Design (technical architecture generation)
- [ ] Stage 5: Tasks (atomic task breakdown)
- [ ] Stage 6: Validate (cross-artifact consistency)
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
