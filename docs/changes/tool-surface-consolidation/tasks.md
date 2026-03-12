# Tasks: Tool Surface Consolidation

## TASK-001: Define consolidated API contracts

**Component:** `internal/memtools`, `internal/tools`
**Description:** Specify request/response schemas for new facade tools (`mem_save` extensions, `mem_session`, `sdd_context`) and alias behavior for legacy tools.
**Acceptance Criteria:**
- [x] New parameters and defaults are documented for each facade.
- [x] Legacy-to-facade mapping is explicit and unambiguous.

## TASK-002: Implement `mem_save` consolidation path

**Component:** `internal/memtools/save.go`, `internal/memory/store.go`
**Description:** Extend `mem_save` to cover observation/prompt/passive flows via `save_type`, with optional `relate_to` and `upsert` behavior.
**Acceptance Criteria:**
- [x] `save_type` supports `observation|prompt|passive`.
- [x] Existing `mem_save` behavior remains valid by default.
- [x] Topic-key duplicate handling can be automatic when enabled.

## TASK-003: Implement session facade

**Component:** `internal/memtools/session.go`
**Description:** Add `mem_session(action=start|end)` and route start/end operations through one entrypoint; support summary on end.
**Acceptance Criteria:**
- [x] Start and end actions validated with clear errors.
- [x] End supports explicit summary and optional auto-summary fallback.
- [x] Legacy session tools remain available as aliases during migration.

## TASK-004: Consolidate memory get/relate APIs

**Component:** `internal/memtools/timeline.go`, `internal/memtools/relate.go`
**Description:** Consolidate retrieval and graph operations into `mem_get(id, depth)` and `mem_relate(action=add|remove)`.
**Acceptance Criteria:**
- [x] `mem_get` supports full observation retrieval and optional depth traversal.
- [x] `mem_relate` supports both add and remove actions.
- [x] Removed tools are covered by replacement mappings in docs.

## TASK-005: Register facades and compatibility aliases

**Component:** `internal/server/server.go`
**Description:** Register new facades and keep legacy tools active with deprecation guidance in descriptions.
**Acceptance Criteria:**
- [x] New canonical tools are registered and callable.
- [x] Removed legacy tools are no longer registered.
- [x] Tool descriptions/docs reflect canonical API names.

## TASK-006: Update docs and migration policy

**Component:** `docs/tool-reference.md`, `docs/workflow-guide.md`, `README.md`
**Description:** Document preferred facades, compatibility window, and deprecation timeline.
**Acceptance Criteria:**
- [x] Docs clearly recommend facades as default path.
- [x] Legacy status is documented as deprecated/compatibility.
- [ ] No contradictory workflow guidance remains.

## TASK-007: Add tests for facade + legacy parity

**Component:** `internal/memtools/memtools_test.go`, `internal/tools/*_test.go`
**Description:** Ensure new facades produce equivalent outcomes to legacy tool paths.
**Acceptance Criteria:**
- [x] Unit tests cover success and validation failures for all new modes.
- [x] Legacy and facade paths are parity-tested for key scenarios.

## TASK-008: Rollout telemetry checkpoints

**Component:** operational/release process
**Description:** Define phased rollout checkpoints and go/no-go thresholds for deprecation progression.
**Acceptance Criteria:**
- [x] P0/P1/P2 checkpoints and metrics are documented.
- [x] Decision rules for removing aliases are explicit.

## Task completion notes (current)

- TASK-001 completed (contracts defined).
- TASK-002 completed (`mem_save` unified save_type path, upsert, relate_to).
- TASK-003 completed (`mem_session` facade + legacy alias guidance).
- TASK-004 completed (`mem_get` + `mem_relate(action)` consolidation).
- TASK-005 completed (canonical tools registered; removed legacy memory APIs).
- TASK-006 completed (docs updated for canonical breaking API surface).
- TASK-007 completed (facade and compatibility tests added; suite passing).
