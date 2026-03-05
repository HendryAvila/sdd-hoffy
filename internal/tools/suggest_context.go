package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/changes"
	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// SuggestContextTool handles the sdd_suggest_context MCP tool.
// It recommends relevant specs, memory, and changes for a given task
// description WITHOUT requiring an active change pipeline or hoofy.json.
// This bridges the gap for ad-hoc sessions that don't go through the
// formal change pipeline (87% of sessions per the Codified Context paper).
type SuggestContextTool struct {
	changeStore changes.Store
	memStore    *memory.Store // nullable — degrades gracefully
}

// NewSuggestContextTool creates a SuggestContextTool with its dependencies.
// memStore may be nil — the tool skips memory search when unavailable.
func NewSuggestContextTool(cs changes.Store, ms *memory.Store) *SuggestContextTool {
	return &SuggestContextTool{changeStore: cs, memStore: ms}
}

// Definition returns the MCP tool definition for registration.
func (t *SuggestContextTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_suggest_context",
		mcp.WithDescription(
			"Recommend relevant specs, memory observations, and completed changes "+
				"for a given task description. Works WITHOUT an active change pipeline "+
				"or hoofy.json — ideal for ad-hoc sessions. Returns a prioritized, "+
				"actionable list of context to read before starting work.",
		),
		mcp.WithString("task_description",
			mcp.Required(),
			mcp.Description("What work are you about to do? Describe the task, "+
				"bug, feature, or question. Used for keyword matching and memory search."),
		),
		mcp.WithString("project_name",
			mcp.Description("Project name for filtering memory search results. "+
				"Optional — if omitted, memory search is unfiltered."),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail: 'summary' (names and IDs only — minimal tokens), "+
					"'standard' (default — truncated excerpts), "+
					"'full' (complete untruncated content).",
			),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates the response "+
				"to stay within budget. 0 or omit for no cap."),
		),
	)
}

// suggestMaxMemoryResults is the maximum number of memory observations to return.
const suggestMaxMemoryResults = 10

// suggestMaxChanges is the maximum number of completed changes to return.
const suggestMaxChanges = 5

// suggestMaxSectionLen is the max characters for an artifact section excerpt.
const suggestMaxSectionLen = 300

// Handle processes the sdd_suggest_context tool call.
func (t *SuggestContextTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskDesc := strings.TrimSpace(req.GetString("task_description", ""))
	projectName := req.GetString("project_name", "")
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	maxTokens := intArgSuggest(req, "max_tokens", 0)

	if taskDesc == "" {
		return mcp.NewToolResultError("'task_description' is required — describe the task you're about to work on"), nil
	}

	// Use working directory directly — no hoofy.json required (FR-030, FR-031).
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	keywords := extractKeywords(taskDesc)
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Suggested Context for: %q\n\n", taskDesc)

	// --- Section 1: Relevant SDD artifacts ---
	artifacts := t.suggestScanArtifacts(cwd)
	hasArtifacts := len(artifacts) > 0

	sb.WriteString("## Relevant Specs\n\n")
	if hasArtifacts {
		for _, a := range artifacts {
			sections := matchArtifactSections(a.content, keywords)
			if len(sections) == 0 {
				// No keyword matches — still mention the artifact exists.
				if detailLevel != memory.DetailSummary {
					fmt.Fprintf(&sb, "- **%s** (%d bytes) — _no keyword matches_\n", a.name, a.size)
				}
				continue
			}
			fmt.Fprintf(&sb, "- **%s** — %d relevant section(s):\n", a.name, len(sections))
			for _, sec := range sections {
				switch detailLevel {
				case memory.DetailSummary:
					fmt.Fprintf(&sb, "  - %s\n", sec.heading)
				case memory.DetailFull:
					fmt.Fprintf(&sb, "  - **%s**:\n    %s\n", sec.heading, sec.content)
				default:
					fmt.Fprintf(&sb, "  - **%s**: %s\n", sec.heading, truncateSuggest(sec.content, suggestMaxSectionLen))
				}
			}
		}
		if !hasMatchingArtifacts(artifacts, keywords) {
			sb.WriteString("_No keyword matches found in specs._\n")
		}
	} else {
		sb.WriteString("_No SDD artifacts found in this project._\n")
	}
	sb.WriteString("\n")

	// --- Section 2: Related completed changes ---
	sb.WriteString("## Related Changes\n\n")
	matches := t.suggestFindChanges(cwd, keywords)
	if len(matches) > 0 {
		for _, m := range matches {
			fmt.Fprintf(&sb, "- **%s** (%s/%s): %s\n", m.ID, m.Type, m.Size, m.Description)
		}
	} else {
		sb.WriteString("_No completed changes found matching this task._\n")
	}
	sb.WriteString("\n")

	// --- Section 3: Memory observations ---
	sb.WriteString("## Memory (Decisions & Patterns)\n\n")
	if t.memStore != nil {
		results, searchErr := t.memStore.Search(taskDesc, memory.SearchOptions{
			Project: projectName,
			Limit:   suggestMaxMemoryResults,
		})
		if searchErr != nil {
			fmt.Fprintf(&sb, "_Memory search failed: %v_\n", searchErr)
		} else if len(results) > 0 {
			for _, r := range results {
				switch detailLevel {
				case memory.DetailSummary:
					fmt.Fprintf(&sb, "- #%d (%s): %s\n", r.ID, r.Type, r.Title)
				case memory.DetailFull:
					fmt.Fprintf(&sb, "- #%d (%s) **%s**: %s\n", r.ID, r.Type, r.Title, r.Content)
				default:
					fmt.Fprintf(&sb, "- #%d (%s) **%s**: %s\n", r.ID, r.Type, r.Title, truncateSuggest(r.Content, 200))
				}
			}
		} else {
			sb.WriteString("_No relevant memory observations found._\n")
		}
	} else {
		sb.WriteString("_Memory subsystem not available._\n")
	}
	sb.WriteString("\n")

	// --- Section 4: Convention files (fallback) ---
	if !hasArtifacts {
		sb.WriteString("## Convention Files\n\n")
		conventions := t.suggestScanConventions(cwd)
		if len(conventions) > 0 {
			for _, c := range conventions {
				switch detailLevel {
				case memory.DetailSummary:
					fmt.Fprintf(&sb, "- %s\n", c.name)
				case memory.DetailFull:
					fmt.Fprintf(&sb, "### %s\n\n```\n%s\n```\n\n", c.name, c.content)
				default:
					fmt.Fprintf(&sb, "### %s\n\n```\n%s\n```\n\n", c.name, truncateSuggest(c.content, 500))
				}
			}
		} else {
			sb.WriteString("_No convention files found._\n")
		}
		sb.WriteString("\n")
	}

	// --- Summary footer ---
	if detailLevel == memory.DetailSummary {
		sb.WriteString(memory.SummaryFooter)
	}

	// Apply post-hoc budget truncation.
	response := sb.String()
	if maxTokens > 0 && memory.EstimateTokens(response) > maxTokens {
		charBudget := maxTokens * 4
		if charBudget < len(response) {
			response = response[:charBudget]
			if lastNL := strings.LastIndex(response, "\n"); lastNL > charBudget/2 {
				response = response[:lastNL]
			}
			response += "\n[...truncated by token budget]"
		}
	}

	response += memory.TokenFooter(memory.EstimateTokens(response))

	return mcp.NewToolResultText(response), nil
}

// --- Private helpers ---

// intArgSuggest extracts an integer argument from a tool request.
func intArgSuggest(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

// suggestScanArtifacts reads SDD artifact files from the cwd's docs/ directory.
// Returns empty slice if docs/ doesn't exist (FR-031).
func (t *SuggestContextTool) suggestScanArtifacts(cwd string) []artifactInfo {
	docsDir := config.DocsPath(cwd)
	var found []artifactInfo

	for _, filename := range sddArtifactFiles {
		path := filepath.Join(docsDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}
		found = append(found, artifactInfo{
			name:    filename,
			size:    len(data),
			content: string(data),
		})
	}

	return found
}

// suggestFindChanges lists completed changes keyword-matched against the task.
func (t *SuggestContextTool) suggestFindChanges(cwd string, keywords []string) []changes.ChangeRecord {
	allChanges, err := t.changeStore.List(cwd)
	if err != nil || len(allChanges) == 0 {
		return nil
	}

	type scored struct {
		change changes.ChangeRecord
		score  int
	}

	var matches []scored
	for _, c := range allChanges {
		lower := strings.ToLower(c.Description + " " + c.ID)
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{change: c, score: score})
		}
	}

	// Sort by score descending.
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) > suggestMaxChanges {
		matches = matches[:suggestMaxChanges]
	}

	result := make([]changes.ChangeRecord, len(matches))
	for i, m := range matches {
		result[i] = m.change
	}
	return result
}

// suggestScanConventions reads convention files from the project root.
func (t *SuggestContextTool) suggestScanConventions(cwd string) []conventionInfo {
	var found []conventionInfo

	for _, filename := range conventionFiles {
		path := filepath.Join(cwd, filename)
		content := readFirstLines(path, maxConventionLines)
		if content != "" {
			found = append(found, conventionInfo{name: filename, content: content})
		}
	}

	for _, dir := range conventionDirs {
		dirPath := filepath.Join(cwd, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dirPath, entry.Name())
			content := readFirstLines(path, maxConventionLines)
			if content != "" {
				found = append(found, conventionInfo{
					name:    filepath.Join(dir, entry.Name()),
					content: content,
				})
			}
		}
	}

	return found
}

// artifactSection represents a matched section within an SDD artifact.
type artifactSection struct {
	heading string
	content string
}

// matchArtifactSections splits artifact content by markdown headers (## )
// and returns sections that contain any of the keywords.
func matchArtifactSections(content string, keywords []string) []artifactSection {
	if len(keywords) == 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	var sections []artifactSection
	var currentHeading string
	var currentLines []string

	flush := func() {
		if currentHeading == "" {
			return
		}
		body := strings.Join(currentLines, "\n")
		lower := strings.ToLower(body + " " + currentHeading)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				sections = append(sections, artifactSection{
					heading: currentHeading,
					content: strings.TrimSpace(body),
				})
				return
			}
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			flush()
			currentHeading = strings.TrimSpace(strings.TrimLeft(line, "#"))
			currentLines = nil
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush() // last section

	return sections
}

// hasMatchingArtifacts returns true if any artifact has keyword-matching sections.
func hasMatchingArtifacts(artifacts []artifactInfo, keywords []string) bool {
	for _, a := range artifacts {
		if len(matchArtifactSections(a.content, keywords)) > 0 {
			return true
		}
	}
	return false
}

// truncateSuggest truncates content to maxLen characters at a line boundary.
func truncateSuggest(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	truncated := content[:maxLen]
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxLen/2 {
		truncated = truncated[:lastNewline]
	}
	return truncated + " [...]"
}
