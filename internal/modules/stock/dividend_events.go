package stock

import (
	"context"
	"time"
)

// DividendEventProvider returns normalized dividend events up to through. An
// implementation may include a bounded overlap before after when its source
// exposes only day-granularity publication times; callers must deduplicate by
// provider event ID.
type DividendEventProvider interface {
	FetchDividendEvents(ctx context.Context, symbol string, after, through time.Time) ([]DividendEvent, error)
}

// DividendKind identifies how an event changes a portfolio.
type DividendKind string

const (
	DividendKindCash   DividendKind = "cash"
	DividendKindShares DividendKind = "shares"
)

// DividendEvent is a provider-independent, validated stock dividend event.
// Cash events set VNDPerShare. Share events set OwnedShares and NewShares.
type DividendEvent struct {
	ProviderID string
	Symbol     string
	Kind       DividendKind

	PublishedAt time.Time
	ExDate      time.Time
	RecordDate  time.Time
	PaymentDate time.Time

	VNDPerShare int64
	OwnedShares int64
	NewShares   int64

	Title     string
	SourceURL string
}
