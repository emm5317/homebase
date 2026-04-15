package engine

import (
	"fmt"
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

func TestParseQuietHours(t *testing.T) {
	t.Run("empty string → disabled", func(t *testing.T) {
		qr, err := parseQuietHours("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qr != nil {
			t.Error("expected nil for empty string")
		}
	})

	t.Run("valid same-day range 22:00-23:00", func(t *testing.T) {
		qr, err := parseQuietHours("22:00-23:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qr == nil {
			t.Fatal("expected non-nil")
		}
		if qr.crossMidnight {
			t.Error("should not be cross-midnight")
		}
	})

	t.Run("valid cross-midnight range 21:00-06:00", func(t *testing.T) {
		qr, err := parseQuietHours("21:00-06:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qr == nil {
			t.Fatal("expected non-nil")
		}
		if !qr.crossMidnight {
			t.Error("expected cross-midnight")
		}
	})

	t.Run("malformed → error, disabled", func(t *testing.T) {
		qr, err := parseQuietHours("2100-0600")
		if err == nil {
			t.Error("expected an error")
		}
		if qr != nil {
			t.Error("expected nil on error")
		}
	})
}

func TestQuietHoursInRange(t *testing.T) {
	loc := time.UTC
	qr, _ := parseQuietHours("21:00-06:00")

	tests := []struct {
		hour int
		min  int
		want bool
	}{
		{21, 0, true},
		{22, 0, true},
		{0, 0, true},
		{5, 59, true},
		{6, 0, false},
		{12, 0, false},
		{20, 59, false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%02d:%02d", tc.hour, tc.min), func(t *testing.T) {
			tm := time.Date(2026, 4, 15, tc.hour, tc.min, 0, 0, loc)
			if got := qr.inRange(tm); got != tc.want {
				t.Errorf("inRange(%02d:%02d) = %v, want %v", tc.hour, tc.min, got, tc.want)
			}
		})
	}
}

func TestQuietHoursFiltering(t *testing.T) {
	loc := time.UTC

	severeCard := cards.Card{
		ID:         "c-severe",
		Source:     "weather",
		Type:       "alert",
		Priority:   0,
		AlertLevel: "severe",
	}
	prio1Card := cards.Card{
		ID:       "c-prio1",
		Source:   "metra",
		Type:     "alert",
		Priority: 1,
	}
	infoCard := cards.Card{
		ID:       "c-info",
		Source:   "meals",
		Type:     "info",
		Priority: 4,
	}

	t.Run("22:00 inside quiet window drops info, keeps severe and prio1", func(t *testing.T) {
		e := &Engine{loc: loc}
		var err error
		e.quietRange, err = parseQuietHours("21:00-06:00")
		if err != nil {
			t.Fatal(err)
		}

		now := time.Date(2026, 4, 15, 22, 0, 0, 0, loc)
		active := []cards.Card{severeCard, prio1Card, infoCard}

		var result []cards.Card
		if e.quietRange != nil && e.quietRange.inRange(now) {
			for _, c := range active {
				if isUrgent(c) {
					result = append(result, c)
				}
			}
		} else {
			result = active
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 cards through quiet filter, got %d", len(result))
		}
		for _, c := range result {
			if c.ID == "c-info" {
				t.Error("info card should have been dropped")
			}
		}
	})

	t.Run("12:00 outside quiet window passes all cards", func(t *testing.T) {
		e := &Engine{loc: loc}
		var err error
		e.quietRange, err = parseQuietHours("21:00-06:00")
		if err != nil {
			t.Fatal(err)
		}

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, loc)
		active := []cards.Card{severeCard, prio1Card, infoCard}

		var result []cards.Card
		if e.quietRange != nil && e.quietRange.inRange(now) {
			for _, c := range active {
				if isUrgent(c) {
					result = append(result, c)
				}
			}
		} else {
			result = active
		}

		if len(result) != 3 {
			t.Fatalf("expected all 3 cards outside quiet window, got %d", len(result))
		}
	})

	t.Run("empty quiet_hours passes all cards at any hour", func(t *testing.T) {
		e := &Engine{loc: loc}
		e.quietRange, _ = parseQuietHours("")

		now := time.Date(2026, 4, 15, 22, 0, 0, 0, loc)
		active := []cards.Card{severeCard, prio1Card, infoCard}

		var result []cards.Card
		if e.quietRange != nil && e.quietRange.inRange(now) {
			for _, c := range active {
				if isUrgent(c) {
					result = append(result, c)
				}
			}
		} else {
			result = active
		}

		if len(result) != 3 {
			t.Fatalf("expected all 3 cards with no quiet hours, got %d", len(result))
		}
	})
}
