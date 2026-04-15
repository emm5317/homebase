package engine

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/store"
)

type Engine struct {
	store      *store.Store
	loc        *time.Location
	quietRange *quietTimeRange
}

// quietTimeRange holds a parsed "HH:MM-HH:MM" quiet-hours range.
type quietTimeRange struct {
	fromMinutes  int
	untilMinutes int
	crossMidnight bool
}

// parseQuietHours parses a "HH:MM-HH:MM" string into a quietTimeRange.
// Returns nil and an error if the string is malformed; nil and nil for an
// empty string (disabled).
func parseQuietHours(s string) (*quietTimeRange, error) {
	if s == "" {
		return nil, nil
	}
	// Expected format: "HH:MM-HH:MM" (11 chars, hyphen at index 5)
	if len(s) != 11 || s[5] != '-' {
		return nil, fmt.Errorf("quiet_hours: expected HH:MM-HH:MM, got %q", s)
	}
	fromStr := s[:5]
	untilStr := s[6:]
	if !isValidHHMM(fromStr) || !isValidHHMM(untilStr) {
		return nil, fmt.Errorf("quiet_hours: expected HH:MM-HH:MM, got %q", s)
	}
	from := parseTimeMinutes(fromStr)
	until := parseTimeMinutes(untilStr)
	return &quietTimeRange{
		fromMinutes:   from,
		untilMinutes:  until,
		crossMidnight: from > until,
	}, nil
}

// isValidHHMM checks that s looks like "HH:MM".
func isValidHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:5])
	return err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59
}

// inRange returns true if the given time falls within the quiet window.
func (q *quietTimeRange) inRange(t time.Time) bool {
	m := t.Hour()*60 + t.Minute()
	if q.crossMidnight {
		// e.g. 21:00–06:00 → active when m >= 21*60 OR m < 6*60
		return m >= q.fromMinutes || m < q.untilMinutes
	}
	return m >= q.fromMinutes && m < q.untilMinutes
}

// isUrgent returns true if a card should pass through quiet hours.
func isUrgent(c cards.Card) bool {
	return c.AlertLevel == "severe" || c.Priority <= 1
}

func New(s *store.Store, loc *time.Location, quietHours string) *Engine {
	qr, _ := parseQuietHours(quietHours)
	return &Engine{store: s, loc: loc, quietRange: qr}
}

type QueryParams struct {
	Mode string // "display", "mobile", "eink"
	Max  int
}

func ParseQuery(mode, maxStr string) QueryParams {
	q := QueryParams{Mode: mode}
	if q.Mode == "" {
		q.Mode = "mobile"
	}
	q.Max, _ = strconv.Atoi(maxStr)
	if q.Max <= 0 {
		switch q.Mode {
		case "eink":
			q.Max = 4
		case "display":
			q.Max = 6
		default:
			q.Max = 15
		}
	}
	return q
}

func (e *Engine) Cards(params QueryParams) []cards.Card {
	all := e.store.AllCards()
	now := time.Now().In(e.loc)

	// Filter by time window
	var active []cards.Card
	for _, c := range all {
		if e.isActive(c, now) {
			active = append(active, c)
		}
	}

	// Apply quiet-hours filter: during quiet window only severe/priority-0-1 cards pass.
	if e.quietRange != nil && e.quietRange.inRange(now) {
		var urgent []cards.Card
		for _, c := range active {
			if isUrgent(c) {
				urgent = append(urgent, c)
			}
		}
		active = urgent
	}

	// Apply priority adjustments based on time of day
	adjusted := e.adjustPriorities(active, now)

	// Sort by priority (lower = higher priority)
	sort.Slice(adjusted, func(i, j int) bool {
		if adjusted[i].Priority != adjusted[j].Priority {
			return adjusted[i].Priority < adjusted[j].Priority
		}
		// Alerts first, then by source order
		if adjusted[i].Type == "alert" && adjusted[j].Type != "alert" {
			return true
		}
		return false
	})

	// Limit to max cards
	if len(adjusted) > params.Max {
		adjusted = adjusted[:params.Max]
	}

	return adjusted
}

func (e *Engine) isActive(c cards.Card, now time.Time) bool {
	// Expired cards are never active
	if c.ExpiresAt != nil && c.ExpiresAt.Before(now) {
		return false
	}

	tw := c.TimeWindow
	if tw == nil || tw.AllDay {
		return true
	}

	currentMinutes := now.Hour()*60 + now.Minute()

	if tw.ActiveFrom != "" {
		from := parseTimeMinutes(tw.ActiveFrom)
		if currentMinutes < from {
			return false
		}
	}
	if tw.ActiveUntil != "" {
		until := parseTimeMinutes(tw.ActiveUntil)
		if currentMinutes > until {
			return false
		}
	}

	return true
}

func (e *Engine) adjustPriorities(cc []cards.Card, now time.Time) []cards.Card {
	hour := now.Hour()
	result := make([]cards.Card, len(cc))
	copy(result, cc)

	for i := range result {
		c := &result[i]

		// Alerts always stay at priority 0
		if c.Type == "alert" {
			c.Priority = 0
			continue
		}

		switch {
		case hour >= 5 && hour < 8:
			// Early morning: boost Metra and weather
			if c.Source == "metra" {
				c.Priority = max(1, c.Priority-2)
			}
			if c.Source == "weather" {
				c.Priority = max(1, c.Priority-1)
			}

		case hour >= 8 && hour < 16:
			// Daytime: boost calendar, lists, chores
			if c.Source == "skylight" {
				c.Priority = max(2, c.Priority-1)
			}

		case hour >= 16 && hour < 19:
			// Evening commute: boost Metra return trains
			if c.Source == "metra" {
				c.Priority = max(1, c.Priority-2)
			}

		case hour >= 19 && hour < 22:
			// Evening: boost tomorrow preview, chores
			if c.Source == "garbage" {
				c.Priority = max(2, c.Priority-2)
			}

		case hour >= 22 || hour < 5:
			// Night: only weather and alerts matter
			if c.Source != "weather" && c.Type != "alert" {
				c.Priority = c.Priority + 5
			}
		}
	}

	return result
}

func parseTimeMinutes(s string) int {
	// Parse "HH:MM" format
	if len(s) < 5 {
		return 0
	}
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:5])
	return h*60 + m
}
