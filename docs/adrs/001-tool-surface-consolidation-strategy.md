# Tool Surface Consolidation Strategy

**ID:** ADR-001
**Status:** accepted
**Date:** 2026-03-12

## Context

Telemetry over 8 sessions and 94 calls showed a large tool surface where only a subset was used consistently. The top 5 tools represented most traffic, while many tools were either overlapping, stage-gated, or relied on the model to remember hygiene behavior.

This increases model cognitive load and token cost in server instructions, and creates selection ambiguity (multiple tools for similar intent).

We need a strategy that reduces external tool complexity without reducing core capability.

## Decision

Adopt a **facade-first consolidation strategy**:

1. Introduce consolidated facade tools in front of existing behavior:
   - `mem_save` becomes the unified entrypoint for observation/prompt/passive save modes.
   - `mem_session` becomes the lifecycle entrypoint (`start`/`end`) with summary at end.
   - `sdd_context` becomes the context entrypoint (`get`/`check`/`suggest`).
2. Apply migration in phases; legacy compatibility may be skipped when a breaking-change rollout is explicitly approved.
3. Move repetitive model-memory responsibilities into server-side defaults/hooks where safe.
4. Delay hard removals until telemetry confirms successful adoption.

## Rationale

- Preserves user and client stability (no immediate breaking change).
- Reduces choice ambiguity by guiding traffic through fewer “front doors”.
- Keeps internal stage-specific validators and business rules intact.
- Enables measurable rollout with before/after telemetry per phase.

## Alternatives Rejected

1. **Immediate hard removal of low-use tools**
   - Rejected because it creates abrupt API breakage for clients/prompts/scripts.

2. **Do nothing (only documentation tweaks)**
   - Rejected because telemetry indicates structural overload, not docs-only confusion.

3. **Single mega-tool for all SDD + memory operations**
   - Rejected because it collapses domain boundaries and weakens explicit contracts.

## Consequences

### Positive

- Smaller practical decision surface for the model.
- Better discoverability via intent-oriented facades.
- Incremental rollout with low operational risk.

### Negative

- Temporary duplication while aliases and facades coexist.
- Extra test matrix (legacy path + new facade path).

### Neutral/Operational

- Requires deprecation policy, release notes, and telemetry checkpoints per phase.
