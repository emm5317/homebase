package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/config"
)

type Garbage struct {
	cfg config.GarbageConfig
	loc *time.Location
}

func NewGarbage(cfg config.GarbageConfig, loc *time.Location) *Garbage {
	return &Garbage{cfg: cfg, loc: loc}
}

func (g *Garbage) Name() string            { return "garbage" }
func (g *Garbage) Interval() time.Duration { return 1 * time.Hour }

func (g *Garbage) Fetch(ctx context.Context) ([]cards.Card, error) {
	now := time.Now().In(g.loc)
	pickupDay := parseDayOfWeek(g.cfg.PickupDay)
	if pickupDay < 0 {
		return nil, fmt.Errorf("invalid pickup_day: %s", g.cfg.PickupDay)
	}

	nextPickup := nextWeekday(now, pickupDay)

	// Holiday shift: if a holiday falls Mon-Thu of pickup week, shift to Friday.
	// Exception: if the holiday IS Friday, keep regular Thursday schedule.
	nextPickup = g.applyHolidayShift(nextPickup, pickupDay)

	isRecycling := g.isRecyclingWeek(nextPickup)
	isYardWaste := g.isYardWasteActive(nextPickup)

	var result []cards.Card

	// Evening before pickup (after 5 PM)
	eveningBefore := time.Date(nextPickup.Year(), nextPickup.Month(), nextPickup.Day()-1, 17, 0, 0, 0, g.loc)
	pickupMorning := time.Date(nextPickup.Year(), nextPickup.Month(), nextPickup.Day(), 0, 0, 0, 0, g.loc)
	pickupEnd := time.Date(nextPickup.Year(), nextPickup.Month(), nextPickup.Day(), 12, 0, 0, 0, g.loc)

	if now.After(eveningBefore) && now.Before(pickupMorning) {
		// Evening before
		items := g.buildPickupItems(isRecycling, isYardWaste)
		result = append(result, cards.Card{
			ID:       fmt.Sprintf("garbage-%s", nextPickup.Format("2006-01-02")),
			Source:   "garbage",
			Type:     "info",
			Priority: 4,
			Icon:     "\U0001f5d1\ufe0f",
			Title:    fmt.Sprintf("%s tomorrow", items),
			Subtitle: "Bins out tonight by 7 AM",
			Color:    "#5a9e78",
			TimeWindow: &cards.TimeWindow{
				ActiveFrom:  "17:00",
				ActiveUntil: "23:59",
			},
		})
	} else if now.After(pickupMorning) && now.Before(pickupEnd) {
		// Morning of pickup
		items := g.buildPickupItems(isRecycling, isYardWaste)
		result = append(result, cards.Card{
			ID:       fmt.Sprintf("garbage-%s", nextPickup.Format("2006-01-02")),
			Source:   "garbage",
			Type:     "info",
			Priority: 4,
			Icon:     "\U0001f5d1\ufe0f",
			Title:    fmt.Sprintf("%s today", items),
			Subtitle: "Bins to curb by 7 AM",
			Color:    "#5a9e78",
			TimeWindow: &cards.TimeWindow{
				ActiveFrom:  "00:00",
				ActiveUntil: "12:00",
			},
		})
	}

	return result, nil
}

func (g *Garbage) applyHolidayShift(pickup time.Time, normalDay time.Weekday) time.Time {
	for _, h := range g.cfg.Holidays {
		holiday, err := time.ParseInLocation("2006-01-02", h, g.loc)
		if err != nil {
			continue
		}
		// Check if holiday is in the same week as pickup
		pickupYear, pickupWeek := pickup.ISOWeek()
		holYear, holWeek := holiday.ISOWeek()
		if pickupYear != holYear || pickupWeek != holWeek {
			continue
		}
		// If holiday is on Friday, keep regular schedule
		if holiday.Weekday() == time.Friday {
			continue
		}
		// If holiday falls on or before the pickup day, shift to Friday
		if holiday.Weekday() <= normalDay {
			friday := nextWeekday(
				time.Date(pickup.Year(), pickup.Month(), pickup.Day()-int(pickup.Weekday()), 0, 0, 0, 0, g.loc),
				time.Friday,
			)
			return friday
		}
	}
	return pickup
}

func (g *Garbage) isRecyclingWeek(date time.Time) bool {
	switch strings.ToLower(g.cfg.RecyclingWeeks) {
	case "even":
		_, week := date.ISOWeek()
		return week%2 == 0
	case "odd":
		_, week := date.ISOWeek()
		return week%2 == 1
	case "every":
		return true
	default:
		return true
	}
}

func (g *Garbage) isYardWasteActive(date time.Time) bool {
	parts := strings.Split(strings.ToLower(g.cfg.YardWaste), "-")
	if len(parts) != 2 {
		return false
	}
	startMonth := parseMonth(parts[0])
	endMonth := parseMonth(parts[1])
	if startMonth == 0 || endMonth == 0 {
		return false
	}
	month := date.Month()
	return month >= startMonth && month <= endMonth
}

func (g *Garbage) buildPickupItems(recycling, yardWaste bool) string {
	items := []string{"Garbage"}
	if recycling {
		items = append(items, "recycling")
	}
	if yardWaste {
		items = append(items, "yard waste")
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 {
		return items[0] + " & " + items[1]
	}
	return items[0] + ", " + strings.Join(items[1:], " & ")
}

func nextWeekday(from time.Time, day time.Weekday) time.Time {
	daysUntil := int(day - from.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}
	// If today IS the pickup day and it's still morning, use today
	if from.Weekday() == day && from.Hour() < 12 {
		return time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	}
	next := from.AddDate(0, 0, daysUntil)
	return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, from.Location())
}

func parseDayOfWeek(s string) time.Weekday {
	switch strings.ToLower(s) {
	case "sunday":
		return time.Sunday
	case "monday":
		return time.Monday
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	default:
		return -1
	}
}

func parseMonth(s string) time.Month {
	months := map[string]time.Month{
		"january": time.January, "february": time.February, "march": time.March,
		"april": time.April, "may": time.May, "june": time.June,
		"july": time.July, "august": time.August, "september": time.September,
		"october": time.October, "november": time.November, "december": time.December,
	}
	return months[strings.TrimSpace(s)]
}
