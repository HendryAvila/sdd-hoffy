# Change Description: Tool Surface Consolidation

## Problem

Hoofy currently exposes a broad MCP tool surface with significant overlap and low utilization in multiple tool groups. Telemetry indicates concentrated usage around a small subset, while many tools are rarely or never selected.

This creates avoidable model-side cognitive load, larger instruction payloads, and ambiguous tool routing.

## Goal

Define and execute a migration plan that reduces practical tool-selection complexity while preserving capability and compatibility.

## Scope

In scope:
- Consolidation plan for memory save/session/context tool families.
- Consolidation plan for SDD context tool family.
- Backward-compatible migration and aliasing strategy.
- Hook/server-automation opportunities and guardrails.

Out of scope:
- Immediate hard deletion of legacy tools in this first phase.
- Rewriting core business logic of pipeline stages.

## Constraints

- Do not break existing clients abruptly.
- Keep current stage/business-rule validations intact.
- Roll out in phases with telemetry verification between phases.

## Success Criteria

- A written migration strategy exists with phased rollout.
- Naming, compatibility policy, and deprecation timeline are explicit.
- Implementation backlog is prioritized (P0/P1/P2) and testable.
