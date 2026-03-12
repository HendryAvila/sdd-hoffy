# API Contracts: Facade Tools (Task 001)

This document defines the canonical request/response contracts for the new facade tools introduced in the consolidation strategy.

## 1) `mem_save` (Unified Save Interface)

### Intent

Single memory write entrypoint for:
- structured observations
- prompt capture
- passive capture

### Request

Required base fields:
- `content: string`

Optional common fields:
- `save_type: "observation" | "prompt" | "passive"` (default: `"observation"`)
- `session_id: string` (default: `"manual-save"`)
- `project: string`
- `namespace: string`

Observation-mode fields (`save_type="observation"`):
- `title: string` (required in observation mode)
- `type: string` (default: `"manual"`)
- `scope: "project" | "personal"` (default: `"project"`)
- `topic_key: string`
- `upsert: boolean` (default: `false`)
- `relate_to: number[]` (observation IDs to relate with `relates_to`)

Prompt-mode fields (`save_type="prompt"`):
- ignores observation-only fields (`title`, `topic_key`, `scope`, etc.)

Passive-mode fields (`save_type="passive"`):
- `source: string`

### Behavior

- `observation`: writes observation via `AddObservation`; if `upsert=true` and `topic_key` matches, update existing topic observation instead of creating duplicate.
- `prompt`: writes via prompt path (`AddPrompt` equivalent behavior).
- `passive`: runs passive extraction pipeline (`PassiveCapture` equivalent behavior).
- `relate_to`: only valid for `observation`; server creates `relates_to` edges from newly saved observation to each target ID (best effort per edge, non-fatal aggregation).

### Response

Canonical text response MUST include:
- operation summary
- primary ID(s)
- any warnings (ignored params, partial relation failures, upsert target used)

Examples:
- `Memory saved: "Auth decision" (decision)\nID: 42`
- `Prompt saved (ID: 77)`
- `Passive capture complete: 3 extracted, 2 saved, 1 duplicates`

### Validation Errors

- invalid `save_type`
- missing `title` when `save_type="observation"`
- missing `content`
- invalid `relate_to` element type/id

## 2) `mem_session` (Unified Session Lifecycle)

### Intent

Single lifecycle entrypoint replacing previous split session start/end APIs.

### Request

Required:
- `action: "start" | "end"`

For `action="start"`:
- `id: string` (required)
- `project: string` (required)
- `directory: string` (optional)

For `action="end"`:
- `id: string` (required)
- `summary: string` (optional)
- `auto_summary: boolean` (optional, default `false`)

### Behavior

- `start`: delegates to session create path.
- `end`: closes session; if `summary` provided, persists it as session summary text on close.
- `auto_summary=true`: server may synthesize short summary from recent session observations if explicit summary absent.

### Response

- `Session "<id>" started for project "<project>"`
- `Session "<id>" completed`
- if auto-summary used: append `Summary auto-generated` note.

### Validation Errors

- missing/invalid `action`
- missing required fields for selected action

## 3) SDD Context APIs (Dedicated Endpoints)

Current canonical endpoints remain dedicated by use case:
- `sdd_get_context`
- `sdd_context_check`
- `sdd_suggest_context`

Each retains its existing request/response contracts and validation behavior.

## 4) Breaking-change Mapping (applied)

Consolidated canonical APIs:

- Prompt persistence and passive capture are routed through `mem_save(save_type=...)`
- Session lifecycle and summary are routed through `mem_session(action=..., summary=...)`
- Observation retrieval and graph traversal are routed through `mem_get(id, depth)`
- Relation creation/removal are routed through `mem_relate(action=...)`

## 5) Compatibility policy

This migration intentionally applies as a breaking change with no legacy aliases.
