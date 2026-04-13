package providers

import (
	"context"
	"time"

	"github.com/emm5317/homebase/internal/cards"
)

type Provider interface {
	Name() string
	Fetch(ctx context.Context) ([]cards.Card, error)
	Interval() time.Duration
}
