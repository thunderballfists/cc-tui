package daemon

import (
	"testing"
)

func TestEncodeProjectPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/Users/allan.beihl", "-Users-allan-beihl"},
		{"/Users/allan.beihl/traffic_api", "-Users-allan-beihl-traffic-api"},
		{"/Users/allan.beihl/.config/tmux", "-Users-allan-beihl--config-tmux"},
	}
	for _, tt := range tests {
		got := EncodeProjectPath(tt.input)
		if got != tt.want {
			t.Errorf("EncodeProjectPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadHistory(t *testing.T) {
	sessions, err := LoadHistory("testdata/history.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	// Should be sorted by last_ts desc
	if sessions[0].ID != "def-456" {
		t.Errorf("first session = %q, want def-456", sessions[0].ID)
	}
}
