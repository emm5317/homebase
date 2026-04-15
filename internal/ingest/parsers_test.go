package ingest

import (
	"strings"
	"testing"
	"time"
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

// TestParseFridgeItem tests the low-level item parser.
func TestParseFridgeItem(t *testing.T) {
	tests := []struct {
		input   string
		name    string
		ttl     int
	}{
		{"milk 3d", "milk", 3},
		{"Leftover pasta 2d", "Leftover pasta", 2},
		{"rotisserie chicken", "rotisserie chicken", 3},  // default TTL
		{"cheese 300d", "cheese", 30},                    // clamped to max
		{"eggs 5d", "eggs", 5},
		{"  cheese  ", "cheese", 3},                      // trim whitespace, default TTL
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			name, ttl := parseFridgeItem(tc.input)
			if name != tc.name {
				t.Errorf("name = %q, want %q", name, tc.name)
			}
			if ttl != tc.ttl {
				t.Errorf("ttl = %d, want %d", ttl, tc.ttl)
			}
		})
	}
}

// TestParseFridge tests the full fridge parser against the spec examples.
func TestParseFridge(t *testing.T) {
	t.Run("single item with ttl", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "milk 3d"}, "Eric")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		c := cs[0]
		if c.Title != "Fridge: milk" {
			t.Errorf("Title = %q", c.Title)
		}
		if c.Source != "fridge" {
			t.Errorf("Source = %q, want fridge", c.Source)
		}
		if c.Type != "info" {
			t.Errorf("Type = %q, want info", c.Type)
		}
		if c.Priority != 7 {
			t.Errorf("Priority = %d, want 7", c.Priority)
		}
		if c.Icon != "\U0001F9C0" {
			t.Errorf("Icon = %q, want 🧀", c.Icon)
		}
		if c.Color != "#5a9e78" {
			t.Errorf("Color = %q, want #5a9e78 (green, ttl=3)", c.Color)
		}
		if c.ExpiresAt == nil {
			t.Fatal("ExpiresAt should be set")
		}
		expectedExpiry := time.Now().Add(3 * 24 * time.Hour)
		diff := c.ExpiresAt.Sub(expectedExpiry)
		if diff < -5*time.Second || diff > 5*time.Second {
			t.Errorf("ExpiresAt off by %v", diff)
		}
		if !strings.HasPrefix(c.Subtitle, "Use by ") {
			t.Errorf("Subtitle = %q, want 'Use by ...'", c.Subtitle)
		}
		if c.CreatedBy != "Eric" {
			t.Errorf("CreatedBy = %q", c.CreatedBy)
		}
		if c.CreatedVia != "email" {
			t.Errorf("CreatedVia = %q", c.CreatedVia)
		}
		if !c.Persistent {
			t.Error("Persistent should be true")
		}
	})

	t.Run("leftover pasta with ttl 2", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "Leftover pasta 2d"}, "Kayla")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		c := cs[0]
		if c.Title != "Fridge: Leftover pasta" {
			t.Errorf("Title = %q", c.Title)
		}
		// ttl==2 → yellow
		if c.Color != "#d4943a" {
			t.Errorf("Color = %q, want #d4943a (yellow, ttl=2)", c.Color)
		}
	})

	t.Run("rotisserie chicken default ttl", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "rotisserie chicken"}, "Eric")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		c := cs[0]
		if c.Title != "Fridge: rotisserie chicken" {
			t.Errorf("Title = %q", c.Title)
		}
		if c.Color != "#5a9e78" {
			t.Errorf("Color = %q, want #5a9e78 (green, ttl=3 default)", c.Color)
		}
	})

	t.Run("comma-separated items", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "milk 3d, eggs 5d, cheese"}, "Eric")
		if len(cs) != 3 {
			t.Fatalf("expected 3 cards, got %d", len(cs))
		}
		if cs[0].Title != "Fridge: milk" {
			t.Errorf("cs[0].Title = %q", cs[0].Title)
		}
		if cs[1].Title != "Fridge: eggs" {
			t.Errorf("cs[1].Title = %q", cs[1].Title)
		}
		if cs[2].Title != "Fridge: cheese" {
			t.Errorf("cs[2].Title = %q", cs[2].Title)
		}
		// eggs ttl=5 → green
		if cs[1].Color != "#5a9e78" {
			t.Errorf("eggs color = %q, want #5a9e78", cs[1].Color)
		}
	})

	t.Run("ttl clamped at 30", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "cheese 300d"}, "Eric")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		expectedExpiry := time.Now().Add(30 * 24 * time.Hour)
		diff := cs[0].ExpiresAt.Sub(expectedExpiry)
		if diff < -5*time.Second || diff > 5*time.Second {
			t.Errorf("ExpiresAt not clamped to 30d, diff=%v", diff)
		}
	})

	t.Run("ttl 1 is red", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "leftovers 1d"}, "Eric")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		if cs[0].Color != "#e86f5a" {
			t.Errorf("Color = %q, want #e86f5a (red, ttl=1)", cs[0].Color)
		}
	})

	t.Run("empty lines skipped", func(t *testing.T) {
		cs := parseFridge(Payload{Body: "milk\n\n\nbread 2d"}, "Eric")
		if len(cs) != 2 {
			t.Fatalf("expected 2 cards, got %d", len(cs))
		}
	})

	t.Run("falls back to subject when body empty", func(t *testing.T) {
		cs := parseFridge(Payload{Subject: "yogurt 4d"}, "Eric")
		if len(cs) != 1 {
			t.Fatalf("expected 1 card, got %d", len(cs))
		}
		if cs[0].Title != "Fridge: yogurt" {
			t.Errorf("Title = %q", cs[0].Title)
		}
	})
}
