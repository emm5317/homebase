package providers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/config"
)

// Reminders emits rolling reminder cards for birthdays, anniversaries, and
// one-off events. Cards appear up to 7 days before the event and escalate as
// the date approaches.
type Reminders struct {
	entries []config.ReminderEntry
	loc     *time.Location
	now     func() time.Time // overridable in tests
}

func NewReminders(entries []config.ReminderEntry, loc *time.Location) *Reminders {
	return &Reminders{
		entries: entries,
		loc:     loc,
		now:     time.Now,
	}
}

func (r *Reminders) Name() string            { return "reminders" }
func (r *Reminders) Interval() time.Duration { return 1 * time.Hour }

func (r *Reminders) Fetch(ctx context.Context) ([]cards.Card, error) {
	if len(r.entries) == 0 {
		return nil, nil
	}

	now := r.now().In(r.loc)
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, r.loc)

	var result []cards.Card

	for _, entry := range r.entries {
		next, annual, err := parseReminderDate(entry.Date, todayMidnight, r.loc)
		if err != nil {
			slog.Warn("reminders: skipping entry with invalid date",
				"date", entry.Date, "text", entry.Text, "error", err)
			continue
		}

		// For one-off dates that are already past, skip silently.
		if !annual && next.Before(todayMidnight) {
			continue
		}

		days := int(next.Sub(todayMidnight).Hours() / 24)

		if days > 7 {
			continue
		}

		icon := entry.Icon
		if icon == "" {
			icon = "🎂"
		}

		text := entry.Text
		subtitle := next.Format("Mon, Jan 2")
		id := fmt.Sprintf("reminder-%s-%s", slugify(text), next.Format("2006-01-02"))

		var card cards.Card
		switch {
		case days == 0:
			card = cards.Card{
				ID:         id,
				Source:     "reminders",
				Type:       "alert",
				Priority:   2,
				Icon:       icon,
				Title:      fmt.Sprintf("%s %s today", icon, text),
				Subtitle:   subtitle,
				Status:     "warning",
				AlertLevel: "warning",
				Color:      "#e86f5a",
			}
		case days == 1:
			card = cards.Card{
				ID:       id,
				Source:   "reminders",
				Type:     "info",
				Priority: 3,
				Icon:     icon,
				Title:    fmt.Sprintf("%s tomorrow", text),
				Subtitle: subtitle,
				Status:   "warning",
				Color:    "#d4943a",
			}
		case days <= 3:
			card = cards.Card{
				ID:       id,
				Source:   "reminders",
				Type:     "info",
				Priority: 5,
				Icon:     icon,
				Title:    fmt.Sprintf("%s in %d days", text, days),
				Subtitle: subtitle,
				Color:    "#d4943a",
			}
		default: // days <= 7
			card = cards.Card{
				ID:       id,
				Source:   "reminders",
				Type:     "info",
				Priority: 7,
				Icon:     icon,
				Title:    fmt.Sprintf("%s %s", text, next.Weekday().String()),
				Subtitle: subtitle,
				Color:    "#4a90d9",
			}
		}

		result = append(result, card)
	}

	return result, nil
}

// parseReminderDate parses "MM-DD" (annual) or "YYYY-MM-DD" (one-off) and
// returns the next occurrence of the date relative to todayMidnight.
// The second return value is true for annual events.
func parseReminderDate(dateStr string, todayMidnight time.Time, loc *time.Location) (time.Time, bool, error) {
	if len(dateStr) == 5 {
		// "MM-DD" — annual
		t, err := time.ParseInLocation("01-02", dateStr, loc)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parsing MM-DD %q: %w", dateStr, err)
		}
		month := t.Month()
		day := t.Day()

		candidate := time.Date(todayMidnight.Year(), month, day, 0, 0, 0, 0, loc)
		if candidate.Before(todayMidnight) {
			candidate = time.Date(todayMidnight.Year()+1, month, day, 0, 0, 0, 0, loc)
		}
		return candidate, true, nil
	}

	// "YYYY-MM-DD" — one-off
	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parsing YYYY-MM-DD %q: %w", dateStr, err)
	}
	return t, false, nil
}

// slugify converts a string to a URL-safe slug suitable for card IDs.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	return b.String()
}
