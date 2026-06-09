package tui

import "testing"

func TestIsUsableSummary(t *testing.T) {
	tests := []struct {
		name    string
		summary string
		want    bool
	}{
		// Junk seen in live SteerKit /sessions responses — must be rejected
		// so it never clobbers a session's real title.
		{
			name:    "transcript stats dump",
			summary: "Session transcript: 132 user exchanges, 969 assistant responses.\n\n## User messages",
			want:    false,
		},
		{
			name:    "transcript stats lowercase",
			summary: "session transcript: 1 user exchanges, 4 assistant responses.",
			want:    false,
		},
		{
			name:    "raw json object dump",
			summary: `{"session_id":"db4e5dbb-85cd-4155-8302-9d134c9573d7","transcript_path":"/Users/a`,
			want:    false,
		},
		{
			name:    "json array dump",
			summary: `[{"role":"user"}]`,
			want:    false,
		},
		{
			name:    "empty",
			summary: "",
			want:    false,
		},
		{
			name:    "whitespace only",
			summary: "   \n\t ",
			want:    false,
		},
		// Real summaries — must be accepted.
		{
			name:    "human summary",
			summary: "Refactored the auth middleware and added token refresh handling",
			want:    true,
		},
		{
			name:    "leading whitespace then real text",
			summary: "  Fixed the tmux path resolution bug on Apple Silicon",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUsableSummary(tt.summary); got != tt.want {
				t.Errorf("isUsableSummary(%q) = %v, want %v", tt.summary, got, tt.want)
			}
		})
	}
}
