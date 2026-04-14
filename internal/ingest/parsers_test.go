package ingest

import (
	"strings"
	"testing"
)

func TestParseGrocery(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		subject   string
		wantItems []string
	}{
		{
			name:      "newline-separated",
			body:      "Milk\nBread\nEggs",
			wantItems: []string{"Milk", "Bread", "Eggs"},
		},
		{
			name:      "comma-separated",
			body:      "Milk, Bread, Eggs",
			wantItems: []string{"Milk", "Bread", "Eggs"},
		},
		{
			name:      "mixed separators",
			body:      "Milk, Bread\nEggs, Cheese",
			wantItems: []string{"Milk", "Bread", "Eggs", "Cheese"},
		},
		{
			name:      "bullet prefixes stripped",
			body:      "- Milk\n* Bread\n• Eggs",
			wantItems: []string{"Milk", "Bread", "Eggs"},
		},
		{
			name:      "numbered list",
			body:      "1. Milk\n2. Bread\n3) Eggs",
			wantItems: []string{"Milk", "Bread", "Eggs"},
		},
		{
			name:      "blank lines and whitespace ignored",
			body:      "  Milk  \n\n  Bread\n\n",
			wantItems: []string{"Milk", "Bread"},
		},
		{
			name:      "falls back to subject when body empty",
			body:      "",
			subject:   "Milk, Eggs",
			wantItems: []string{"Milk", "Eggs"},
		},
		{
			name:      "verbose item descriptions preserved",
			body:      "The good orange juice not the cheap one\nGoldfish crackers for the boys",
			wantItems: []string{"The good orange juice not the cheap one", "Goldfish crackers for the boys"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cards := parseGrocery(Payload{Body: tc.body, Subject: tc.subject}, "Grandma")
			if len(cards) != 1 {
				t.Fatalf("expected 1 card, got %d", len(cards))
			}
			got := cards[0].Items
			if len(got) != len(tc.wantItems) {
				t.Fatalf("expected %d items, got %d: %v", len(tc.wantItems), len(got), got)
			}
			for i, item := range got {
				if item.Text != tc.wantItems[i] {
					t.Errorf("item[%d] = %q, want %q", i, item.Text, tc.wantItems[i])
				}
				if item.Done {
					t.Errorf("item[%d] should not be done", i)
				}
			}
		})
	}
}

func TestParseGroceryMetadata(t *testing.T) {
	cards := parseGrocery(Payload{Body: "Milk"}, "Eric")
	c := cards[0]

	if c.Source != "email" {
		t.Errorf("Source = %q, want email", c.Source)
	}
	if c.Type != "list" {
		t.Errorf("Type = %q, want list", c.Type)
	}
	if c.CreatedBy != "Eric" {
		t.Errorf("CreatedBy = %q, want Eric", c.CreatedBy)
	}
	if c.CreatedVia != "email" {
		t.Errorf("CreatedVia = %q, want email", c.CreatedVia)
	}
	if c.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
	if !strings.Contains(c.Subtitle, "Eric") {
		t.Errorf("Subtitle %q should contain Eric", c.Subtitle)
	}
}

func TestParseChore(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		body    string
		want    string
	}{
		{"subject is chore name", "Take out trash", "", "Take out trash"},
		{"falls back to first body line", "", "Unload dishwasher\nplease", "Unload dishwasher"},
		{"empty defaults to placeholder", "", "", "New chore"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cards := parseChore(Payload{Subject: tc.subject, Body: tc.body}, "Eric")
			if cards[0].Items[0].Text != tc.want {
				t.Errorf("got %q, want %q", cards[0].Items[0].Text, tc.want)
			}
		})
	}
}

func TestParseNote(t *testing.T) {
	cards := parseNote(Payload{Subject: "Babysitter update", Body: "Both kids napped well."}, "Sarah")
	c := cards[0]

	if c.Title != "Babysitter update" {
		t.Errorf("Title = %q", c.Title)
	}
	if c.Body != "Both kids napped well." {
		t.Errorf("Body = %q", c.Body)
	}

	// Empty subject — body becomes title
	cards = parseNote(Payload{Body: "Just a quick note"}, "Sarah")
	if cards[0].Title != "Note" {
		t.Errorf("Title = %q, want Note", cards[0].Title)
	}
}
