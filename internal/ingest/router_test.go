package ingest

import (
	"strings"
	"testing"
)

func TestRouteByTag(t *testing.T) {
	tests := map[string]string{
		"grocery":  "list", // grocery card type
		"chore":    "list",
		"note":     "info",
		"reminder": "info",
		"calendar": "event",
		"cal":      "event",
		"fridge":   "info",
	}

	for tag, wantType := range tests {
		t.Run(tag, func(t *testing.T) {
			cards, err := Route(Payload{Tag: tag, Subject: "Test", Body: "Body"}, "Eric")
			if err != nil {
				t.Fatalf("Route returned error: %v", err)
			}
			if len(cards) == 0 {
				t.Fatal("Route returned no cards")
			}
			if cards[0].Type != wantType {
				t.Errorf("Type = %q, want %q", cards[0].Type, wantType)
			}
		})
	}
}

func TestClassifyAndRoute(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		body    string
		// We assert which parser was invoked by checking the resulting card title/icon
		wantTitleContains string
	}{
		{"grocery keyword", "shopping list", "milk, eggs", "Grocery"},
		{"pick up keyword", "things to pick up", "milk", "Grocery"},
		{"chore keyword", "todo", "vacuum", "To-Do"},
		{"please do", "please don't forget the laundry", "", "To-Do"},
		{"calendar — practice", "Soccer practice moved", "to 4pm", "Soccer"},
		{"calendar — appointment", "Dentist appointment", "tomorrow", "Dentist"},
		{"reminder keyword", "Reminder for tomorrow", "", "Reminder"},
		{"unknown — defaults to note", "Random thought", "random body", "Random"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cards, err := classifyAndRoute(Payload{Subject: tc.subject, Body: tc.body}, "Eric")
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if len(cards) == 0 {
				t.Fatal("no cards returned")
			}
			combined := cards[0].Title + " " + cards[0].Subtitle
			if !strings.Contains(combined, tc.wantTitleContains) {
				t.Errorf("title %q should contain %q", combined, tc.wantTitleContains)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	c1, _ := Route(Payload{Tag: "grocery", Body: "Milk\nEggs"}, "Eric")
	c2, _ := Route(Payload{Tag: "grocery", Body: "Milk\nEggs"}, "Eric")

	// Same content (ignoring time-based ID) → same hash
	h1 := contentHash(c1[0])
	h2 := contentHash(c2[0])
	if h1 != h2 {
		t.Errorf("identical content should produce identical hashes, got %q vs %q", h1, h2)
	}

	// Different content → different hash
	c3, _ := Route(Payload{Tag: "grocery", Body: "Bread"}, "Eric")
	h3 := contentHash(c3[0])
	if h1 == h3 {
		t.Errorf("different content should produce different hashes, both %q", h1)
	}
}
