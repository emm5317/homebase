package ingest

import "testing"

func TestExtractEmail(t *testing.T) {
	tests := map[string]string{
		"eric@example.com":                "eric@example.com",
		"Eric Smith <eric@example.com>":   "eric@example.com",
		"<eric@example.com>":              "eric@example.com",
		"  eric@example.com  ":            "eric@example.com",
		"\"Eric\" <eric@example.com>":     "eric@example.com",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := extractEmail(input); got != want {
				t.Errorf("extractEmail(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
