package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// agentSectionMarker is used to detect if the Hoofy section already exists
// in CLAUDE.md or AGENTS.md (idempotent append).
const agentSectionMarker = "## Hoofy SDD Project"

// InitTool handles the sdd_init_project MCP tool.
// It creates the docs/ directory structure and initial configuration.
type InitTool struct {
	store    config.Store
	renderer templates.Renderer
}

// NewInitTool creates an InitTool with the given config store and template renderer.
func NewInitTool(store config.Store, renderer templates.Renderer) *InitTool {
	return &InitTool{store: store, renderer: renderer}
}

// Definition returns the MCP tool definition for registration.
func (t *InitTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_init_project",
		mcp.WithDescription(
			"Initialize a new SDD (Spec-Driven Development) project. "+
				"Creates the docs/ directory with configuration and empty templates. "+
				"This is always the first step in the SDD pipeline.",
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Project name"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Brief description of what the project does"),
		),
		mcp.WithString("mode",
			mcp.Description("Interaction mode: 'guided' (step-by-step for non-technical users) or 'expert' (streamlined for developers). Defaults to 'guided'."),
			mcp.DefaultString("guided"),
			mcp.Enum("guided", "expert"),
		),
	)
}

// Handle processes the sdd_init_project tool call.
func (t *InitTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	description := req.GetString("description", "")
	modeStr := req.GetString("mode", "guided")

	if name == "" {
		return mcp.NewToolResultError("'name' is required"), nil
	}
	if description == "" {
		return mcp.NewToolResultError("'description' is required"), nil
	}

	mode := config.Mode(modeStr)
	if mode != config.ModeGuided && mode != config.ModeExpert {
		return mcp.NewToolResultError("'mode' must be 'guided' or 'expert'"), nil
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	// Guard: don't overwrite an existing project.
	if config.Exists(projectRoot) {
		return mcp.NewToolResultError(
			"SDD project already exists in this directory. Use sdd_get_context to see current state.",
		), nil
	}

	// Create directory structure.
	docsDir := config.DocsPath(projectRoot)
	dirs := []string{
		docsDir,
		filepath.Join(docsDir, "history"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Write initial config.
	cfg := config.NewProjectConfig(name, description, mode)
	if err := t.store.Save(projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	// Generate and write/append agent instructions file.
	agentFile, agentAction, err := t.writeAgentInstructions(projectRoot, name, config.DocsDir)
	if err != nil {
		// Non-fatal: log but don't fail initialization.
		agentFile = ""
		agentAction = "skipped (error: " + err.Error() + ")"
	}

	// Build response based on mode.
	modeLabel := "Guided"
	modeHint := "I'll walk you through each step with examples and explanations."
	if mode == config.ModeExpert {
		modeLabel = "Expert"
		modeHint = "Streamlined flow — I'll ask fewer questions and accept technical input directly."
	}

	agentLine := ""
	if agentFile != "" {
		agentLine = fmt.Sprintf("├── %s   # Agent instructions (%s)\n", filepath.Base(agentFile), agentAction)
	}

	response := fmt.Sprintf(
		"# SDD Project Initialized\n\n"+
			"**Project:** %s\n"+
			"**Mode:** %s\n"+
			"**Location:** `%s/`\n\n"+
			"## What was created\n\n"+
			"```\n%s/\n├── hoofy.json        # Project configuration\n└── history/          # For completed changes\n```\n\n"+
			"%s"+
			"## Next Step\n\n"+
			"The pipeline is now at **Stage 1: Principles**.\n\n"+
			"%s\n\n"+
			"Use `sdd_create_principles` to define your project's golden invariants.\n\n"+
			"**Tell me about your project's core beliefs** — what rules should NEVER be broken?",
		name, modeLabel, config.DocsDir, config.DocsDir,
		agentLine, modeHint,
	)

	return mcp.NewToolResultText(response), nil
}

// writeAgentInstructions generates the Hoofy SDD section and writes or appends
// it to the appropriate agent instructions file.
//
// Detection order:
//  1. CLAUDE.md exists → append to it
//  2. AGENTS.md exists → append to it
//  3. Neither → create AGENTS.md
//
// Idempotent: if the marker "## Hoofy SDD Project" already exists, skip.
// Returns (filepath, action, error) where action is "created" or "appended".
func (t *InitTool) writeAgentInstructions(projectRoot, projectName, docsDir string) (string, string, error) {
	data := templates.AgentInstructionsData{
		Name:    projectName,
		DocsDir: docsDir,
	}

	content, err := t.renderer.Render(templates.AgentInstructions, data)
	if err != nil {
		return "", "", fmt.Errorf("rendering agent instructions: %w", err)
	}

	// Detect which file to use.
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")

	targetPath := agentsPath
	action := "created"

	if fileExists(claudePath) {
		targetPath = claudePath
		action = "appended"
	} else if fileExists(agentsPath) {
		targetPath = agentsPath
		action = "appended"
	}

	// If the file exists, check for idempotency.
	if action == "appended" {
		existing, err := os.ReadFile(targetPath)
		if err != nil {
			return "", "", fmt.Errorf("reading %s: %w", filepath.Base(targetPath), err)
		}
		if strings.Contains(string(existing), agentSectionMarker) {
			// Section already exists — skip.
			return targetPath, "already present", nil
		}
		// Append with a separator.
		f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return "", "", fmt.Errorf("opening %s for append: %w", filepath.Base(targetPath), err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + content); err != nil {
			return "", "", fmt.Errorf("appending to %s: %w", filepath.Base(targetPath), err)
		}
		return targetPath, action, nil
	}

	// Create new file.
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return "", "", fmt.Errorf("creating %s: %w", filepath.Base(targetPath), err)
	}
	return targetPath, action, nil
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
