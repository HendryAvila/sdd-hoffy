# Spec: Tool Surface Consolidation

## Overview

Hoofy will reduce model-facing tool complexity through consolidated facade tools and server-side defaults while keeping backward compatibility during migration.

## Functional Requirements

- **FR-TS-001**: The system MUST provide a unified save interface via `mem_save` supporting save modes for observation, prompt, and passive capture.
- **FR-TS-002**: The system MUST provide a unified session lifecycle interface via `mem_session(action=start|end)`.
- **FR-TS-003**: The system MUST provide consolidated memory retrieval and relation APIs: `mem_get(id, depth)` and `mem_relate(action=add|remove)`.
- **FR-TS-004**: Replaced legacy memory tools MUST be removed (breaking change accepted).
- **FR-TS-005**: Consolidated tools MUST preserve behavior parity for equivalent inputs.
- **FR-TS-006**: Tool definitions and docs MUST reflect the new canonical APIs.

## Non-Functional Requirements

- **NFR-TS-001**: Consolidation rollout MAY be breaking if explicitly approved.
- **NFR-TS-002**: New facade paths MUST be covered by automated tests before legacy retirement.
- **NFR-TS-003**: Migration MUST be observable via telemetry checkpoints (usage share and error rate).

## Breaking-change rules

1. Removed tools must have explicit replacements documented.
2. Tests and docs must be updated in the same change.
3. Release notes should call out migration mappings clearly.

## Validation Scenarios

### Scenario 1: Unified memory save
- Given a caller uses `mem_save(save_type="prompt")`
- When content and metadata are provided
- Then a prompt-equivalent record is stored via the canonical unified API

### Scenario 2: Unified session lifecycle
- Given a caller uses `mem_session(action="start")`
- When required fields are provided
- Then session start behavior is handled by the canonical lifecycle API

### Scenario 3: Unified memory retrieval
- Given a caller uses `mem_get(id=42, depth=2)`
- When related observations exist
- Then full observation content and graph context are returned in one call

### Scenario 4: Breaking migration mapping
- Given an existing integration uses a removed tool
- When migrated to its replacement API
- Then equivalent behavior is preserved
