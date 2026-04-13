package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emm5317/homebase/internal/cards"
	_ "modernc.org/sqlite"
)

type Store struct {
	mu    sync.RWMutex
	cards map[string][]cards.Card // keyed by provider name

	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	// SD-card-safe pragmas
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-20000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("setting %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrating: %w", err)
	}

	s := &Store{
		cards: make(map[string][]cards.Card),
		db:    db,
	}

	// Load persisted email cards into memory
	if err := s.loadPersisted(); err != nil {
		slog.Warn("failed to load persisted cards", "error", err)
	}

	return s, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS email_cards (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			data TEXT NOT NULL,
			content_hash TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		)
	`)
	return err
}

func (s *Store) loadPersisted() error {
	rows, err := s.db.Query(`
		SELECT data FROM email_cards
		WHERE expires_at IS NULL OR expires_at > ?
	`, time.Now().UTC())
	if err != nil {
		return err
	}
	defer rows.Close()

	var emailCards []cards.Card
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var c cards.Card
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue
		}
		c.Persistent = true
		emailCards = append(emailCards, c)
	}

	s.mu.Lock()
	s.cards["email"] = emailCards
	s.mu.Unlock()

	slog.Info("loaded persisted email cards", "count", len(emailCards))
	return nil
}

// SetProviderCards replaces all cards for a given provider (API-sourced, in-memory only).
func (s *Store) SetProviderCards(provider string, cc []cards.Card) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[provider] = cc
}

// AddEmailCard persists a card to SQLite and adds it to the in-memory cache.
func (s *Store) AddEmailCard(c cards.Card, contentHash string) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	var expiresAt *time.Time
	if c.ExpiresAt != nil {
		expiresAt = c.ExpiresAt
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO email_cards (id, source, data, content_hash, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, c.ID, c.Source, string(data), contentHash, expiresAt)
	if err != nil {
		return fmt.Errorf("persisting card: %w", err)
	}

	c.Persistent = true
	s.mu.Lock()
	s.cards["email"] = append(s.cards["email"], c)
	s.mu.Unlock()

	return nil
}

// HasContentHash checks if a card with this content hash already exists (deduplication).
func (s *Store) HasContentHash(hash string) bool {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM email_cards WHERE content_hash = ?`, hash).Scan(&count)
	return err == nil && count > 0
}

// AllCards returns all cards from all sources.
func (s *Store) AllCards() []cards.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []cards.Card
	for _, cc := range s.cards {
		all = append(all, cc...)
	}
	return all
}

// GetCardsBySource returns cards from a specific provider.
func (s *Store) GetCardsBySource(source string) []cards.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cards[source]
}

// UpdateEmailCard updates a persisted card in both SQLite and memory.
func (s *Store) UpdateEmailCard(c cards.Card) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE email_cards SET data = ? WHERE id = ?`, string(data), c.ID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	emailCards := s.cards["email"]
	for i, existing := range emailCards {
		if existing.ID == c.ID {
			c.Persistent = true
			emailCards[i] = c
			break
		}
	}
	return nil
}

// CleanExpired removes expired email cards from SQLite and memory.
func (s *Store) CleanExpired() {
	now := time.Now().UTC()
	s.db.Exec(`DELETE FROM email_cards WHERE expires_at IS NOT NULL AND expires_at < ?`, now)

	s.mu.Lock()
	defer s.mu.Unlock()
	emailCards := s.cards["email"]
	kept := emailCards[:0]
	for _, c := range emailCards {
		if c.ExpiresAt == nil || c.ExpiresAt.After(now) {
			kept = append(kept, c)
		}
	}
	s.cards["email"] = kept
}

func (s *Store) Close() error {
	if s.db != nil {
		s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}
