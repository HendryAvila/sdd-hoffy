package resources

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HendryAvila/Hoofy/internal/config"
)

// findRoot walks up from cwd looking for docs/hoofy.json (or docs/specs/hoofy.json).
// Shared utility for resource handlers.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	current := dir
	for {
		// Check primary: docs/hoofy.json
		primary := filepath.Join(current, config.DocsDir, config.ConfigFile)
		if _, err := os.Stat(primary); err == nil {
			return current, nil
		}

		// Check fallback: docs/specs/hoofy.json
		fallback := filepath.Join(current, config.DocsDir, config.DocsDirFallback, config.ConfigFile)
		if _, err := os.Stat(fallback); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return dir, nil
		}
		current = parent
	}
}
