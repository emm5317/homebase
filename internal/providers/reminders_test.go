package providers

import (
	"context"
	"testing"
	"time"

	"github.com/emm5317/homebase/internal/config"
)

func TestRemindersNextOccurrence(t *testing.T) {
	loc := time.UTC

	t.Run("MM-DD before today this year", func(t *testing.T) {
		// "today" is April 15; date is April 14 → next is April 14 next year
		today := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
		next, annual, err := parseReminderDate("04-14", today, loc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !annual {
			t.Error("expected annual=true")
		}
		want := time.Date(2027, 4, 14, 0, 0, 0, 0, loc)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("MM-DD equal to today", func(t *testing.T) {
		// "today" is April 15; date is April 15 → this year
		today := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
		next, annual, err := parseReminderDate("04-15", today, loc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !annual {
			t.Error("expected annual=true")
		}
		want := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("MM-DD after today this year", func(t *testing.T) {
		// "today" is April 15; date is May 12 → this year
		today := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
		next, annual, err := parseReminderDate("05-12", today, loc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !annual {
			t.Error("expected annual=true")
		}
		want := time.Date(2026, 5, 12, 0, 0, 0, 0, loc)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})
}

func TestRemindersOneOffPastDate(t *testing.T) {
	loc := time.UTC
	// Fixed "now" at April 15 10:00 AM
	fixedNow := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)

	r := NewReminders([]config.ReminderEntry{
		{Date: "2026-04-13", Text: "Past event", Icon: "🎉"},
	}, loc)
	r.now = func() time.Time { return fixedNow }

	got, err := r.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 cards for past one-off, got %d", len(got))
	}
}

func TestRemindersThresholdBands(t *testing.T) {
	loc := time.UTC
	// We'll set "now" to April 15, then set the event date to various offsets.

	tests := []struct {
		name          string
		eventDate     string
		nowOffset     int // days from April 15
		wantCards     int
		wantPriority  int
		wantColorPart string
	}{
		{"today (0 days)", "2026-04-15", 0, 1, 2, "#e86f5a"},
		{"1 day away", "2026-04-16", 0, 1, 3, "#d4943a"},
		{"3 days away", "2026-04-18", 0, 1, 5, "#d4943a"},
		{"7 days away", "2026-04-22", 0, 1, 7, "#4a90d9"},
		{"8 days away — no card", "2026-04-23", 0, 0, 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseNow := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)
			fixedNow := baseNow.AddDate(0, 0, tc.nowOffset)

			r := NewReminders([]config.ReminderEntry{
				{Date: tc.eventDate, Text: "Test event", Icon: "🎂"},
			}, loc)
			r.now = func() time.Time { return fixedNow }

			got, err := r.Fetch(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantCards {
				t.Errorf("len(cards) = %d, want %d", len(got), tc.wantCards)
				return
			}
			if tc.wantCards == 0 {
				return
			}
			if got[0].Priority != tc.wantPriority {
				t.Errorf("Priority = %d, want %d", got[0].Priority, tc.wantPriority)
			}
			if got[0].Color != tc.wantColorPart {
				t.Errorf("Color = %q, want %q", got[0].Color, tc.wantColorPart)
			}
		})
	}
}

func TestRemindersEmptyList(t *testing.T) {
	loc := time.UTC
	r := NewReminders(nil, loc)
	r.now = func() time.Time { return time.Date(2026, 4, 15, 10, 0, 0, 0, loc) }

	got, err := r.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 cards for empty list, got %d", len(got))
	}
}

func TestRemindersDefaultIcon(t *testing.T) {
	loc := time.UTC
	fixedNow := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)

	r := NewReminders([]config.ReminderEntry{
		{Date: "2026-04-15", Text: "No icon event"},
	}, loc)
	r.now = func() time.Time { return fixedNow }

	got, err := r.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 card, got %d", len(got))
	}
	if got[0].Icon != "🎂" {
		t.Errorf("Icon = %q, want 🎂", got[0].Icon)
	}
}

func TestRemindersSubtitle(t *testing.T) {
	loc := time.UTC
	fixedNow := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)

	r := NewReminders([]config.ReminderEntry{
		{Date: "2026-04-18", Text: "Test event", Icon: "🎉"},
	}, loc)
	r.now = func() time.Time { return fixedNow }

	got, err := r.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 card, got %d", len(got))
	}
	// April 18, 2026 is a Saturday
	wantSubtitle := "Sat, Apr 18"
	if got[0].Subtitle != wantSubtitle {
		t.Errorf("Subtitle = %q, want %q", got[0].Subtitle, wantSubtitle)
	}
}

func TestRemindersTodayTitle(t *testing.T) {
	loc := time.UTC
	fixedNow := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)

	r := NewReminders([]config.ReminderEntry{
		{Date: "2026-04-15", Text: "Mom's birthday", Icon: "🎂"},
	}, loc)
	r.now = func() time.Time { return fixedNow }

	got, err := r.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 card, got %d", len(got))
	}
	wantTitle := "🎂 Mom's birthday today"
	if got[0].Title != wantTitle {
		t.Errorf("Title = %q, want %q", got[0].Title, wantTitle)
	}
	if got[0].Type != "alert" {
		t.Errorf("Type = %q, want alert", got[0].Type)
	}
}
