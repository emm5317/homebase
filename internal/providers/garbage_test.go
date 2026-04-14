package providers

import (
	"testing"
	"time"

	"github.com/emm5317/homebase/internal/config"
)

func TestApplyHolidayShift(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")

	tests := []struct {
		name        string
		holidays    []string
		pickup      time.Time
		wantWeekday time.Weekday
	}{
		{
			name:        "no holiday in week — Thursday pickup unchanged",
			holidays:    []string{"2026-01-01"},
			pickup:      time.Date(2026, 4, 16, 0, 0, 0, 0, loc), // Thu Apr 16
			wantWeekday: time.Thursday,
		},
		{
			name:        "Monday holiday in same week — shifts to Friday",
			holidays:    []string{"2026-05-25"},                  // Mon May 25 (Memorial Day)
			pickup:      time.Date(2026, 5, 28, 0, 0, 0, 0, loc), // Thu May 28
			wantWeekday: time.Friday,
		},
		{
			name:        "Thursday holiday — shifts to Friday",
			holidays:    []string{"2026-11-26"},                  // Thanksgiving (Thu)
			pickup:      time.Date(2026, 11, 26, 0, 0, 0, 0, loc),
			wantWeekday: time.Friday,
		},
		{
			name:        "Friday holiday — Thursday pickup stays Thursday",
			holidays:    []string{"2026-07-03"},                 // Fake Friday holiday
			pickup:      time.Date(2026, 7, 2, 0, 0, 0, 0, loc), // Thu Jul 2
			wantWeekday: time.Thursday,
		},
		{
			name:        "Saturday holiday — pickup unchanged (after pickup day)",
			holidays:    []string{"2026-04-18"},                  // Sat
			pickup:      time.Date(2026, 4, 16, 0, 0, 0, 0, loc), // Thu before
			wantWeekday: time.Thursday,
		},
		{
			name:        "Holiday in different week — pickup unchanged",
			holidays:    []string{"2026-07-04"},                  // Sat
			pickup:      time.Date(2026, 4, 16, 0, 0, 0, 0, loc), // Thu in different week
			wantWeekday: time.Thursday,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGarbage(config.GarbageConfig{
				PickupDay: "thursday",
				Holidays:  tc.holidays,
			}, loc)
			got := g.applyHolidayShift(tc.pickup, time.Thursday)
			if got.Weekday() != tc.wantWeekday {
				t.Errorf("got weekday %s (%s), want %s",
					got.Weekday(), got.Format("2006-01-02"), tc.wantWeekday)
			}
		})
	}
}

func TestIsRecyclingWeek(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")

	tests := []struct {
		name  string
		weeks string
		date  time.Time
		want  bool
	}{
		{"every — always true", "every", time.Date(2026, 4, 16, 0, 0, 0, 0, loc), true},
		{"even — week 16 is even", "even", time.Date(2026, 4, 16, 0, 0, 0, 0, loc), true},
		{"even — week 17 is odd", "even", time.Date(2026, 4, 23, 0, 0, 0, 0, loc), false},
		{"odd — week 17 is odd", "odd", time.Date(2026, 4, 23, 0, 0, 0, 0, loc), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGarbage(config.GarbageConfig{RecyclingWeeks: tc.weeks}, loc)
			if got := g.isRecyclingWeek(tc.date); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsYardWasteActive(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")

	tests := []struct {
		name      string
		yardWaste string
		date      time.Time
		want      bool
	}{
		{"April active", "april-november", time.Date(2026, 4, 1, 0, 0, 0, 0, loc), true},
		{"July active", "april-november", time.Date(2026, 7, 15, 0, 0, 0, 0, loc), true},
		{"November active", "april-november", time.Date(2026, 11, 30, 0, 0, 0, 0, loc), true},
		{"December inactive", "april-november", time.Date(2026, 12, 1, 0, 0, 0, 0, loc), false},
		{"March inactive", "april-november", time.Date(2026, 3, 31, 0, 0, 0, 0, loc), false},
		{"empty config", "", time.Date(2026, 7, 1, 0, 0, 0, 0, loc), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGarbage(config.GarbageConfig{YardWaste: tc.yardWaste}, loc)
			if got := g.isYardWasteActive(tc.date); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseDayOfWeek(t *testing.T) {
	tests := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"Thursday":  time.Thursday, // case-insensitive
		"FRIDAY":    time.Friday,
		"saturday":  time.Saturday,
		"notvalid":  -1,
		"":          -1,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := parseDayOfWeek(input); got != want {
				t.Errorf("parseDayOfWeek(%q) = %d, want %d", input, got, want)
			}
		})
	}
}

func TestBuildPickupItems(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	g := NewGarbage(config.GarbageConfig{}, loc)

	tests := []struct {
		recycling, yardWaste bool
		want                 string
	}{
		{false, false, "Garbage"},
		{true, false, "Garbage & recycling"},
		{false, true, "Garbage & yard waste"},
		{true, true, "Garbage, recycling & yard waste"},
	}

	for _, tc := range tests {
		got := g.buildPickupItems(tc.recycling, tc.yardWaste)
		if got != tc.want {
			t.Errorf("buildPickupItems(%v, %v) = %q, want %q", tc.recycling, tc.yardWaste, got, tc.want)
		}
	}
}
