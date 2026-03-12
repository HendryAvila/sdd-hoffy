package prompts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// MemoryGuidePrompt serves on-demand persistent memory documentation.
// This is the "cold" counterpart to the memory essentials in serverInstructions().
// Content moved here: progress tracking, compaction, topic keys, user prompts,
// progressive disclosure, detail_level, namespace scoping, context budget, knowledge graph.
type MemoryGuidePrompt struct{}

// NewMemoryGuidePrompt creates a MemoryGuidePrompt.
func NewMemoryGuidePrompt() *MemoryGuidePrompt {
	return &MemoryGuidePrompt{}
}

// Definition returns the MCP prompt definition for registration.
func (p *MemoryGuidePrompt) Definition() mcp.Prompt {
	return mcp.NewPrompt("sdd-memory-guide",
		mcp.WithPromptDescription(
			"Full persistent memory system documentation. "+
				"Covers progress tracking, compaction, topic keys, namespace scoping, "+
				"context budget, knowledge graph, and progressive disclosure patterns.",
		),
	)
}

// Handle returns the full memory system documentation.
func (p *MemoryGuidePrompt) Handle(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "SDD Memory System Guide",
		Messages: []mcp.PromptMessage{
			{
				Role:    mcp.RoleUser,
				Content: mcp.NewTextContent(memoryGuideContent),
			},
		},
	}, nil
}

const memoryGuideContent = `# SDD Memory System Guide

## Progress Tracking (mem_progress)

Use mem_progress to persist a structured work-in-progress document that survives context compaction.
Unlike session summaries (end-of-session), progress tracks WHERE YOU ARE mid-session.

**Dual behavior**:
- Read: mem_progress(project="X") — returns current progress (call at session start!)
- Write: mem_progress(project="X", content=JSON) — upserts the progress doc

**When to use**:
- At session start: read progress to check for prior WIP
- After completing significant work: update with current state
- Before context compaction: save progress so the next window can continue

**Content must be valid JSON.** Recommended structure:
{"goal": "...", "completed": ["..."], "next_steps": ["..."], "blockers": ["..."]}

One active progress per project — each write replaces the previous one.

## Memory Compaction (mem_compact)

Use mem_compact to identify and clean up stale observations that add noise to memory.
Over time, memory accumulates old session notes, outdated discoveries, and superseded
decisions. Compaction keeps memory lean and relevant.

**Dual behavior**:
- Identify: mem_compact(older_than_days=90) — lists stale candidates without deleting
- Execute: mem_compact(older_than_days=90, compact_ids="[1,2,3]") — batch soft-deletes

**Workflow** (two-step process):
1. Call mem_compact WITHOUT compact_ids to review candidates
2. Review the list — decide which observations are truly stale
3. Optionally write a summary to preserve key knowledge
4. Call mem_compact WITH compact_ids (and optional summary_title/summary_content)

**When to suggest compaction**:
- When mem_context returns many old, low-value observations
- When a user complains about memory noise or irrelevant results
- After a major milestone (v1 shipped, refactor complete) — clean up WIP notes
- When observation count exceeds 200+ for a project

**Summary observations**:
When compacting, create a summary to preserve the essence of what was deleted:
- summary_title: "Compacted 15 pre-v1 session notes"
- summary_content: Key decisions and patterns extracted from the deleted observations
- The summary is saved as type "compaction_summary" — searchable via mem_search

## Topic Keys for Evolving Observations

Use topic_key when an observation should UPDATE over time (not create duplicates):
- Architecture decisions: "architecture/auth-model", "architecture/data-layer"
- Project configuration: "config/deployment", "config/ci-cd"
Use mem_suggest_topic_key to generate a normalized key from a title.

## User Prompts

Call mem_save with save_type="prompt" to record what the user asked — their intent and goals.
This helps future sessions understand context without the user repeating themselves.

## Progressive Disclosure Pattern

1. Start with mem_context for recent observations
2. Use mem_search for specific topics
3. Use mem_timeline to see chronological context around a search result
4. Use mem_get to read the full, untruncated content

## Response Verbosity Control (detail_level parameter)

Several read-heavy tools support a detail_level parameter that controls response size.
Use this to manage context window budget — fetch the minimum detail needed first,
then drill deeper only when necessary (Anthropic: "context is a finite resource").

**Available levels**:
- summary: Minimal tokens — IDs, titles, metadata only. Use for orientation and triage.
- standard: Truncated content snippets. Good balance for most operations.
- full: Complete untruncated content. Use only when you need to analyze details.

**Default detail_level by tool**:
- sdd_get_context: defaults to summary (minimal pipeline overview)
- mem_context, mem_search, mem_timeline, sdd_context_check: default to standard

**Tools that support detail_level**:
- mem_context: Controls observation content in recent memory context
- mem_search: Controls search result content (summary = titles only, full = complete content)
- mem_timeline: Controls timeline entries (summary = titles only, full = all content untruncated)
- sdd_context_check: Controls artifact excerpts and memory results in change reports
- sdd_get_context: Controls pipeline artifact content (summary = stage status only, full = complete artifacts)

**Navigation hints**:
When results are capped by limit, tools append a "Showing X of Y" footer.
This tells you whether you're seeing everything or need to adjust limits.
Tools with navigation hints: mem_search, mem_context, mem_timeline.

**Progressive disclosure with detail_level**:
1. Start with summary to scan what exists (minimal tokens)
2. If something looks relevant, use standard for that specific tool call
3. Only use full when you need the complete content for analysis

Summary-mode responses include a footer hint reminding about the option to use
standard or full for more detail.

## Sub-Agent Memory Scoping (namespace parameter)

When multiple AI sub-agents work in parallel (e.g., orchestrator spawns researcher, coder, reviewer),
use the namespace parameter to isolate each sub-agent's memory observations.

**What namespace does**:
- Tags observations with a namespace string (e.g., "subagent/task-123", "agent/researcher")
- Read tools filter by namespace when provided — each sub-agent sees only its own notes
- Omitting namespace = no filter — the orchestrator sees EVERYTHING (by design)

**Namespace vs scope**: These are orthogonal concepts:
- scope = WHO sees it (project vs personal) — visibility level
- namespace = WHICH AGENT owns it — isolation boundary

**Tools that support namespace**:
- Write: mem_save, mem_session, mem_progress
- Read: mem_search, mem_context, mem_compact

**Convention for namespace values**:
- Sub-agents by task: "subagent/task-123", "subagent/research-auth"
- Sub-agents by role: "agent/researcher", "agent/coder", "agent/reviewer"
- Orchestrator: omit namespace entirely (sees all namespaces)

**Typical multi-agent workflow**:
1. Orchestrator spawns sub-agent with a task ID
2. Sub-agent uses namespace="subagent/<task-id>" on all mem_save/mem_search calls
3. Sub-agent's observations are isolated — no cross-contamination with other sub-agents
4. Orchestrator reads without namespace to see all observations, then synthesizes
5. Orchestrator saves final synthesis without namespace (shared knowledge)

**mem_progress with namespace**: When namespace is provided, the topic_key becomes
"progress/<namespace>/<project>" instead of "progress/<project>", giving each
sub-agent its own progress document.

**mem_timeline does NOT support namespace**: Timeline is inherently ID-scoped
(centered on a specific observation_id), so namespace filtering is unnecessary.

## Context Budget Awareness (max_tokens parameter)

Five read-heavy tools accept an optional max_tokens integer parameter that caps response
size by estimated token count. Use this when context window space is limited or when
you need to fit a response within a specific token budget.

**How it works**:
- Token estimation uses len(text)/4 heuristic (fast O(1), no tokenizer dependency)
- When max_tokens is set, the response is capped at approximately that many tokens
- Every response from these tools includes a "~N tokens" footer showing estimated size
- When a response is budget-capped, a "Budget-capped" notice is prepended to the footer

**Tools that support max_tokens**:
- mem_context: Incremental build — stops adding observations when budget would be exceeded
- mem_search: Incremental build — stops adding results when budget would be exceeded
- mem_timeline: Post-hoc truncation — builds full response, then truncates to budget
- sdd_get_context: Post-hoc truncation — builds full response, then truncates to budget
- sdd_context_check: Post-hoc truncation — builds full response, then truncates to budget

**When to use max_tokens**:
- When you know your remaining context window budget and want to stay within it
- When fetching context for a sub-agent with a smaller context window
- When you need a quick overview and want to cap verbosity beyond what detail_level provides

**max_tokens vs detail_level**: These are complementary:
- detail_level controls WHAT is included (summary vs standard vs full content)
- max_tokens controls HOW MUCH total output, regardless of detail level
- Use detail_level first to control content type, then max_tokens as a hard cap if needed

## Knowledge Graph (Relations)

Observations can be connected with typed, directional relations to form a knowledge graph.
This transforms flat memories into a navigable web of connected decisions, patterns, and discoveries.

**Creating relations** — use mem_relate after saving related observations:
- mem_relate(from_id, to_id, relation_type) — creates a directional edge
- Common types: relates_to, implements, depends_on, caused_by, supersedes, part_of
- Use bidirectional=true when the relationship goes both ways
- Add a note to explain WHY the observations are related

**Traversing the graph** — use mem_get(depth=...) to explore connections:
- mem_get(id=...) — full observation with direct relations
- mem_get(id=..., depth=3) — includes deeper connected graph context
- Use this when exploring a topic to understand its full web of related decisions

**Removing relations** — use mem_relate(action="remove", id=...) with the relation ID

**When to create relations**:
- After a bug fix, relate it to the decision that caused it (caused_by)
- After implementing a feature, relate tasks to their requirements (implements)
- When a new decision supersedes an old one (supersedes)
- When observations are about the same topic (relates_to)
- When one pattern depends on another (depends_on)
`
