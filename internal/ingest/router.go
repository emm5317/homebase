package ingest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/emm5317/homebase/internal/cards"
)

func Route(p Payload, senderName string) ([]cards.Card, error) {
	switch strings.ToLower(p.Tag) {
	case "grocery":
		return parseGrocery(p, senderName), nil
	case "chore":
		return parseChore(p, senderName), nil
	case "note":
		return parseNote(p, senderName), nil
	case "reminder":
		return parseReminder(p, senderName), nil
	case "calendar", "cal":
		return parseCalendar(p, senderName), nil
	default:
		return classifyAndRoute(p, senderName)
	}
}

func classifyAndRoute(p Payload, senderName string) ([]cards.Card, error) {
	combined := strings.ToLower(p.Subject + " " + p.Body)

	// Grocery signals
	groceryPatterns := []string{
		"grocery", "groceries", "pick up", "buy",
		"need from store", "shopping list", "from the store",
	}
	for _, pat := range groceryPatterns {
		if strings.Contains(combined, pat) {
			return parseGrocery(p, senderName), nil
		}
	}

	// Chore signals
	chorePatterns := []string{
		"chore", "todo", "to do", "to-do",
		"don't forget", "remember to", "make sure",
	}
	for _, pat := range chorePatterns {
		if strings.Contains(combined, pat) {
			return parseChore(p, senderName), nil
		}
	}

	// Calendar signals
	calendarPatterns := []string{
		"practice", "game", "appointment", "meeting",
		"lesson", "class", "pickup", "drop off",
		"moved to", "rescheduled", "cancelled", "canceled",
	}
	for _, pat := range calendarPatterns {
		if strings.Contains(combined, pat) {
			return parseCalendar(p, senderName), nil
		}
	}

	// Reminder signals
	reminderPatterns := []string{
		"remind", "reminder", "don't forget",
	}
	for _, pat := range reminderPatterns {
		if strings.Contains(combined, pat) {
			return parseReminder(p, senderName), nil
		}
	}

	// Default: create a note card
	return parseNote(p, senderName), nil
}

func contentHash(c cards.Card) string {
	data, _ := json.Marshal(struct {
		Source string
		Title  string
		Body   string
		Items  []cards.ListItem
	}{c.Source, c.Title, c.Body, c.Items})
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
