package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// telegramAPIBase is the Bot API host (matches go-telegram's default).
const telegramAPIBase = "https://api.telegram.org"

// webhookDeleteTimeout bounds a single deleteWebhook HTTP call so a stalled
// connection cannot wedge startup.
const webhookDeleteTimeout = 10 * time.Second

// DeleteWebhook clears any configured webhook via a plain Bot API GET.
//
// go-telegram's b.DeleteWebhook posts an empty multipart form (DeleteWebhookParams
// is all-omitempty), and some networks answer that bodyless POST with an empty
// response body — decoded as "unexpected end of JSON input" — so the webhook is
// never actually removed and getUpdates keeps returning 409. A GET with no body
// sidesteps that request shape. Pending updates are intentionally kept (the API
// default) so the long poller drains any buffered queue.
func DeleteWebhook(ctx context.Context, token string) error {
	return deleteWebhookAt(ctx, telegramAPIBase, token)
}

// deleteWebhookAt is the testable core; base lets tests point at an httptest
// server instead of the live API.
func deleteWebhookAt(ctx context.Context, base, token string) error {
	ctx, cancel := context.WithTimeout(ctx, webhookDeleteTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/bot"+token+"/deleteWebhook", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var r struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		// Body is safe to log (no secret); never include the URL (carries the token).
		return fmt.Errorf("decode deleteWebhook response (status %d, body %q): %w", resp.StatusCode, string(body), err)
	}
	if !r.OK {
		return fmt.Errorf("deleteWebhook rejected (status %d): %s", resp.StatusCode, r.Description)
	}
	return nil
}
