package engine

import (
	"sort"
	"strconv"
	"time"

	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/store"
)

type Engine struct {
	store *store.Store
	loc   *time.Location
}

func New(s *store.Store, loc *time.Location) *Engine {
	return &Engine{store: s, loc: loc}
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
