package ingest

import (
	"fmt"
	"strings"
	"time"

	"github.com/emm5317/homebase/internal/cards"
)

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
