package provider

import "testing"

func TestUsageCacheInfo(t *testing.T) {
	tests := []struct {
		name       string
		input      int
		cacheRead  int
		cacheWrite int
		total      int
		want       string
	}{
		// ── No data ──────────────────────────────────────────────────────────
		{
			name: "all_zeros_empty",
		},
		// ── Input with no cache activity ─────────────────────────────────────
		{
			name:  "input_only_shows_zero_pct",
			input: 1000,
			want:  "Cache: 0%",
		},
		{
			name:  "single_token_no_cache",
			input: 1,
			want:  "Cache: 0%",
		},
		// ── Cache hit percentage ──────────────────────────────────────────────
		{
			name:      "cache_25pct",
			input:     1000,
			cacheRead: 250,
			want:      "Cache: 25%",
		},
		{
			name:      "cache_50pct",
			input:     1000,
			cacheRead: 500,
			want:      "Cache: 50%",
		},
		{
			name:      "cache_75pct",
			input:     1000,
			cacheRead: 750,
			want:      "Cache: 75%",
		},
		{
			name:      "cache_100pct_exact",
			input:     1000,
			cacheRead: 1000,
			want:      "Cache: 100%",
		},
		{
			name:      "prompt_tokens_use_total_tokens_when_present",
			input:     400,
			cacheRead: 200,
			cacheWrite: 100,
			total:     700,
			want:      "Cache: 29%",
		},
		// ── Rounding ─────────────────────────────────────────────────────────
		// 333/1000 = 33.3… → rounds to 33%
		{
			name:      "rounding_down_33pct",
			input:     1000,
			cacheRead: 333,
			want:      "Cache: 33%",
		},
		// 667/1000 = 66.7… → rounds to 67%
		{
			name:      "rounding_up_67pct",
			input:     1000,
			cacheRead: 667,
			want:      "Cache: 67%",
		},
		// Small counts: 3/4 = 75%
		{
			name:      "small_counts_75pct",
			input:     4,
			cacheRead: 3,
			want:      "Cache: 75%",
		},
		// ── Defensive cap: cache read > input ────────────────────────────────
		{
			name:      "cache_read_exceeds_input_capped_at_100pct",
			input:     100,
			cacheRead: 200,
			want:      "Cache: 100%",
		},
		// ── Cache write only (Anthropic first-turn: no reads yet) ─────────────
		{
			name:       "cache_write_only_no_input",
			cacheWrite: 5000,
			want:       "CacheWrite: 5000",
		},
		// First turn: cache written, input sent, but no reads yet
		{
			name:       "cache_write_with_input_no_reads",
			input:      1000,
			cacheWrite: 5000,
			want:       "CacheWrite: 5000",
		},
		// ── Edge: cache read present but input is zero ────────────────────────
		// Can happen with malformed proxy responses; no meaningful percentage.
		{
			name:      "cache_read_without_input_empty",
			cacheRead: 500,
			want:      "",
		},
		// ── Both cache read and write, no input ──────────────────────────────
		// Read > 0 so case 1 (Input > 0 && CacheRead > 0) doesn't match;
		// case 2 (CacheWrite > 0 && CacheRead == 0) doesn't match either.
		// Falls through to default → "".
		{
			name:       "read_and_write_no_input_empty",
			cacheRead:  200,
			cacheWrite: 300,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Usage{
				Input:      tt.input,
				CacheRead:  tt.cacheRead,
				CacheWrite: tt.cacheWrite,
				TotalTokens: tt.total,
			}
			got := u.CacheInfo()
			if got != tt.want {
				t.Errorf("CacheInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUsagePromptTokens(t *testing.T) {
	tests := []struct {
		name       string
		usage      *Usage
		wantPrompt int
	}{
		{
			name:       "nil usage",
			usage:      nil,
			wantPrompt: 0,
		},
		{
			name: "uses total tokens when present",
			usage: &Usage{
				Input:       400,
				Output:      50,
				CacheRead:   200,
				CacheWrite:  100,
				TotalTokens: 750,
			},
			wantPrompt: 700,
		},
		{
			name: "falls back to input when total missing",
			usage: &Usage{
				Input:      400,
				Output:     50,
				CacheRead:  200,
				CacheWrite: 100,
			},
			wantPrompt: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.usage.PromptTokens(); got != tt.wantPrompt {
				t.Errorf("PromptTokens() = %d, want %d", got, tt.wantPrompt)
			}
		})
	}
}

func TestUsageTotalInputTokens(t *testing.T) {
	tests := []struct {
		name       string
		usage      *Usage
		wantInput  int
	}{
		{
			name:      "nil usage",
			usage:     nil,
			wantInput: 0,
		},
		{
			name: "uses total tokens when present",
			usage: &Usage{
				Input:       400,
				Output:      50,
				CacheRead:   200,
				CacheWrite:  100,
				TotalTokens: 750,
			},
			wantInput: 700,
		},
		{
			name: "falls back to components when total missing",
			usage: &Usage{
				Input:      400,
				Output:     50,
				CacheRead:  200,
				CacheWrite: 100,
			},
			wantInput: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.usage.TotalInputTokens(); got != tt.wantInput {
				t.Errorf("TotalInputTokens() = %d, want %d", got, tt.wantInput)
			}
		})
	}
}
