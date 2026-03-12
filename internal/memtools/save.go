package memtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// SaveTool handles the mem_save MCP tool.
type SaveTool struct {
	store *memory.Store
}

// NewSaveTool creates a SaveTool with the given memory store.
func NewSaveTool(store *memory.Store) *SaveTool {
	return &SaveTool{store: store}
}

// Definition returns the MCP tool definition for mem_save.
func (t *SaveTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_save",
		mcp.WithDescription(
			"Save memory entries through a unified interface. Supports observations (default), prompts, and passive capture via save_type. "+
				"Call this PROACTIVELY after significant work so future sessions retain context.",
		),
		mcp.WithString("save_type",
			mcp.Description("Save mode: observation (default), prompt, passive"),
		),
		mcp.WithString("title",
			mcp.Description("Short, searchable title (required for save_type=observation)"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Structured content using **What**, **Why**, **Where**, **Learned** format"),
		),
		mcp.WithString("type",
			mcp.Description("Category: decision, architecture, bugfix, pattern, config, discovery, learning (default: manual)"),
		),
		mcp.WithString("session_id",
			mcp.Description("Session ID to associate with (default: manual-save)"),
		),
		mcp.WithString("project",
			mcp.Description("Project name"),
		),
		mcp.WithString("scope",
			mcp.Description("Scope for this observation: project (default) or personal"),
		),
		mcp.WithString("topic_key",
			mcp.Description("Optional topic identifier for upserts (e.g. architecture/auth-model). Reuses and updates the latest observation in same project+scope."),
		),
		mcp.WithString("namespace",
			mcp.Description("Optional sub-agent namespace for memory isolation (e.g. 'subagent/task-123', 'agent/researcher'). When set, observations are scoped to this namespace."),
		),
		mcp.WithBoolean("upsert",
			mcp.Description("If true and save_type=observation, auto-generate topic_key when missing to enable evolving-topic updates"),
		),
		mcp.WithString("relate_to",
			mcp.Description("Optional related observation IDs. Accepts JSON array string like '[1,2]' or an array in tool args. Observation mode only."),
		),
		mcp.WithString("source",
			mcp.Description("Source identifier for save_type=passive (e.g. tool name)"),
		),
	)
}

// Handle processes the mem_save tool call.
func (t *SaveTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	saveType := strings.ToLower(strings.TrimSpace(req.GetString("save_type", "observation")))
	if saveType == "" {
		saveType = "observation"
	}

	content := req.GetString("content", "")
	if content == "" {
		return mcp.NewToolResultError("'content' is required"), nil
	}

	sessionID := req.GetString("session_id", "manual-save")
	project := req.GetString("project", "")
	namespace := req.GetString("namespace", "")

	switch saveType {
	case "observation":
		return t.handleSaveObservation(req, sessionID, project, namespace)
	case "prompt":
		id, err := t.store.AddPrompt(memory.AddPromptParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
			Namespace: namespace,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save prompt: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Prompt saved (ID: %d)", id)), nil
	case "passive":
		source := req.GetString("source", "")
		result, err := t.store.PassiveCapture(memory.PassiveCaptureParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
			Source:    source,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("passive capture failed: %v", err)), nil
		}
		return mcp.NewToolResultText(
			fmt.Sprintf("Passive capture complete: %d extracted, %d saved, %d duplicates", result.Extracted, result.Saved, result.Duplicates),
		), nil
	default:
		return mcp.NewToolResultError("'save_type' must be one of: observation, prompt, passive"), nil
	}
}

func (t *SaveTool) handleSaveObservation(req mcp.CallToolRequest, sessionID, project, namespace string) (*mcp.CallToolResult, error) {
	title := req.GetString("title", "")
	if title == "" {
		return mcp.NewToolResultError("'title' is required for save_type=observation"), nil
	}

	content := req.GetString("content", "")
	typ := req.GetString("type", "manual")
	scope := req.GetString("scope", "project")
	topicKey := req.GetString("topic_key", "")
	upsert := boolArg(req, "upsert", false)

	if upsert && topicKey == "" {
		topicKey = memory.SuggestTopicKey(typ, title, content)
	}

	id, err := t.store.AddObservation(memory.AddObservationParams{
		SessionID: sessionID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Project:   project,
		Scope:     scope,
		TopicKey:  topicKey,
		Namespace: namespace,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save observation: %v", err)), nil
	}

	response := fmt.Sprintf("Memory saved: %q (%s)", title, typ)
	if topicKey == "" && !upsert {
		suggested := memory.SuggestTopicKey(typ, title, content)
		response += fmt.Sprintf("\nSuggested topic_key: %s", suggested)
	} else if topicKey != "" {
		response += fmt.Sprintf("\nTopic key: %s", topicKey)
	}
	response += fmt.Sprintf("\nID: %d", id)

	relatedIDs, parseErr := parseRelatedIDs(req.GetArguments()["relate_to"])
	if parseErr != nil {
		response += fmt.Sprintf("\nWarning: relate_to ignored (%v)", parseErr)
		return mcp.NewToolResultText(response), nil
	}

	if len(relatedIDs) > 0 {
		linked := 0
		var warnings []string
		for _, targetID := range relatedIDs {
			_, relErr := t.store.AddRelation(memory.AddRelationParams{
				FromID: id,
				ToID:   targetID,
				Type:   "relates_to",
			})
			if relErr != nil {
				warnings = append(warnings, fmt.Sprintf("%d (%v)", targetID, relErr))
				continue
			}
			linked++
		}
		response += fmt.Sprintf("\nRelations created: %d/%d", linked, len(relatedIDs))
		if len(warnings) > 0 {
			response += "\nRelation warnings: " + strings.Join(warnings, "; ")
		}
	}

	return mcp.NewToolResultText(response), nil
}

func parseRelatedIDs(raw any) ([]int64, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, nil
		}
		var vals []float64
		if err := json.Unmarshal([]byte(trimmed), &vals); err != nil {
			return nil, fmt.Errorf("expected JSON array of numbers")
		}
		out := make([]int64, 0, len(vals))
		for _, n := range vals {
			id := int64(n)
			if id <= 0 {
				return nil, fmt.Errorf("ids must be > 0")
			}
			out = append(out, id)
		}
		return out, nil
	case []any:
		out := make([]int64, 0, len(v))
		for _, item := range v {
			n, ok := item.(float64)
			if !ok {
				return nil, fmt.Errorf("array must contain numbers")
			}
			id := int64(n)
			if id <= 0 {
				return nil, fmt.Errorf("ids must be > 0")
			}
			out = append(out, id)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported type")
	}
}
