package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/emm5317/homebase/internal/providers"
	"github.com/emm5317/homebase/internal/store"
)

type ProviderStatus struct {
	Name      string    `json:"name"`
	LastFetch time.Time `json:"last_fetch"`
	LastError string    `json:"last_error,omitempty"`
	CardCount int       `json:"card_count"`
}

type Scheduler struct {
	store     *store.Store
	providers []providers.Provider

	mu       sync.RWMutex
	statuses map[string]*ProviderStatus

	cancel context.CancelFunc
}

func New(s *store.Store, pp ...providers.Provider) *Scheduler {
	statuses := make(map[string]*ProviderStatus, len(pp))
	for _, p := range pp {
		statuses[p.Name()] = &ProviderStatus{Name: p.Name()}
	}
	return &Scheduler{
		store:     s,
		providers: pp,
		statuses:  statuses,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, p := range s.providers {
		go s.run(ctx, p)
	}

	// Clean expired email cards every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.store.CleanExpired()
			}
		}
	}()
}

func (s *Scheduler) run(ctx context.Context, p providers.Provider) {
	// Fetch immediately on start
	s.fetch(ctx, p)

	ticker := time.NewTicker(p.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetch(ctx, p)
		}
	}
}

func (s *Scheduler) fetch(ctx context.Context, p providers.Provider) {
	cards, err := p.Fetch(ctx)

	s.mu.Lock()
	status := s.statuses[p.Name()]
	status.LastFetch = time.Now()
	if err != nil {
		status.LastError = err.Error()
		slog.Warn("provider fetch failed", "provider", p.Name(), "error", err)
	} else {
		status.LastError = ""
		status.CardCount = len(cards)
		s.store.SetProviderCards(p.Name(), cards)
		slog.Debug("provider fetch ok", "provider", p.Name(), "cards", len(cards))
	}
	s.mu.Unlock()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) Statuses() []ProviderStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ProviderStatus, 0, len(s.statuses))
	for _, st := range s.statuses {
		out = append(out, *st)
	}
	return out
}
