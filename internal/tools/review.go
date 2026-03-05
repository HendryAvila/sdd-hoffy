package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// ReviewTool handles the sdd_review MCP tool.
// It generates a spec-aware code review checklist by parsing project
// specs (requirements, business rules, design, ADRs) and matching them
// against a change description. Unlike generic code reviewers, every
// checklist item references a specific spec ID (FR-XXX, BRC-XXX, etc.).
//
// Standalone by design (ADR: three-feature design) — works without an
// active change pipeline or hoofy.json.
type ReviewTool struct {
	memStore *memory.Store // nullable — degrades gracefully
}

// NewReviewTool creates a ReviewTool with its dependencies.
// memStore may be nil — the tool skips ADR search when unavailable.
func NewReviewTool(ms *memory.Store) *ReviewTool {
	return &ReviewTool{memStore: ms}
}

// Definition returns the MCP tool definition for registration.
func (t *ReviewTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_review",
		mcp.WithDescription(
			"Generate a spec-aware code review checklist for a given change. "+
				"Parses project specs (requirements, business rules, design, ADRs) "+
				"and generates verification items that reference specific spec IDs. "+
				"Works WITHOUT an active change pipeline or hoofy.json — standalone tool. "+
				"The AI then reviews actual code against this checklist.",
		),
		mcp.WithString("change_description",
			mcp.Required(),
			mcp.Description("What was changed or what to review? Describe the "+
				"implementation, bug fix, or feature. Used for matching against specs."),
		),
		mcp.WithString("project_name",
			mcp.Description("Project name for filtering memory ADR search. "+
				"Optional — if omitted, memory search is unfiltered."),
		),
		mcp.WithString("detail_level",
			mcp.Description(
				"Level of detail: 'summary' (checklist items only — minimal tokens), "+
					"'standard' (default — items with brief context), "+
					"'full' (items with full spec text).",
			),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates the response "+
				"to stay within budget. 0 or omit for no cap."),
		),
	)
}

// reviewMaxADRs is the maximum number of ADR observations to include.
const reviewMaxADRs = 5

// Handle processes the sdd_review tool call.
func (t *ReviewTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	changeDesc := strings.TrimSpace(req.GetString("change_description", ""))
	projectName := req.GetString("project_name", "")
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	maxTokens := intArgReview(req, "max_tokens", 0)

	if changeDesc == "" {
		return mcp.NewToolResultError("'change_description' is required — describe the change to review"), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	keywords := extractKeywords(changeDesc)
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Spec-Aware Review Checklist\n\n")
	fmt.Fprintf(&sb, "**Change**: %q\n", changeDesc)

	// Track which specs were found for the header.
	var specsFound []string

	// --- Section 1: Requirements Verification ---
	reqItems := t.parseRequirements(cwd, keywords)
	if len(reqItems) > 0 {
		specsFound = append(specsFound, "requirements.md ✅")
	}

	// --- Section 2: Business Rule Compliance ---
	ruleItems := t.parseBusinessRules(cwd, keywords)
	if len(ruleItems) > 0 {
		specsFound = append(specsFound, "business-rules.md ✅")
	}

	// --- Section 3: Design Conformance ---
	designItems := t.parseDesign(cwd, keywords)
	if len(designItems) > 0 {
		specsFound = append(specsFound, "design.md ✅")
	}

	// --- Section 4: ADR Alignment ---
	adrItems := t.searchADRs(changeDesc, projectName)

	// Write header with specs analyzed.
	if len(specsFound) > 0 {
		fmt.Fprintf(&sb, "**Specs analyzed**: %s\n\n", strings.Join(specsFound, ", "))
	} else {
		sb.WriteString("**Specs analyzed**: _none found_\n\n")
	}

	// Write requirement items.
	sb.WriteString("## Requirements Verification\n\n")
	if len(reqItems) > 0 {
		for _, item := range reqItems {
			writeChecklistItem(&sb, item, detailLevel)
		}
	} else {
		sb.WriteString("_No matching requirements found._\n")
	}
	sb.WriteString("\n")

	// Write business rule items.
	sb.WriteString("## Business Rule Compliance\n\n")
	if len(ruleItems) > 0 {
		for _, item := range ruleItems {
			writeChecklistItem(&sb, item, detailLevel)
		}
	} else {
		sb.WriteString("_No matching business rules found._\n")
	}
	sb.WriteString("\n")

	// Write design items.
	sb.WriteString("## Design Conformance\n\n")
	if len(designItems) > 0 {
		for _, item := range designItems {
			writeChecklistItem(&sb, item, detailLevel)
		}
	} else {
		sb.WriteString("_No matching design elements found._\n")
	}
	sb.WriteString("\n")

	// Write ADR items.
	sb.WriteString("## ADR Alignment\n\n")
	if len(adrItems) > 0 {
		for _, item := range adrItems {
			writeChecklistItem(&sb, item, detailLevel)
		}
	} else {
		sb.WriteString("_No relevant ADRs found._\n")
	}
	sb.WriteString("\n")

	// General checks — always included.
	sb.WriteString("## General Checks\n\n")
	sb.WriteString("- [ ] No new business rules introduced without updating business-rules.md\n")
	sb.WriteString("- [ ] No new API endpoints without updating design.md\n")
	sb.WriteString("- [ ] Changes align with the documented architectural pattern\n")
	sb.WriteString("- [ ] New domain terms are added to the Ubiquitous Language glossary\n")
	sb.WriteString("\n")

	// Summary footer.
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

// --- Checklist item types ---

// checklistItem represents a single review checklist item.
type checklistItem struct {
	id       string // e.g., "FR-012", "BRC-003", "AuthModule"
	summary  string // brief description for standard mode
	fullText string // complete spec text for full mode
}

// writeChecklistItem writes a single checklist item with appropriate detail.
func writeChecklistItem(sb *strings.Builder, item checklistItem, detailLevel string) {
	switch detailLevel {
	case memory.DetailSummary:
		fmt.Fprintf(sb, "- [ ] **%s**\n", item.id)
	case memory.DetailFull:
		fmt.Fprintf(sb, "- [ ] **%s**: %s\n", item.id, item.fullText)
	default:
		fmt.Fprintf(sb, "- [ ] **%s**: %s\n", item.id, item.summary)
	}
}

// --- Spec parsers ---

// requirementPattern matches lines like "- **FR-001**: description" or "- **NFR-001**: description".
var requirementPattern = regexp.MustCompile(`^\s*-\s*\*\*([FN](?:FR|R)-\d+)\*\*:\s*(.+)$`)

// parseRequirements reads requirements.md and extracts FR/NFR lines that match keywords.
func (t *ReviewTool) parseRequirements(cwd string, keywords []string) []checklistItem {
	content := readFileContent(filepath.Join(config.DocsPath(cwd), "requirements.md"))
	if content == "" {
		return nil
	}

	var items []checklistItem
	for _, line := range strings.Split(content, "\n") {
		matches := requirementPattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		id := matches[1]
		desc := matches[2]

		if keywordMatch(desc, keywords) {
			items = append(items, checklistItem{
				id:       id,
				summary:  truncateReview(desc, 120),
				fullText: desc,
			})
		}
	}

	return items
}

// constraintPattern matches "When ... Then ..." business rule lines.
var constraintPattern = regexp.MustCompile(`(?i)^\s*-\s*(?:\*\*([^*]+)\*\*:?\s*)?[Ww]hen\s+`)

// parseBusinessRules reads business-rules.md and extracts constraints that match keywords.
func (t *ReviewTool) parseBusinessRules(cwd string, keywords []string) []checklistItem {
	content := readFileContent(filepath.Join(config.DocsPath(cwd), "business-rules.md"))
	if content == "" {
		return nil
	}

	// Find the Constraints section.
	lines := strings.Split(content, "\n")
	var items []checklistItem
	inConstraints := false
	ruleIdx := 0

	for _, line := range lines {
		// Detect constraint section headers.
		if strings.Contains(strings.ToLower(line), "constraint") && strings.HasPrefix(line, "##") {
			inConstraints = true
			continue
		}
		// Exit constraints section on next ## header.
		if inConstraints && strings.HasPrefix(line, "## ") && !strings.Contains(strings.ToLower(line), "constraint") {
			inConstraints = false
			continue
		}

		if !inConstraints {
			continue
		}

		// Match constraint lines.
		if constraintPattern.MatchString(line) {
			ruleIdx++
			ruleText := strings.TrimSpace(strings.TrimLeft(line, "- "))
			id := fmt.Sprintf("BRC-%03d", ruleIdx)

			// Check if the constraint line itself has an explicit ID.
			if matches := constraintPattern.FindStringSubmatch(line); len(matches) > 1 && matches[1] != "" {
				id = matches[1]
			}

			if keywordMatch(ruleText, keywords) {
				items = append(items, checklistItem{
					id:       id,
					summary:  truncateReview(ruleText, 120),
					fullText: ruleText,
				})
			}
		}
	}

	return items
}

// parseDesign reads design.md and extracts component sections that match keywords.
func (t *ReviewTool) parseDesign(cwd string, keywords []string) []checklistItem {
	content := readFileContent(filepath.Join(config.DocsPath(cwd), "design.md"))
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var items []checklistItem
	var currentHeading string
	var currentBody []string

	flush := func() {
		if currentHeading == "" {
			return
		}
		body := strings.Join(currentBody, "\n")
		combined := currentHeading + " " + body
		if keywordMatch(combined, keywords) {
			items = append(items, checklistItem{
				id:       currentHeading,
				summary:  truncateReview(strings.TrimSpace(body), 120),
				fullText: strings.TrimSpace(body),
			})
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			flush()
			currentHeading = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			currentBody = nil
		} else if strings.HasPrefix(line, "## ") {
			flush()
			currentHeading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			currentBody = nil
		} else if currentHeading != "" {
			currentBody = append(currentBody, line)
		}
	}
	flush()

	return items
}

// searchADRs queries memory for decision-type observations matching the change.
func (t *ReviewTool) searchADRs(changeDesc, projectName string) []checklistItem {
	if t.memStore == nil {
		return nil
	}

	results, err := t.memStore.Search(changeDesc, memory.SearchOptions{
		Type:    "decision",
		Project: projectName,
		Limit:   reviewMaxADRs,
	})
	if err != nil || len(results) == 0 {
		return nil
	}

	var items []checklistItem
	for _, r := range results {
		items = append(items, checklistItem{
			id:       fmt.Sprintf("ADR #%d", r.ID),
			summary:  truncateReview(r.Title, 120),
			fullText: r.Title + ": " + truncateReview(r.Content, 500),
		})
	}

	return items
}

// --- Private helpers ---

// intArgReview extracts an integer argument from a tool request.
func intArgReview(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

// readFileContent reads a file and returns its content as a string.
// Returns empty string if the file doesn't exist or can't be read.
func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// keywordMatch returns true if the text contains any of the keywords.
func keywordMatch(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// truncateReview truncates text to maxLen characters.
func truncateReview(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
