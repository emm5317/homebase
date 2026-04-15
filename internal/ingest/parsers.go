package ingest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/emm5317/homebase/internal/cards"
)

// ttlPattern matches an optional trailing "Nd" suffix (e.g. "3d", "14D").
var ttlPattern = regexp.MustCompile(`(?i)\s+(\d+)d$`)

const (
	fridgeDefaultTTL = 3
	fridgeMaxTTL     = 30
)

// slug converts a string to a lowercase, alphanumeric-only identifier.
func slug(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = false
		} else if !prev && b.Len() > 0 {
			b.WriteByte('-')
			prev = true
		}
	}
	result := b.String()
	return strings.TrimRight(result, "-")
}

// parseFridgeItem parses a single item string of the form "<name>[ Nd]".
// Returns the item name and TTL in days.
func parseFridgeItem(raw string) (name string, ttl int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}
	ttl = fridgeDefaultTTL
	if m := ttlPattern.FindStringSubmatchIndex(raw); m != nil {
		digits := raw[m[2]:m[3]]
		if n, err := strconv.Atoi(digits); err == nil && n > 0 {
			ttl = n
			if ttl > fridgeMaxTTL {
				ttl = fridgeMaxTTL
			}
		}
		name = strings.TrimSpace(raw[:m[0]])
	} else {
		name = raw
	}
	return name, ttl
}

func parseFridge(p Payload, senderName string) []cards.Card {
	raw := p.Body
	if raw == "" {
		raw = p.Subject
	}
	// Normalize: commas become newlines (matches grocery parser behavior)
	raw = strings.ReplaceAll(raw, ",", "\n")

	now := time.Now()
	var result []cards.Card

	for _, line := range strings.Split(raw, "\n") {
		name, ttl := parseFridgeItem(line)
		if name == "" {
			continue
		}

		expiry := now.Add(time.Duration(ttl) * 24 * time.Hour)

		color := "#5a9e78" // green: ttl > 2
		if ttl <= 1 {
			color = "#e86f5a" // red
		} else if ttl <= 2 {
			color = "#d4943a" // yellow
		}

		result = append(result, cards.Card{
			ID:         fmt.Sprintf("fridge-%s-%d", slug(name), now.Unix()),
			Source:     "fridge",
			Type:       "info",
			Priority:   7,
			Icon:       "\U0001F9C0", // 🧀
			Title:      "Fridge: " + name,
			Subtitle:   "Use by " + expiry.Format("Mon 1/2"),
			Color:      color,
			ExpiresAt:  &expiry,
			CreatedBy:  senderName,
			CreatedVia: "email",
			Persistent: true,
		})
	}

	return result
}

func parseGrocery(p Payload, senderName string) []cards.Card {
	raw := p.Body
	if raw == "" {
		raw = p.Subject
	}
	// Normalize separators
	raw = strings.ReplaceAll(raw, ",", "\n")

	lines := strings.Split(raw, "\n")
	var items []cards.ListItem
	for _, line := range lines {
		text := strings.TrimSpace(line)
		// Strip common list prefixes
		text = strings.TrimLeft(text, "-•*")
		text = strings.TrimSpace(text)
		// Strip leading numbers like "1. " or "1) "
		if len(text) > 2 && text[0] >= '0' && text[0] <= '9' {
			if idx := strings.IndexAny(text, ".)"); idx > 0 && idx < 4 {
				text = strings.TrimSpace(text[idx+1:])
			}
		}
		if text != "" {
			items = append(items, cards.ListItem{Text: text, Done: false})
		}
	}

	expires := time.Now().Add(48 * time.Hour)
	return []cards.Card{{
		ID:         fmt.Sprintf("email-grocery-%s", time.Now().Format("20060102-150405")),
		Source:     "email",
		Type:       "list",
		Priority:   5,
		Icon:       "\U0001f6d2",
		Title:      "Grocery List",
		Subtitle:   fmt.Sprintf("Added by %s", senderName),
		Color:      "#d4943a",
		Items:      items,
		CreatedBy:  senderName,
		CreatedVia: "email",
		ExpiresAt:  &expires,
		TimeWindow: &cards.TimeWindow{AllDay: true},
	}}
}

func parseChore(p Payload, senderName string) []cards.Card {
	choreName := p.Subject
	if choreName == "" {
		lines := strings.Split(strings.TrimSpace(p.Body), "\n")
		if len(lines) > 0 {
			choreName = lines[0]
		}
	}
	if choreName == "" {
		choreName = "New chore"
	}

	items := []cards.ListItem{{Text: choreName, Done: false}}

	expires := time.Now().Add(24 * time.Hour)
	return []cards.Card{{
		ID:         fmt.Sprintf("email-chore-%s", time.Now().Format("20060102-150405")),
		Source:     "email",
		Type:       "list",
		Priority:   5,
		Icon:       "\u2705",
		Title:      "To-Do",
		Subtitle:   fmt.Sprintf("Added by %s", senderName),
		Color:      "#5a9e78",
		Items:      items,
		CreatedBy:  senderName,
		CreatedVia: "email",
		ExpiresAt:  &expires,
		TimeWindow: &cards.TimeWindow{AllDay: true},
	}}
}

func parseNote(p Payload, senderName string) []cards.Card {
	title := p.Subject
	if title == "" {
		title = "Note"
	}
	body := strings.TrimSpace(p.Body)
	if body == "" {
		body = title
		title = "Note"
	}

	expires := time.Now().Add(24 * time.Hour)
	return []cards.Card{{
		ID:         fmt.Sprintf("email-note-%s", time.Now().Format("20060102-150405")),
		Source:     "email",
		Type:       "info",
		Priority:   6,
		Icon:       "\U0001f4dd",
		Title:      title,
		Body:       body,
		Subtitle:   fmt.Sprintf("From %s", senderName),
		Color:      "#8b6aae",
		CreatedBy:  senderName,
		CreatedVia: "email",
		ExpiresAt:  &expires,
		TimeWindow: &cards.TimeWindow{AllDay: true},
	}}
}

func parseReminder(p Payload, senderName string) []cards.Card {
	text := p.Subject
	if text == "" {
		text = strings.TrimSpace(p.Body)
	}
	if text == "" {
		text = "Reminder"
	}

	expires := time.Now().Add(24 * time.Hour)
	return []cards.Card{{
		ID:         fmt.Sprintf("email-reminder-%s", time.Now().Format("20060102-150405")),
		Source:     "email",
		Type:       "info",
		Priority:   3,
		Icon:       "\u23f0",
		Title:      text,
		Subtitle:   fmt.Sprintf("From %s", senderName),
		Color:      "#e86f5a",
		CreatedBy:  senderName,
		CreatedVia: "email",
		ExpiresAt:  &expires,
		TimeWindow: &cards.TimeWindow{AllDay: true},
	}}
}

func parseCalendar(p Payload, senderName string) []cards.Card {
	title := p.Subject
	if title == "" {
		title = "Calendar Update"
	}
	body := strings.TrimSpace(p.Body)

	expires := time.Now().Add(24 * time.Hour)
	return []cards.Card{{
		ID:         fmt.Sprintf("email-calendar-%s", time.Now().Format("20060102-150405")),
		Source:     "email",
		Type:       "event",
		Priority:   4,
		Icon:       "\U0001f4c5",
		Title:      title,
		Body:       body,
		Subtitle:   fmt.Sprintf("From %s", senderName),
		Color:      "#8b6aae",
		CreatedBy:  senderName,
		CreatedVia: "email",
		ExpiresAt:  &expires,
		TimeWindow: &cards.TimeWindow{AllDay: true},
	}}
}
