// Package tools implements MCP tool handlers for the SDD pipeline.
//
// Each tool is a function that receives dependencies via its struct (DIP)
// and returns a handler compatible with mcp-go's CallToolRequest signature.
//
// Design principles:
// - SRP: each file = one tool
// - DIP: tools depend on interfaces (config.Store, templates.Renderer), not concretions
// - OCP: new tools are added without modifying existing ones
package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HendryAvila/Hoofy/internal/config"
)

// findProjectRoot walks up from the current working directory looking
// for an existing docs/hoofy.json (or docs/specs/hoofy.json fallback).
// If none is found, returns cwd.
// This allows tools to work from any subdirectory of the project.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	// Walk up looking for docs/hoofy.json or docs/specs/hoofy.json
	current := dir
	for {
		primary := filepath.Join(current, config.DocsDir, config.ConfigFile)
		if _, err := os.Stat(primary); err == nil {
			return current, nil
		}

		fallback := filepath.Join(current, config.DocsDir, config.DocsDirFallback, config.ConfigFile)
		if _, err := os.Stat(fallback); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root, no Hoofy project found.
			// Return original cwd — the caller decides what to do.
			return dir, nil
		}
		current = parent
	}
}

// readStageFile reads the content of a stage's markdown artifact.
// Returns empty string if the file doesn't exist (not an error —
// the stage just hasn't been completed yet).
func readStageFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

// writeStageFile writes content to a stage's markdown artifact,
// creating parent directories as needed.
func writeStageFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
