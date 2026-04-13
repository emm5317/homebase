package ingest

import (
	"log/slog"
	"strings"

	"github.com/emm5317/homebase/internal/config"
	"github.com/emm5317/homebase/internal/store"
	"github.com/gofiber/fiber/v3"
)

type Payload struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Subject    string `json:"subject"`
	Tag        string `json:"tag"`
	Body       string `json:"body"`
	ReceivedAt string `json:"received_at"`
}

type Handler struct {
	store *store.Store
	cfg   *config.Config
}

func NewHandler(s *store.Store, cfg *config.Config) *Handler {
	return &Handler{store: s, cfg: cfg}
}

func (h *Handler) Handle(c fiber.Ctx) error {
	// Verify bearer token
	auth := c.Get("Authorization")
	expected := "Bearer " + h.cfg.IngestBearerToken
	if h.cfg.IngestBearerToken == "" || auth != expected {
		return c.SendStatus(401)
	}

	var p Payload
	if err := c.Bind().JSON(&p); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Verify sender is on allowlist
	email := extractEmail(p.From)
	senderName, allowed := h.cfg.Senders[strings.ToLower(email)]
	if !allowed {
		slog.Warn("rejected email from unknown sender", "from", p.From)
		return c.SendStatus(403)
	}

	// Route and create cards
	cards, err := Route(p, senderName)
	if err != nil {
		slog.Error("failed to route email", "error", err, "from", p.From)
		return c.Status(500).JSON(fiber.Map{"error": "routing failed"})
	}

	// Store cards
	created := 0
	for _, card := range cards {
		hash := contentHash(card)
		if h.store.HasContentHash(hash) {
			slog.Debug("skipping duplicate card", "id", card.ID)
			continue
		}
		if err := h.store.AddEmailCard(card, hash); err != nil {
			slog.Error("failed to persist card", "error", err, "id", card.ID)
			continue
		}
		created++
	}

	slog.Info("processed email", "from", senderName, "tag", p.Tag, "cards_created", created)
	return c.JSON(fiber.Map{"status": "ok", "cards_created": created})
}

func extractEmail(from string) string {
	// Handle "Name <email>" format
	if idx := strings.Index(from, "<"); idx >= 0 {
		end := strings.Index(from, ">")
		if end > idx {
			return strings.TrimSpace(from[idx+1 : end])
		}
	}
	return strings.TrimSpace(from)
}
