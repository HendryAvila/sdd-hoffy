package memtools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// SessionTool handles the unified mem_session MCP tool.
type SessionTool struct {
	store *memory.Store
}

// NewSessionTool creates a SessionTool.
func NewSessionTool(store *memory.Store) *SessionTool {
	return &SessionTool{store: store}
}

// Definition returns the MCP tool definition for mem_session.
func (t *SessionTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_session",
		mcp.WithDescription(
			"Manage coding session lifecycle with a single entrypoint. Use action=start to begin a session and action=end to close it.",
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Session action: start or end"),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Session identifier"),
		),
		mcp.WithString("project",
			mcp.Description("Project name (required for action=start)"),
		),
		mcp.WithString("directory",
			mcp.Description("Working directory (optional for action=start)"),
		),
		mcp.WithString("summary",
			mcp.Description("Summary of what was accomplished (optional for action=end)"),
		),
	)
}

// Handle processes the mem_session tool call.
func (t *SessionTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.GetString("action", "")))
	id := req.GetString("id", "")
	if id == "" {
		return mcp.NewToolResultError("'id' is required"), nil
	}

	switch action {
	case "start":
		project := req.GetString("project", "")
		if project == "" {
			return mcp.NewToolResultError("'project' is required for action=start"), nil
		}
		directory := req.GetString("directory", "")
		if err := t.store.CreateSession(id, project, directory); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to start session: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Session %q started for project %q", id, project)), nil
	case "end":
		summary := req.GetString("summary", "")
		if err := t.store.EndSession(id, summary); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to end session: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Session %q completed", id)), nil
	default:
		return mcp.NewToolResultError("'action' must be one of: start, end"), nil
	}
}
