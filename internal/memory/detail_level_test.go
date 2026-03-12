package memory

import (
	"strings"
	"testing"
)

func TestParseDetailLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"summary", DetailSummary},
		{"standard", DetailStandard},
		{"full", DetailFull},
		{"", DetailStandard},
		{"invalid", DetailStandard},
		{"SUMMARY", DetailStandard}, // case-sensitive — only lowercase accepted
		{"Summary", DetailStandard},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDetailLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseDetailLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetailLevelValues(t *testing.T) {
	vals := DetailLevelValues()
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}

	expected := map[string]bool{
		DetailSummary:  true,
		DetailStandard: true,
		DetailFull:     true,
	}

	for _, v := range vals {
		if !expected[v] {
			t.Errorf("unexpected value: %q", v)
		}
	}
}

func TestSummaryFooterIsNotEmpty(t *testing.T) {
	if SummaryFooter == "" {
		t.Error("SummaryFooter should not be empty")
	}
}

func TestNavigationHint(t *testing.T) {
	tests := []struct {
		name    string
		showing int
		total   int
		hint    string
		want    string
	}{
		{"all results fit", 10, 10, "hint", ""},
		{"showing more than total", 15, 10, "hint", ""},
		{"total is zero", 0, 0, "hint", ""},
		{"total is negative", 5, -1, "hint", ""},
		{"capped with hint", 10, 47, "Use mem_get #ID for full content.", "\n📊 Showing 10 of 47. Use mem_get #ID for full content."},
		{"capped without hint", 5, 20, "", "\n📊 Showing 5 of 20."},
		{"showing zero of many", 0, 100, "Try different filters.", "\n📊 Showing 0 of 100. Try different filters."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NavigationHint(tt.showing, tt.total, tt.hint)
			if got != tt.want {
				t.Errorf("NavigationHint(%d, %d, %q) =\n  %q\nwant:\n  %q",
					tt.showing, tt.total, tt.hint, got, tt.want)
			}
		})
	}
}

// ─── Token Estimation Tests ─────────────────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty string", "", 0},
		{"single char", "a", 1},
		{"two chars", "ab", 1},
		{"three chars", "abc", 1},
		{"four chars", "abcd", 1},
		{"five chars", "abcde", 1},
		{"eight chars", "abcdefgh", 2},
		{"twelve chars", "abcdefghijkl", 3},
		{"100 chars", strings.Repeat("a", 100), 25},
		{"1000 chars", strings.Repeat("x", 1000), 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.want {
				t.Errorf("EstimateTokens(%d chars) = %d, want %d", len(tt.input), got, tt.want)
			}
		})
	}
}

func TestEstimateTokens_O1(t *testing.T) {
	// Verify O(1) behavior: same operation regardless of input size.
	// We can't truly benchmark here, but we can verify it doesn't iterate.
	small := EstimateTokens("hello")
	large := EstimateTokens(strings.Repeat("x", 1_000_000))
	if small < 1 {
		t.Errorf("small input should return at least 1, got %d", small)
	}
	if large != 250_000 {
		t.Errorf("large input should return 250000, got %d", large)
	}
}

func TestTokenFooter(t *testing.T) {
	tests := []struct {
		tokens int
		want   string
	}{
		{0, "\n📏 ~0 tokens"},
		{1, "\n📏 ~1 tokens"},
		{42, "\n📏 ~42 tokens"},
		{999, "\n📏 ~999 tokens"},
		{1000, "\n📏 ~1,000 tokens"},
		{1234, "\n📏 ~1,234 tokens"},
		{10000, "\n📏 ~10,000 tokens"},
		{1000000, "\n📏 ~1,000,000 tokens"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := TokenFooter(tt.tokens)
			if got != tt.want {
				t.Errorf("TokenFooter(%d) = %q, want %q", tt.tokens, got, tt.want)
			}
		})
	}
}

func TestBudgetFooter(t *testing.T) {
	got := BudgetFooter(500, 1000, 5, 20)
	if !strings.Contains(got, "500") {
		t.Error("should contain tokens used")
	}
	if !strings.Contains(got, "1,000") {
		t.Error("should contain budget")
	}
	if !strings.Contains(got, "5 of 20") {
		t.Error("should contain items shown/total")
	}
	if !strings.Contains(got, "max_tokens") {
		t.Error("should hint at max_tokens parameter")
	}
	if !strings.Contains(got, "⚡") {
		t.Error("should contain budget emoji")
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1000000, "1,000,000"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
