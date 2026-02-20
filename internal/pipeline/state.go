// Package pipeline handles SDD pipeline state transitions and validation.
//
// Separated from config to honor SRP: config deals with persistence,
// pipeline deals with business rules (what transitions are valid,
// what conditions must be met to advance).
package pipeline

import (
	"fmt"

	"github.com/HendryAvila/sdd-hoffy/internal/config"
)

// --- Clarity Gate thresholds (Liskov-safe: both modes use the same interface) ---

const (
	// ClarityThresholdGuided is the minimum clarity score for guided mode.
	ClarityThresholdGuided = 70
	// ClarityThresholdExpert is the minimum clarity score for expert mode.
	ClarityThresholdExpert = 50
)

// ClarityThreshold returns the required clarity score for the given mode.
func ClarityThreshold(mode config.Mode) int {
	if mode == config.ModeExpert {
		return ClarityThresholdExpert
	}
	return ClarityThresholdGuided
}

// --- State machine ---

// StageIndex returns the ordinal position of a stage, or -1 if unknown.
func StageIndex(stage config.Stage) int {
	for i, s := range config.StageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// CanAdvance checks whether the pipeline can move past the current stage.
// It enforces the Clarity Gate: you cannot leave the "clarify" stage
// until the clarity score meets the threshold for the active mode.
func CanAdvance(cfg *config.ProjectConfig) error {
	if cfg.CurrentStage == config.StageClarify {
		threshold := ClarityThreshold(cfg.Mode)
		if cfg.ClarityScore < threshold {
			return fmt.Errorf(
				"clarity gate not passed: score %d/%d (need %d for %s mode) — "+
					"run sdd_clarify to resolve ambiguities",
				cfg.ClarityScore, 100, threshold, cfg.Mode,
			)
		}
	}

	idx := StageIndex(cfg.CurrentStage)
	if idx < 0 {
		return fmt.Errorf("unknown stage: %s", cfg.CurrentStage)
	}
	if idx >= len(config.StageOrder)-1 {
		return fmt.Errorf("already at the final stage: %s", cfg.CurrentStage)
	}

	return nil
}

// Advance moves the pipeline to the next stage. It validates the
// transition first and updates stage statuses atomically.
func Advance(cfg *config.ProjectConfig) error {
	if err := CanAdvance(cfg); err != nil {
		return err
	}

	idx := StageIndex(cfg.CurrentStage)
	nextStage := config.StageOrder[idx+1]

	// Mark current as completed.
	markCompleted(cfg, cfg.CurrentStage)

	// Move forward.
	cfg.CurrentStage = nextStage
	markInProgress(cfg, nextStage)

	return nil
}

// MarkInProgress marks the current stage as actively being worked on.
func MarkInProgress(cfg *config.ProjectConfig) {
	markInProgress(cfg, cfg.CurrentStage)
}

// IsCompleted checks whether a specific stage has been completed.
func IsCompleted(cfg *config.ProjectConfig, stage config.Stage) bool {
	st, ok := cfg.StageStatus[stage]
	return ok && st.Status == "completed"
}

// RequireStage returns an error if the current stage doesn't match expected.
// Tools use this to ensure they're called at the right pipeline moment.
func RequireStage(cfg *config.ProjectConfig, expected config.Stage) error {
	if cfg.CurrentStage != expected {
		current := config.Stages[cfg.CurrentStage]
		exp := config.Stages[expected]
		return fmt.Errorf(
			"wrong pipeline stage: currently at '%s' (%s), but this tool requires '%s' (%s)",
			cfg.CurrentStage, current.Name, expected, exp.Name,
		)
	}
	return nil
}

// --- internal helpers ---

func markCompleted(cfg *config.ProjectConfig, stage config.Stage) {
	st := cfg.StageStatus[stage]
	st.Status = "completed"
	st.CompletedAt = now()
	cfg.StageStatus[stage] = st
}

func markInProgress(cfg *config.ProjectConfig, stage config.Stage) {
	st := cfg.StageStatus[stage]
	st.Status = "in_progress"
	if st.StartedAt == "" {
		st.StartedAt = now()
	}
	st.Iterations++
	cfg.StageStatus[stage] = st
}

func now() string {
	return Now()
}

// Now returns the current UTC time formatted as RFC3339.
// Exported for use by tools that need to record timestamps
// consistently with the pipeline's time format.
func Now() string {
	return timeNow().UTC().Format("2006-01-02T15:04:05Z07:00")
}
