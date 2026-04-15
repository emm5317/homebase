package providers

import (
	"context"
	"testing"
	"time"

	"github.com/emm5317/homebase/internal/config"
)

// rotation used across all tests (7 meals so indices are easy to verify)
var testRotation = []string{
	"Spaghetti",   // day 0
	"Tacos",       // day 1
	"Stir fry",    // day 2
	"Pizza",       // day 3
	"Leftovers",   // day 4
	"Grill night", // day 5
	"Soup",        // day 6
}

const testStartDate = "2026-04-13" // Monday

func newTestMeals(rotation []string, startDate string) *Meals {
	loc := time.UTC
	m := NewMeals(config.MealsConfig{
		Rotation:  rotation,
		StartDate: startDate,
	}, loc)
	return m
}

// fixedAt sets the provider's now clock to a specific date+hour in UTC.
func fixedAt(m *Meals, year, month, day, hour, minute int) {
	loc := time.UTC
	t := time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc)
	m.now = func() time.Time { return t }
}

// TestMealsDayIndexMath verifies rotation wrapping across a week boundary.
func TestMealsDayIndexMath(t *testing.T) {
	tests := []struct {
		name     string
		date     string // YYYY-MM-DD
		wantMeal string
	}{
		// start + 0 → index 0
		{"start day (day 0)", "2026-04-13", "Spaghetti"},
		// start + 1 → index 1
		{"day 1", "2026-04-14", "Tacos"},
		// start + 7 → index 0 again (wrap)
		{"day 7 wraps to index 0", "2026-04-20", "Spaghetti"},
		// start + 8 → index 1
		{"day 8 wraps to index 1", "2026-04-21", "Tacos"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMeals(testRotation, testStartDate)
			// Parse date
			d, err := time.ParseInLocation("2006-01-02", tc.date, time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			// Set time to 15:30 (inside "tonight" window)
			m.now = func() time.Time {
				return time.Date(d.Year(), d.Month(), d.Day(), 15, 30, 0, 0, time.UTC)
			}

			cards, err := m.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(cards) != 1 {
				t.Fatalf("expected 1 card, got %d", len(cards))
			}
			want := "Tonight: " + tc.wantMeal
			if cards[0].Title != want {
				t.Errorf("Title = %q, want %q", cards[0].Title, want)
			}
		})
	}
}

func TestMealsDisabledCases(t *testing.T) {
	loc := time.UTC

	t.Run("empty rotation", func(t *testing.T) {
		m := NewMeals(config.MealsConfig{Rotation: nil, StartDate: testStartDate}, loc)
		m.now = func() time.Time { return time.Date(2026, 4, 15, 16, 0, 0, 0, loc) }
		cards, err := m.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(cards) != 0 {
			t.Errorf("expected 0 cards for empty rotation, got %d", len(cards))
		}
	})

	t.Run("invalid start_date", func(t *testing.T) {
		m := NewMeals(config.MealsConfig{Rotation: testRotation, StartDate: "not-a-date"}, loc)
		m.now = func() time.Time { return time.Date(2026, 4, 15, 16, 0, 0, 0, loc) }
		cards, err := m.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(cards) != 0 {
			t.Errorf("expected 0 cards for invalid start_date, got %d", len(cards))
		}
	})

	t.Run("before start date", func(t *testing.T) {
		m := newTestMeals(testRotation, "2026-04-20") // start in the future
		m.now = func() time.Time { return time.Date(2026, 4, 15, 16, 0, 0, 0, loc) }
		cards, err := m.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(cards) != 0 {
			t.Errorf("expected 0 cards before start date, got %d", len(cards))
		}
	})
}

func TestMealsHourWindows(t *testing.T) {
	tests := []struct {
		name       string
		hour, min  int
		wantCards  int
		wantPrefix string
	}{
		{"14:59 → no card", 14, 59, 0, ""},
		{"15:00 → tonight", 15, 0, 1, "Tonight:"},
		{"18:59 → tonight", 18, 59, 1, "Tonight:"},
		{"19:00 → tomorrow", 19, 0, 1, "Tomorrow:"},
		{"21:59 → tomorrow", 21, 59, 1, "Tomorrow:"},
		{"22:00 → no card", 22, 0, 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMeals(testRotation, testStartDate)
			// Use 2026-04-15 (day 2 = Stir fry) at the specified hour
			m.now = func() time.Time {
				return time.Date(2026, 4, 15, tc.hour, tc.min, 0, 0, time.UTC)
			}

			cards, err := m.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(cards) != tc.wantCards {
				t.Fatalf("len(cards) = %d, want %d", len(cards), tc.wantCards)
			}
			if tc.wantCards == 0 {
				return
			}
			title := cards[0].Title
			if len(title) < len(tc.wantPrefix) || title[:len(tc.wantPrefix)] != tc.wantPrefix {
				t.Errorf("Title = %q, want prefix %q", title, tc.wantPrefix)
			}
		})
	}
}

func TestMealsTomorrowMeal(t *testing.T) {
	// On 2026-04-15 (day 2 = "Stir fry") at 19:30 → tomorrow is day 3 = "Pizza"
	m := newTestMeals(testRotation, testStartDate)
	m.now = func() time.Time {
		return time.Date(2026, 4, 15, 19, 30, 0, 0, time.UTC)
	}

	cards, err := m.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	want := "Tomorrow: Pizza"
	if cards[0].Title != want {
		t.Errorf("Title = %q, want %q", cards[0].Title, want)
	}
}

func TestMealsCardFields(t *testing.T) {
	// Verify card shape: source, type, icon, color, time window
	m := newTestMeals(testRotation, testStartDate)
	m.now = func() time.Time {
		return time.Date(2026, 4, 15, 16, 0, 0, 0, time.UTC)
	}

	got, err := m.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 card, got %d", len(got))
	}
	c := got[0]
	if c.Source != "meals" {
		t.Errorf("Source = %q, want meals", c.Source)
	}
	if c.Type != "info" {
		t.Errorf("Type = %q, want info", c.Type)
	}
	if c.Icon != "🍽️" {
		t.Errorf("Icon = %q, want 🍽️", c.Icon)
	}
	if c.Color != "#4a90d9" {
		t.Errorf("Color = %q, want #4a90d9", c.Color)
	}
	if c.TimeWindow == nil {
		t.Fatal("TimeWindow is nil")
	}
	if c.TimeWindow.ActiveFrom != "15:00" || c.TimeWindow.ActiveUntil != "22:00" {
		t.Errorf("TimeWindow = {%s, %s}, want {15:00, 22:00}",
			c.TimeWindow.ActiveFrom, c.TimeWindow.ActiveUntil)
	}
}
