package memtools

import (
	"context"
	"fmt"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// SearchTool handles the mem_search MCP tool.
type SearchTool struct {
	store *memory.Store
}

// NewSearchTool creates a SearchTool.
func NewSearchTool(store *memory.Store) *SearchTool {
	return &SearchTool{store: store}
}

// Definition returns the MCP tool definition for mem_search.
func (t *SearchTool) Definition() mcp.Tool {
	return mcp.NewTool("mem_search",
		mcp.WithDescription(
			"Search your persistent memory across all sessions. Use this to find past decisions, "+
				"bugs fixed, patterns used, files changed, or any context from previous coding sessions.",
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query — natural language or keywords"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by type: tool_use, file_change, command, file_read, search, manual, decision, architecture, bugfix, pattern"),
		),
		mcp.WithString("project",
			mcp.Description("Filter by project name"),
		),
		mcp.WithString("scope",
			mcp.Description("Filter by scope: project (default) or personal"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 10, max: 20)"),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail: 'summary' (IDs, types, and titles only — minimal tokens), "+
					"'standard' (default — 300-char content snippets), "+
					"'full' (complete untruncated content per result).",
			),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithString("namespace",
			mcp.Description("Optional sub-agent namespace filter (e.g. 'subagent/task-123'). When set, only returns memories from this namespace."),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, stops adding results once the budget would be exceeded. 0 or omit for no cap."),
		),
	)
}

// Handle processes the mem_search tool call.
func (t *SearchTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("'query' is required"), nil
	}

	typ := req.GetString("type", "")
	project := req.GetString("project", "")
	scope := req.GetString("scope", "")
	limit := intArg(req, "limit", 10)
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	namespace := req.GetString("namespace", "")
	maxTokens := intArg(req, "max_tokens", 0)

	results, err := t.store.Search(query, memory.SearchOptions{
		Type:      typ,
		Project:   project,
		Scope:     scope,
		Limit:     limit,
		Namespace: namespace,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No memories found matching your query."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d memories:\n\n", len(results))

	shown := 0
	for i, r := range results {
		projectStr := ""
		if r.Project != nil {
			projectStr = *r.Project
		}
		topicInfo := ""
		if r.TopicKey != nil && *r.TopicKey != "" {
			topicInfo = fmt.Sprintf(" | topic: %s", *r.TopicKey)
		}

		var entry string
		switch detailLevel {
		case memory.DetailSummary:
			entry = fmt.Sprintf("[%d] #%d (%s) - %s\n",
				i+1, r.ID, r.Type, r.Title)

		case memory.DetailFull:
			entry = fmt.Sprintf("[%d] #%d (%s) - %s\n    %s\n    %s%s | scope: %s\n\n",
				i+1, r.ID, r.Type, r.Title,
				r.Content,
				projectStr, topicInfo, r.Scope,
			)

		default: // standard
			snippet := memory.Truncate(r.Content, 300)
			entry = fmt.Sprintf("[%d] #%d (%s) - %s\n    %s\n    %s%s | scope: %s\n\n",
				i+1, r.ID, r.Type, r.Title,
				snippet,
				projectStr, topicInfo, r.Scope,
			)
		}

		if maxTokens > 0 && memory.EstimateTokens(b.String()+entry) > maxTokens {
			b.WriteString(memory.BudgetFooter(memory.EstimateTokens(b.String()), maxTokens, shown, len(results)))
			b.WriteString(memory.TokenFooter(memory.EstimateTokens(b.String())))
			return mcp.NewToolResultText(b.String()), nil
		}

		b.WriteString(entry)
		shown++
	}

	// Append footer hint for summary mode.
	if detailLevel == memory.DetailSummary {
		b.WriteString(memory.SummaryFooter)
	}

	// Navigation hint when results are capped by limit.
	total, err := t.store.CountSearchResults(query, memory.SearchOptions{
		Type:      typ,
		Project:   project,
		Scope:     scope,
		Namespace: namespace,
	})
	if err == nil {
		b.WriteString(memory.NavigationHint(len(results), total,
			"Use mem_get #ID for full content."))
	}

	// Always append token footer for context budget visibility.
	b.WriteString(memory.TokenFooter(memory.EstimateTokens(b.String())))

	return mcp.NewToolResultText(b.String()), nil
}
