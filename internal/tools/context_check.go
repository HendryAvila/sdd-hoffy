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

// ContextCheckTool handles the sdd_context_check MCP tool.
// It scans existing project artifacts, completed changes, and memory
// to build a context report. The tool is a SCANNER — it returns raw
// findings for the AI to analyze using research-backed heuristics
// from server instructions (ADR-001).
type ContextCheckTool struct {
	changeStore changes.Store
	memStore    *memory.Store // nullable — works without memory
}

// NewContextCheckTool creates a ContextCheckTool with its dependencies.
// memStore may be nil — the tool degrades gracefully by skipping memory search.
func NewContextCheckTool(cs changes.Store, ms *memory.Store) *ContextCheckTool {
	return &ContextCheckTool{changeStore: cs, memStore: ms}
}

// Definition returns the MCP tool definition for registration.
func (t *ContextCheckTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_context_check",
		mcp.WithDescription(
			"Scan existing project specs, completed changes, and memory for context "+
				"relevant to the current change. This is a SCANNER tool — it returns a "+
				"structured report of findings. The AI then analyzes the report using "+
				"IEEE 29148 Requirements Smells heuristics and impact classification "+
				"from server instructions to generate the context-check.md artifact. "+
				"Call this when context-check is the current stage in the change pipeline.",
		),
		mcp.WithString("change_description",
			mcp.Required(),
			mcp.Description("The description of the change being made. Used for keyword "+
				"matching against completed changes and memory search."),
		),
		mcp.WithString("project_name",
			mcp.Description("Project name for filtering memory search results. "+
				"Optional — if omitted, memory search is unfiltered."),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail: 'summary' (filenames, sizes, and slugs only — minimal tokens), "+
					"'standard' (default — truncated excerpts of artifacts and memory), "+
					"'full' (complete untruncated artifact content and memory results).",
			),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates the response to stay within budget. 0 or omit for no cap."),
		),
	)
}

// conventionFiles are project files to scan when no SDD artifacts exist.
// These provide context about project conventions and patterns.
var conventionFiles = []string{
	"CLAUDE.md",
	"AGENTS.md",
	"README.md",
	"CONTRIBUTING.md",
}

// conventionDirs are directories to scan for convention files.
var conventionDirs = []string{
	".cursor/rules",
}

// maxConventionLines is the maximum number of lines to read from convention files.
const maxConventionLines = 200

// maxKeywordMatches is the maximum number of completed changes to return.
const maxKeywordMatches = 10

// maxMemoryResults is the maximum number of explore observations to return.
const maxMemoryResults = 5

// Handle processes the sdd_context_check tool call.
func (t *ContextCheckTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	changeDesc := strings.TrimSpace(req.GetString("change_description", ""))
	projectName := req.GetString("project_name", "")
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	maxTokens := intArgContextCheck(req, "max_tokens", 0)

	if changeDesc == "" {
		return mcp.NewToolResultError("'change_description' is required — describe the change to check context for"), nil
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Context Check Report\n\n")

	// --- Section 1: Existing SDD artifacts ---
	artifacts := t.scanArtifacts(projectRoot)
	hasArtifacts := len(artifacts) > 0

	sb.WriteString("## Existing Artifacts Found\n\n")
	if hasArtifacts {
		for _, a := range artifacts {
			fmt.Fprintf(&sb, "- **%s** (%d bytes)\n", a.name, a.size)
		}
	} else {
		sb.WriteString("_No SDD artifacts found in this project._\n")
	}
	sb.WriteString("\n")

	// --- Section 2: Relevant prior changes ---
	sb.WriteString("## Relevant Prior Changes\n\n")
	keywords := extractKeywords(changeDesc)
	matches := t.findRelevantChanges(projectRoot, keywords)
	if len(matches) > 0 {
		for _, m := range matches {
			fmt.Fprintf(&sb, "- **%s** (%s/%s): %s\n", m.ID, m.Type, m.Size, m.Description)
		}
	} else {
		sb.WriteString("_No completed changes found matching this description._\n")
	}
	sb.WriteString("\n")

	// --- Section 3: Explore context from memory ---
	sb.WriteString("## Explore Context (Memory)\n\n")
	if t.memStore != nil {
		results, searchErr := t.memStore.Search(changeDesc, memory.SearchOptions{
			Type:    "explore",
			Project: projectName,
			Limit:   maxMemoryResults,
		})
		if searchErr != nil {
			fmt.Fprintf(&sb, "_Memory search failed: %v_\n", searchErr)
		} else if len(results) > 0 {
			for _, r := range results {
				switch detailLevel {
				case memory.DetailSummary:
					fmt.Fprintf(&sb, "- **%s** (ID: %d)\n", r.Title, r.ID)
				case memory.DetailFull:
					fmt.Fprintf(&sb, "- **%s** (ID: %d): %s\n", r.Title, r.ID, r.Content)
				default:
					fmt.Fprintf(&sb, "- **%s** (ID: %d): %s\n", r.Title, r.ID, truncateContent(r.Content, 200))
				}
			}
		} else {
			sb.WriteString("_No explore observations found._\n")
		}
	} else {
		sb.WriteString("_Memory subsystem not available._\n")
	}
	sb.WriteString("\n")

	// --- Section 4: Convention files (fallback when no SDD artifacts) ---
	sb.WriteString("## Convention Files\n\n")
	if !hasArtifacts {
		conventions := t.scanConventionFiles(projectRoot)
		if len(conventions) > 0 {
			sb.WriteString("_No formal SDD specs found. Scanning project conventions:_\n\n")
			for _, c := range conventions {
				switch detailLevel {
				case memory.DetailSummary:
					fmt.Fprintf(&sb, "- %s\n", c.name)
				case memory.DetailFull:
					fmt.Fprintf(&sb, "### %s\n\n```\n%s\n```\n\n", c.name, c.content)
				default:
					fmt.Fprintf(&sb, "### %s\n\n```\n%s\n```\n\n", c.name, c.content)
				}
			}
		} else {
			sb.WriteString("_No convention files found._\n")
		}
	} else {
		sb.WriteString("_SDD artifacts present — convention file scan skipped._\n")
	}
	sb.WriteString("\n")

	// --- Section 5: Artifact content excerpts ---
	if hasArtifacts && detailLevel != memory.DetailSummary {
		sb.WriteString("## Artifact Excerpts\n\n")
		for _, a := range artifacts {
			switch detailLevel {
			case memory.DetailFull:
				fmt.Fprintf(&sb, "### %s\n\n%s\n\n", a.name, a.content)
			default:
				fmt.Fprintf(&sb, "### %s\n\n%s\n\n", a.name, truncateContent(a.content, 500))
			}
		}
	}

	// Append footer hint for summary mode.
	if detailLevel == memory.DetailSummary {
		sb.WriteString(memory.SummaryFooter)
	}

	// Apply post-hoc budget truncation and token footer.
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

// intArgContextCheck extracts an integer argument from a tool request (JSON numbers are float64).
func intArgContextCheck(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

// --- Private types and helpers ---

// artifactInfo holds metadata about a scanned SDD artifact.
type artifactInfo struct {
	name    string
	size    int
	content string
}

// conventionInfo holds a scanned convention file.
type conventionInfo struct {
	name    string
	content string
}

// sddArtifactFiles are the SDD files to scan for existing project context.
var sddArtifactFiles = []string{
	"business-rules.md",
	"requirements.md",
	"charter.md",
	"principles.md",
	"design.md",
}

// scanArtifacts reads SDD artifact files from the project's docs/ directory.
func (t *ContextCheckTool) scanArtifacts(projectRoot string) []artifactInfo {
	sddDir := config.DocsPath(projectRoot)
	var found []artifactInfo

	for _, filename := range sddArtifactFiles {
		path := filepath.Join(sddDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // file doesn't exist or can't be read
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

// extractKeywords splits a description into keywords, filtering stop words.
func extractKeywords(description string) []string {
	words := strings.Fields(strings.ToLower(description))
	var keywords []string
	for _, w := range words {
		// Strip basic punctuation.
		w = strings.Trim(w, ".,;:!?\"'()-")
		if w == "" || len(w) < 3 || stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

// stopWords is a set of common words to filter from keyword matching.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true,
	"not": true, "you": true, "all": true, "can": true, "had": true,
	"her": true, "was": true, "one": true, "our": true, "out": true,
	"has": true, "its": true, "let": true, "may": true, "who": true,
	"did": true, "get": true, "him": true, "his": true, "how": true,
	"man": true, "new": true, "now": true, "old": true, "see": true,
	"way": true, "day": true, "too": true, "use": true, "she": true,
	"that": true, "with": true, "have": true, "this": true, "will": true,
	"your": true, "from": true, "they": true, "been": true, "said": true,
	"each": true, "which": true, "their": true, "there": true, "about": true,
	"would": true, "make": true, "like": true, "just": true, "over": true,
	"such": true, "take": true, "also": true, "into": true, "than": true,
	"them": true, "then": true, "some": true, "what": true, "when": true,
	"were": true, "other": true, "could": true, "after": true, "should": true,
}

// findRelevantChanges lists completed changes and keyword-matches against the description.
func (t *ContextCheckTool) findRelevantChanges(projectRoot string, keywords []string) []changes.ChangeRecord {
	allChanges, err := t.changeStore.List(projectRoot)
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

	// Sort by score descending (simple selection — max 10 results).
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Cap at maxKeywordMatches.
	if len(matches) > maxKeywordMatches {
		matches = matches[:maxKeywordMatches]
	}

	result := make([]changes.ChangeRecord, len(matches))
	for i, m := range matches {
		result[i] = m.change
	}
	return result
}

// scanConventionFiles reads convention files from the project root.
// Returns only files that exist, with content truncated to maxConventionLines.
func (t *ContextCheckTool) scanConventionFiles(projectRoot string) []conventionInfo {
	var found []conventionInfo

	// Scan individual files.
	for _, filename := range conventionFiles {
		path := filepath.Join(projectRoot, filename)
		content := readFirstLines(path, maxConventionLines)
		if content != "" {
			found = append(found, conventionInfo{name: filename, content: content})
		}
	}

	// Scan convention directories.
	for _, dir := range conventionDirs {
		dirPath := filepath.Join(projectRoot, dir)
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

// readFirstLines reads up to maxLines from a file. Returns empty string if unreadable.
func readFirstLines(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.SplitN(string(data), "\n", maxLines+1)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

// truncateContent truncates content to maxLen characters at a line boundary.
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	truncated := content[:maxLen]
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxLen/2 {
		truncated = truncated[:lastNewline]
	}
	return truncated + "\n[...truncated]"
}
