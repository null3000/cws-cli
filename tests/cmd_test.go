package tests

import (
	"testing"

	"github.com/null3000/cws-cli/cmd"
)

// --- FormatState tests ---

func TestFormatState(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PUBLISHED", "Published"},
		{"PENDING_REVIEW", "Pending Review"},
		{"DRAFT", "Draft"},
		{"DEFERRED", "Staged (Deferred)"},
		{"STATE_UNSPECIFIED", "Unknown"},
		{"", ""},
		{"SOME_NEW_STATE", "SOME_NEW_STATE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cmd.FormatState(tt.input)
			if got != tt.want {
				t.Errorf("FormatState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Version tests ---

func TestVersionVariable(t *testing.T) {
	cmd.Version = "1.2.3"
	if cmd.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", cmd.Version, "1.2.3")
	}
}
