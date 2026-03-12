# Rollout Telemetry Checkpoints (TASK-008)

## Objective

Track facade adoption and safety before retiring compatibility aliases.

## Phases

### P0 (Facade introduction)

- Scope: `mem_save(save_type)`, `mem_session(action)`, `sdd_context(mode)` available; legacy tools still active.
- Monitor:
  - facade call share by flow
  - facade error rate vs legacy error rate
  - migration-hint exposure counts

### P1 (Facade preference enforcement)

- Scope: prompts/instructions/docs prefer facades by default.
- Monitor:
  - facade share >= 60%
  - no significant regression in tool-call success rate
  - no increase in repeated retries for replaced flows

### P2 (Alias retirement readiness)

- Scope: evaluate deprecation/removal candidates.
- Gate conditions (all required):
  - facade share >= 80% for replaced flows over rolling 14 days
  - facade error rate <= legacy error rate + 0.5pp
  - no high-severity compatibility incidents from existing clients

## Decision rules

- If all P2 gates pass: mark aliases as removable in next minor release planning.
- If any P2 gate fails: keep aliases, tune docs/instructions, and re-evaluate next checkpoint window.

## Recommended metrics table

| Metric | Definition | Threshold |
|---|---|---|
| Facade adoption | % of calls using facade endpoint for migrated flow | >= 80% (P2) |
| Error parity | facade error rate - legacy error rate | <= +0.5pp |
| Retry churn | repeated same-flow retries per session | non-increasing |
| Incident count | compatibility incidents attributed to consolidation | 0 high severity |
