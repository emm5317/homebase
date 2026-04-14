package engine

import (
	"testing"
	"time"

	"github.com/emm5317/homebase/internal/cards"
)

func TestParseTimeMinutes(t *testing.T) {
	tests := map[string]int{
		"00:00": 0,
		"05:00": 300,
		"09:30": 570,
		"17:45": 1065,
		"23:59": 1439,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := parseTimeMinutes(input); got != want {
				t.Errorf("parseTimeMinutes(%q) = %d, want %d", input, got, want)
			}
		})
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		max      string
		wantMode string
		wantMax  int
	}{
		{"empty defaults to mobile", "", "", "mobile", 15},
		{"display default max", "display", "", "display", 6},
		{"eink default max", "eink", "", "eink", 4},
		{"explicit max overrides", "display", "10", "display", 10},
		{"invalid max uses default", "mobile", "abc", "mobile", 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := ParseQuery(tc.mode, tc.max)
			if q.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", q.Mode, tc.wantMode)
			}
			if q.Max != tc.wantMax {
				t.Errorf("Max = %d, want %d", q.Max, tc.wantMax)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	loc := time.UTC
	e := &Engine{loc: loc}
	now := time.Date(2026, 4, 15, 7, 30, 0, 0, loc) // 7:30 AM

	tests := []struct {
		name string
		card cards.Card
		want bool
	}{
		{
			"no time window — always active",
			cards.Card{},
			true,
		},
		{
			"all-day window — active",
			cards.Card{TimeWindow: &cards.TimeWindow{AllDay: true}},
			true,
		},
		{
			"within window 5-9 AM",
			cards.Card{TimeWindow: &cards.TimeWindow{ActiveFrom: "05:00", ActiveUntil: "09:00"}},
			true,
		},
		{
			"before window starts",
			cards.Card{TimeWindow: &cards.TimeWindow{ActiveFrom: "08:00", ActiveUntil: "12:00"}},
			false,
		},
		{
			"after window ends",
			cards.Card{TimeWindow: &cards.TimeWindow{ActiveFrom: "05:00", ActiveUntil: "07:00"}},
			false,
		},
		{
			"expired card",
			cards.Card{ExpiresAt: timePtr(now.Add(-1 * time.Hour))},
			false,
		},
		{
			"not yet expired",
			cards.Card{ExpiresAt: timePtr(now.Add(1 * time.Hour))},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.isActive(tc.card, now); got != tc.want {
				t.Errorf("isActive = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAdjustPriorities(t *testing.T) {
	loc := time.UTC
	e := &Engine{loc: loc}

	morning := time.Date(2026, 4, 15, 6, 0, 0, 0, loc)  // 6 AM
	evening := time.Date(2026, 4, 15, 17, 0, 0, 0, loc) // 5 PM
	night := time.Date(2026, 4, 15, 23, 0, 0, 0, loc)   // 11 PM

	t.Run("morning boosts Metra priority", func(t *testing.T) {
		input := []cards.Card{{Source: "metra", Priority: 5, Type: "status"}}
		out := e.adjustPriorities(input, morning)
		if out[0].Priority >= 5 {
			t.Errorf("expected priority < 5, got %d", out[0].Priority)
		}
	})

	t.Run("evening commute boosts Metra", func(t *testing.T) {
		input := []cards.Card{{Source: "metra", Priority: 5, Type: "status"}}
		out := e.adjustPriorities(input, evening)
		if out[0].Priority >= 5 {
			t.Errorf("expected priority < 5, got %d", out[0].Priority)
		}
	})

	t.Run("night demotes non-weather cards", func(t *testing.T) {
		input := []cards.Card{{Source: "skylight", Priority: 5, Type: "list"}}
		out := e.adjustPriorities(input, night)
		if out[0].Priority <= 5 {
			t.Errorf("expected priority > 5, got %d", out[0].Priority)
		}
	})

	t.Run("alerts always priority 0", func(t *testing.T) {
		input := []cards.Card{{Source: "weather", Priority: 5, Type: "alert"}}
		out := e.adjustPriorities(input, night)
		if out[0].Priority != 0 {
			t.Errorf("alert priority = %d, want 0", out[0].Priority)
		}
	})
}

func timePtr(t time.Time) *time.Time { return &t }
