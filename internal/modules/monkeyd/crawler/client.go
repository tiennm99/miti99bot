package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// The site rejects requests without a browser-like User-Agent.
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// maxPageSize caps how much of a response we buffer; chapter pages are ~130 KB.
const maxPageSize = 8 << 20

// statusError reports an unexpected HTTP status so Get can decide whether
// retrying is worthwhile.
type statusError struct {
	code   int
	status string
}

func (e *statusError) Error() string { return "unexpected status " + e.status }

// retryable is true for transient failures. A 404 or 403 will not fix itself,
// so those fail immediately instead of burning the retry budget.
func (e *statusError) retryable() bool {
	return e.code == http.StatusTooManyRequests || e.code >= 500
}

// Client fetches pages from monkeydd.com. It spaces requests out by a fixed
// delay no matter how many goroutines call Get, so raising the worker count
// never raises the request rate, and it retries transient failures with
// exponential backoff.
type Client struct {
	http    *http.Client
	ua      string
	delay   time.Duration
	retries int

	mu       sync.Mutex
	nextSlot time.Time
}

func NewClient(delay time.Duration, retries int) *Client {
	return &Client{
		http:    &http.Client{Timeout: 45 * time.Second},
		ua:      defaultUserAgent,
		delay:   delay,
		retries: retries,
	}
}

// reserve claims the next request slot and blocks until it comes due, holding
// the global rate at one request per delay across all callers.
func (c *Client) reserve(ctx context.Context) error {
	c.mu.Lock()
	slot := c.nextSlot
	if now := time.Now(); slot.Before(now) {
		slot = now
	}
	c.nextSlot = slot.Add(c.delay)
	c.mu.Unlock()

	wait := time.Until(slot)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Get fetches url, retrying transient failures.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		if err := c.reserve(ctx); err != nil {
			return nil, err
		}

		body, err := c.fetch(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err

		var se *statusError
		if errors.As(err, &se) && !se.retryable() {
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("fetch %s: %w", url, lastErr)
}

func (c *Client) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &statusError{code: resp.StatusCode, status: resp.Status}
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxPageSize))
}
