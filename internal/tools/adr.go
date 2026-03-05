package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/HendryAvila/Hoofy/internal/changes"
	"github.com/HendryAvila/Hoofy/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// ADRTool handles the sdd_adr MCP tool.
// It captures Architecture Decision Records in a unified central location.
type ADRTool struct {
	store  changes.Store
	bridge ChangeObserver
}

// NewADRTool creates an ADRTool with the given change store.
func NewADRTool(store changes.Store) *ADRTool {
	return &ADRTool{store: store}
}

// SetBridge injects an optional ChangeObserver for memory persistence.
func (t *ADRTool) SetBridge(obs ChangeObserver) { t.bridge = obs }

// validADRStatuses contains the allowed ADR status values.
var validADRStatuses = map[string]bool{
	"proposed":   true,
	"accepted":   true,
	"deprecated": true,
	"superseded": true,
}

// Definition returns the MCP tool definition for registration.
func (t *ADRTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_adr",
		mcp.WithDescription(
			"Capture an Architecture Decision Record (ADR). "+
				"Works with or without an active change. "+
				"ADRs are always stored in `docs/adrs/` with sequential numbering (001, 002, ...). "+
				"If captured during an active change, the ADR is linked to that change via its ChangeID. "+
				"ADRs document important architectural decisions with context, "+
				"rationale, and alternatives considered.",
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Short title for the decision. Example: 'Use PostgreSQL over MongoDB'"),
		),
		mcp.WithString("context",
			mcp.Required(),
			mcp.Description("Problem context — what situation requires a decision?"),
		),
		mcp.WithString("decision",
			mcp.Required(),
			mcp.Description("What was decided — the actual architectural decision made."),
		),
		mcp.WithString("rationale",
			mcp.Required(),
			mcp.Description("Why this decision was made — the reasoning behind it."),
		),
		mcp.WithString("alternatives_rejected",
			mcp.Description("What other options were considered and why they were rejected."),
		),
		mcp.WithString("status",
			mcp.Description("ADR status: proposed, accepted (default), deprecated, superseded."),
		),
	)
}

// Handle processes the sdd_adr tool call.
func (t *ADRTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title := req.GetString("title", "")
	adrContext := req.GetString("context", "")
	decision := req.GetString("decision", "")
	rationale := req.GetString("rationale", "")
	alternatives := req.GetString("alternatives_rejected", "")
	status := req.GetString("status", "accepted")

	// Validate required fields.
	if strings.TrimSpace(title) == "" {
		return mcp.NewToolResultError("'title' is required — provide a short title for the decision"), nil
	}
	if strings.TrimSpace(adrContext) == "" {
		return mcp.NewToolResultError("'context' is required — describe the problem context"), nil
	}
	if strings.TrimSpace(decision) == "" {
		return mcp.NewToolResultError("'decision' is required — state what was decided"), nil
	}
	if strings.TrimSpace(rationale) == "" {
		return mcp.NewToolResultError("'rationale' is required — explain why this decision was made"), nil
	}

	// Validate status.
	if !validADRStatuses[status] {
		return mcp.NewToolResultError(fmt.Sprintf(
			"invalid ADR status %q: must be one of: proposed, accepted, deprecated, superseded", status,
		)), nil
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}

	// Determine next ADR number by scanning docs/adrs/.
	adrsDir := config.ADRsPath(projectRoot)
	if err := os.MkdirAll(adrsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating adrs directory: %w", err)
	}

	adrNum := nextADRNumber(adrsDir)
	adrID := fmt.Sprintf("ADR-%03d", adrNum)
	slug := slugifyTitle(title)
	filename := fmt.Sprintf("%03d-%s.md", adrNum, slug)

	// Build ADR markdown content.
	var content strings.Builder
	fmt.Fprintf(&content, "# %s\n\n", title)
	fmt.Fprintf(&content, "**ID:** %s\n", adrID)
	fmt.Fprintf(&content, "**Status:** %s\n\n", status)

	// Check for active change and link it.
	active, err := t.store.LoadActive(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading active change: %w", err)
	}
	if active != nil {
		fmt.Fprintf(&content, "**Change:** `%s`\n\n", active.ID)
	}

	content.WriteString("## Context\n\n")
	content.WriteString(adrContext + "\n\n")
	content.WriteString("## Decision\n\n")
	content.WriteString(decision + "\n\n")
	content.WriteString("## Rationale\n\n")
	content.WriteString(rationale + "\n\n")
	if alternatives != "" {
		content.WriteString("## Alternatives Rejected\n\n")
		content.WriteString(alternatives + "\n")
	}

	adrContent := content.String()

	// Always write to docs/adrs/.
	adrPath := filepath.Join(adrsDir, filename)
	if err := writeStageFile(adrPath, adrContent); err != nil {
		return nil, fmt.Errorf("writing ADR: %w", err)
	}

	if active != nil {
		// Update change record to link this ADR.
		active.ADRs = append(active.ADRs, adrID)
		if err := t.store.Save(projectRoot, active); err != nil {
			return nil, fmt.Errorf("saving change: %w", err)
		}

		// Notify bridge with ADR content.
		notifyChangeObserver(t.bridge, active.ID, "adr", adrContent)

		response := fmt.Sprintf(
			"# ADR Captured\n\n"+
				"**ID:** %s\n"+
				"**Title:** %s\n"+
				"**Status:** %s\n"+
				"**Change:** `%s`\n\n"+
				"Saved to `docs/adrs/%s`\n\n"+
				"## Content\n\n%s",
			adrID, title, status, active.ID,
			filename,
			adrContent,
		)
		return mcp.NewToolResultText(response), nil
	}

	// No active change — still saved to file, notify bridge for memory.
	notifyChangeObserver(t.bridge, "", "adr", adrContent)

	response := fmt.Sprintf(
		"# ADR Captured\n\n"+
			"**ID:** %s\n"+
			"**Title:** %s\n"+
			"**Status:** %s\n\n"+
			"Saved to `docs/adrs/%s`\n\n"+
			"## Content\n\n%s",
		adrID, title, status,
		filename,
		adrContent,
	)
	return mcp.NewToolResultText(response), nil
}

// adrNumberPattern matches ADR filenames like "001-some-title.md".
var adrNumberPattern = regexp.MustCompile(`^(\d{3})-`)

// nextADRNumber scans the adrs directory and returns the next sequential number.
func nextADRNumber(adrsDir string) int {
	entries, err := os.ReadDir(adrsDir)
	if err != nil {
		return 1
	}

	maxNum := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := adrNumberPattern.FindStringSubmatch(e.Name())
		if len(matches) == 2 {
			if n, err := strconv.Atoi(matches[1]); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	return maxNum + 1
}

// slugifyTitle converts a title into a URL-friendly slug.
// "Use PostgreSQL over MongoDB" → "use-postgresql-over-mongodb"
func slugifyTitle(title string) string {
	// Lowercase.
	s := strings.ToLower(title)
	// Replace non-alphanumeric with hyphens.
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	// Trim leading/trailing hyphens.
	s = strings.Trim(s, "-")
	// Limit length.
	if len(s) > 60 {
		s = s[:60]
		// Don't end mid-word.
		if idx := strings.LastIndex(s, "-"); idx > 30 {
			s = s[:idx]
		}
	}
	return s
}

// listADRFiles returns sorted ADR filenames from the adrs directory.
// Exported for use by other tools (e.g., context_check, reverse_engineer).
func listADRFiles(adrsDir string) []string {
	entries, err := os.ReadDir(adrsDir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}
