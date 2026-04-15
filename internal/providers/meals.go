package providers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/config"
)

// Meals emits a dinner-rotation card based on a flat rotation list anchored
// to a start date. Shows "Tonight: X" from 3–7 PM and "Tomorrow: X" from
// 7–10 PM.
type Meals struct {
	rotation     []string
	startMidnight time.Time
	loc          *time.Location
	disabled     bool
	now          func() time.Time // overridable in tests
}

func NewMeals(cfg config.MealsConfig, loc *time.Location) *Meals {
	m := &Meals{
		rotation: cfg.Rotation,
		loc:      loc,
		now:      time.Now,
	}

	if len(cfg.Rotation) == 0 || cfg.StartDate == "" {
		m.disabled = true
		return m
	}

	t, err := time.ParseInLocation("2006-01-02", cfg.StartDate, loc)
	if err != nil {
		slog.Warn("meals: invalid start_date, provider disabled",
			"start_date", cfg.StartDate, "error", err)
		m.disabled = true
		return m
	}

	m.startMidnight = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return m
}

func (m *Meals) Name() string            { return "meals" }
func (m *Meals) Interval() time.Duration { return 1 * time.Hour }

func (m *Meals) Fetch(ctx context.Context) ([]cards.Card, error) {
	if m.disabled {
		return nil, nil
	}

	now := m.now().In(m.loc)
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, m.loc)

	daysSinceStart := int(todayMidnight.Sub(m.startMidnight).Hours() / 24)
	if daysSinceStart < 0 {
		return nil, nil
	}

	hour := now.Hour()
	n := len(m.rotation)

	switch {
	case hour >= 15 && hour < 19:
		// "Tonight" window
		idx := daysSinceStart % n
		meal := m.rotation[idx]
		return []cards.Card{{
			ID:       fmt.Sprintf("meals-%s", todayMidnight.Format("2006-01-02")),
			Source:   "meals",
			Type:     "info",
			Priority: 5,
			Icon:     "🍽️",
			Title:    fmt.Sprintf("Tonight: %s", meal),
			Color:    "#4a90d9",
			TimeWindow: &cards.TimeWindow{
				ActiveFrom:  "15:00",
				ActiveUntil: "22:00",
			},
		}}, nil

	case hour >= 19 && hour < 22:
		// "Tomorrow" window — show next day's meal
		tomorrowMidnight := todayMidnight.AddDate(0, 0, 1)
		daysTomorrow := int(tomorrowMidnight.Sub(m.startMidnight).Hours() / 24)
		idx := daysTomorrow % n
		meal := m.rotation[idx]
		return []cards.Card{{
			ID:       fmt.Sprintf("meals-next-%s", tomorrowMidnight.Format("2006-01-02")),
			Source:   "meals",
			Type:     "info",
			Priority: 6,
			Icon:     "🍽️",
			Title:    fmt.Sprintf("Tomorrow: %s", meal),
			Color:    "#4a90d9",
			TimeWindow: &cards.TimeWindow{
				ActiveFrom:  "15:00",
				ActiveUntil: "22:00",
			},
		}}, nil

	default:
		return nil, nil
	}
}
