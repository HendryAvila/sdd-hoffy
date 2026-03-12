package memtools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// ─── RelateTool ─────────────────────────────────────────────────────────────

// RelateTool handles the mem_relate MCP tool.
type RelateTool struct {
	store *memory.Store
}

// NewRelateTool creates a RelateTool with the given memory store.
func NewRelateTool(store *memory.Store) *RelateTool {
	return &RelateTool{store: store}
}

// Definition returns the MCP tool definition for mem_relate.
func (t *RelateTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_relate",
		mcp.WithDescription(
			"Create or remove typed relations between memory observations. "+
				"Use action=add (default) to create links and action=remove to delete by relation ID.",
		),
		mcp.WithString("action",
			mcp.Description("Relation action: add (default) or remove"),
		),
		mcp.WithNumber("from_id",
			mcp.Description("Source observation ID (required for action=add)"),
		),
		mcp.WithNumber("to_id",
			mcp.Description("Target observation ID (required for action=add)"),
		),
		mcp.WithString("relation_type",
			mcp.Description("Type of relation: relates_to, implements, depends_on, caused_by, supersedes, part_of (required for action=add)"),
		),
		mcp.WithString("note",
			mcp.Description("Optional context about why these observations are related"),
		),
		mcp.WithBoolean("bidirectional",
			mcp.Description("If true, creates both A→B and B→A relations atomically (default: false)"),
		),
		mcp.WithNumber("id",
			mcp.Description("Relation ID to remove (required for action=remove)"),
		),
	)
}

// Handle processes the mem_relate tool call.
func (t *RelateTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.GetString("action", "add")))
	if action == "" {
		action = "add"
	}

	if action == "remove" {
		id := intArg(req, "id", 0)
		if id == 0 {
			return mcp.NewToolResultError("'id' is required for action=remove"), nil
		}
		err := t.store.RemoveRelation(int64(id))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to remove relation: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Relation %d removed", id)), nil
	}

	if action != "add" {
		return mcp.NewToolResultError("'action' must be one of: add, remove"), nil
	}

	fromID := intArg(req, "from_id", 0)
	toID := intArg(req, "to_id", 0)

	if fromID == 0 {
		return mcp.NewToolResultError("'from_id' is required"), nil
	}
	if toID == 0 {
		return mcp.NewToolResultError("'to_id' is required"), nil
	}

	relType := req.GetString("relation_type", "")
	if relType == "" {
		return mcp.NewToolResultError("'relation_type' is required"), nil
	}

	note := req.GetString("note", "")
	bidir := boolArg(req, "bidirectional", false)

	ids, err := t.store.AddRelation(memory.AddRelationParams{
		FromID:        int64(fromID),
		ToID:          int64(toID),
		Type:          relType,
		Note:          note,
		Bidirectional: bidir,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create relation: %v", err)), nil
	}

	if bidir {
		return mcp.NewToolResultText(
			fmt.Sprintf("Bidirectional relation created: #%d ↔ #%d (%s)\nRelation IDs: %d, %d",
				fromID, toID, relType, ids[0], ids[1]),
		), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Relation created: #%d → #%d (%s)\nRelation ID: %d",
			fromID, toID, relType, ids[0]),
	), nil
}
