package memtools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// TimelineTool handles the mem_timeline MCP tool.
type TimelineTool struct {
	store *memory.Store
}

// NewTimelineTool creates a TimelineTool.
func NewTimelineTool(store *memory.Store) *TimelineTool {
	return &TimelineTool{store: store}
}

// Definition returns the MCP tool definition for mem_timeline.
func (t *TimelineTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_timeline",
		mcp.WithDescription(
			"Show chronological context around a specific observation. Use after mem_search "+
				"to drill into the timeline of events surrounding a search result. This is the "+
				"progressive disclosure pattern: search first, then timeline to understand context.",
		),
		mcp.WithNumber("observation_id",
			mcp.Required(),
			mcp.Description("The observation ID to center the timeline on (from mem_search results)"),
		),
		mcp.WithNumber("before",
			mcp.Description("Number of observations to show before the focus (default: 5)"),
		),
		mcp.WithNumber("after",
			mcp.Description("Number of observations to show after the focus (default: 5)"),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail: 'summary' (titles and timestamps only — minimal tokens), "+
					"'standard' (default — 200-char snippets for before/after, full content for focus), "+
					"'full' (complete untruncated content for ALL entries).",
			),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates the response to stay within budget. 0 or omit for no cap."),
		),
	)
}

// Handle processes the mem_timeline tool call.
func (t *TimelineTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	obsID := intArg(req, "observation_id", 0)
	if obsID == 0 {
		return mcp.NewToolResultError("'observation_id' is required"), nil
	}

	before := intArg(req, "before", 5)
	after := intArg(req, "after", 5)
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	maxTokens := intArg(req, "max_tokens", 0)

	result, err := t.store.Timeline(int64(obsID), before, after)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("timeline failed: %v", err)), nil
	}

	var b strings.Builder

	// Session context
	if result.SessionInfo != nil {
		fmt.Fprintf(&b, "Session: %s (%s)\n", result.SessionInfo.Project, result.SessionInfo.StartedAt)
		fmt.Fprintf(&b, "Total observations in session: %d\n\n", result.TotalInRange)
	}

	switch detailLevel {
	case memory.DetailSummary:
		t.formatTimelineSummary(&b, result)
	case memory.DetailFull:
		t.formatTimelineFull(&b, result)
	default:
		t.formatTimelineStandard(&b, result)
	}

	// Append footer hint for summary mode.
	if detailLevel == memory.DetailSummary {
		b.WriteString(memory.SummaryFooter)
	}

	// Navigation hint when timeline window is smaller than total observations.
	showing := len(result.Before) + 1 + len(result.After)
	b.WriteString(memory.NavigationHint(showing, result.TotalInRange,
		"Increase before/after to see more context."))

	// Apply post-hoc budget truncation if max_tokens is set.
	response := b.String()
	if maxTokens > 0 && memory.EstimateTokens(response) > maxTokens {
		// Truncate at approximate character position (maxTokens * 4 chars/token).
		charBudget := maxTokens * 4
		if charBudget < len(response) {
			response = response[:charBudget]
			// Find last newline to avoid cutting mid-line.
			if lastNL := strings.LastIndex(response, "\n"); lastNL > charBudget/2 {
				response = response[:lastNL]
			}
			response += "\n" + memory.BudgetFooter(memory.EstimateTokens(response), maxTokens, showing, result.TotalInRange)
		}
	}

	// Always append token footer for context budget visibility.
	response += memory.TokenFooter(memory.EstimateTokens(response))

	return mcp.NewToolResultText(response), nil
}

// formatTimelineStandard is the original behavior: 200-char snippets for
// before/after entries, full content for the focus observation.
func (t *TimelineTool) formatTimelineStandard(b *strings.Builder, result *memory.TimelineResult) {
	if len(result.Before) > 0 {
		b.WriteString("--- Before ---\n")
		for _, e := range result.Before {
			snippet := memory.Truncate(e.Content, 200)
			fmt.Fprintf(b, "#%d [%s] %s: %s\n", e.ID, e.Type, e.Title, snippet)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b, ">>> #%d [%s] %s <<<\n", result.Focus.ID, result.Focus.Type, result.Focus.Title)
	fmt.Fprintf(b, "%s\n\n", result.Focus.Content)

	if len(result.After) > 0 {
		b.WriteString("--- After ---\n")
		for _, e := range result.After {
			snippet := memory.Truncate(e.Content, 200)
			fmt.Fprintf(b, "#%d [%s] %s: %s\n", e.ID, e.Type, e.Title, snippet)
		}
	}
}

// formatTimelineSummary shows only titles and types — no content at all.
func (t *TimelineTool) formatTimelineSummary(b *strings.Builder, result *memory.TimelineResult) {
	if len(result.Before) > 0 {
		b.WriteString("--- Before ---\n")
		for _, e := range result.Before {
			fmt.Fprintf(b, "#%d [%s] %s\n", e.ID, e.Type, e.Title)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b, ">>> #%d [%s] %s <<<\n\n", result.Focus.ID, result.Focus.Type, result.Focus.Title)

	if len(result.After) > 0 {
		b.WriteString("--- After ---\n")
		for _, e := range result.After {
			fmt.Fprintf(b, "#%d [%s] %s\n", e.ID, e.Type, e.Title)
		}
	}
}

// formatTimelineFull shows complete untruncated content for ALL entries.
func (t *TimelineTool) formatTimelineFull(b *strings.Builder, result *memory.TimelineResult) {
	if len(result.Before) > 0 {
		b.WriteString("--- Before ---\n")
		for _, e := range result.Before {
			fmt.Fprintf(b, "#%d [%s] %s: %s\n", e.ID, e.Type, e.Title, e.Content)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b, ">>> #%d [%s] %s <<<\n", result.Focus.ID, result.Focus.Type, result.Focus.Title)
	fmt.Fprintf(b, "%s\n\n", result.Focus.Content)

	if len(result.After) > 0 {
		b.WriteString("--- After ---\n")
		for _, e := range result.After {
			fmt.Fprintf(b, "#%d [%s] %s: %s\n", e.ID, e.Type, e.Title, e.Content)
		}
	}
}

// ─── GetObservationTool ─────────────────────────────────────────────────────

// GetObservationTool handles the mem_get MCP tool.
type GetObservationTool struct {
	store *memory.Store
}

// NewGetObservationTool creates a GetObservationTool.
func NewGetObservationTool(store *memory.Store) *GetObservationTool {
	return &GetObservationTool{store: store}
}

// Definition returns the MCP tool definition for mem_get.
func (t *GetObservationTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_get",
		mcp.WithDescription(
			"Get the full content of a specific observation by ID. Optionally include related graph context via depth.",
		),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("The observation ID to retrieve"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Optional graph traversal depth (0 default, max 5). When > 0, includes related context."),
		),
	)
}

// Handle processes the mem_get tool call.
func (t *GetObservationTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := intArg(req, "id", 0)
	if id == 0 {
		return mcp.NewToolResultError("'id' is required"), nil
	}
	depth := intArg(req, "depth", 0)
	if depth < 0 {
		return mcp.NewToolResultError("'depth' must be >= 0"), nil
	}
	if depth > 5 {
		depth = 5
	}

	obs, err := t.store.GetObservation(int64(id))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("observation #%d not found", id)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Observation #%d\n\n", obs.ID)
	fmt.Fprintf(&b, "**Title:** %s\n", obs.Title)
	fmt.Fprintf(&b, "**Type:** %s\n", obs.Type)
	fmt.Fprintf(&b, "**Scope:** %s\n", obs.Scope)

	if obs.Project != nil {
		fmt.Fprintf(&b, "**Project:** %s\n", *obs.Project)
	}
	if obs.TopicKey != nil && *obs.TopicKey != "" {
		fmt.Fprintf(&b, "**Topic Key:** %s\n", *obs.TopicKey)
	}
	if obs.ToolName != nil && *obs.ToolName != "" {
		fmt.Fprintf(&b, "**Tool:** %s\n", *obs.ToolName)
	}

	fmt.Fprintf(&b, "**Session:** %s\n", obs.SessionID)
	fmt.Fprintf(&b, "**Created:** %s\n", obs.CreatedAt)
	fmt.Fprintf(&b, "**Updated:** %s\n", obs.UpdatedAt)
	fmt.Fprintf(&b, "**Revisions:** %d\n", obs.RevisionCount)
	fmt.Fprintf(&b, "**Duplicates:** %d\n\n", obs.DuplicateCount)
	fmt.Fprintf(&b, "## Content\n\n%s\n", obs.Content)

	// Append direct relations if any exist
	rels, relErr := t.store.GetRelations(obs.ID)
	if relErr == nil && len(rels) > 0 {
		var outgoing, incoming []string
		for _, r := range rels {
			if r.FromID == obs.ID {
				label := fmt.Sprintf("- → #%d (%s)", r.ToID, r.Type)
				if r.Note != "" {
					label += fmt.Sprintf(" — %s", r.Note)
				}
				outgoing = append(outgoing, label)
			} else {
				label := fmt.Sprintf("- ← #%d (%s)", r.FromID, r.Type)
				if r.Note != "" {
					label += fmt.Sprintf(" — %s", r.Note)
				}
				incoming = append(incoming, label)
			}
		}

		b.WriteString("\n## Relations\n\n")
		if len(outgoing) > 0 {
			b.WriteString("**Outgoing:**\n")
			for _, o := range outgoing {
				b.WriteString(o + "\n")
			}
		}
		if len(incoming) > 0 {
			if len(outgoing) > 0 {
				b.WriteString("\n")
			}
			b.WriteString("**Incoming:**\n")
			for _, i := range incoming {
				b.WriteString(i + "\n")
			}
		}
	}

	if depth > 0 {
		ctxResult, ctxErr := t.store.BuildContext(obs.ID, depth)
		if ctxErr == nil && ctxResult != nil && len(ctxResult.Connected) > 0 {
			b.WriteString("\n## Graph Context\n\n")
			for _, n := range ctxResult.Connected {
				arrow := "→"
				if n.Direction == "incoming" {
					arrow = "←"
				}
				fmt.Fprintf(&b, "- depth %d %s #%d (%s) %q\n", n.Depth, arrow, n.ID, n.RelationType, n.Title)
			}
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}
