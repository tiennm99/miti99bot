package misc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// The standard renderer is a deployment of
	// https://github.com/tiennm99/wheelofnames. Operators may point this to any
	// service that implements the same /api/gif contract.
	wheelOfNamesAPIURLEnv   = "WHEELOFNAMES_API_URL"
	wheelOfNamesAPITokenEnv = "WHEELOFNAMES_API_TOKEN"

	wheelRemoteDurationMs = 6000
	wheelRemoteHoldMs     = 1000
	wheelRemoteFPS        = 20
	wheelRemoteSize       = 512
	wheelRemoteTheme      = "classic"
	wheelRemoteDuration   = (wheelRemoteDurationMs + wheelRemoteHoldMs) / 1000
	wheelRemoteMaxBytes   = 12 << 20
	wheelRemoteTimeout    = 30 * time.Second
)

var errWheelAPINotConfigured = errors.New("wheelofnames api not configured")

type wheelAPIClient struct {
	HTTP  *http.Client
	URL   string
	Token string
}

type wheelAPIRequest struct {
	Options     []string `json:"options"`
	WinnerIndex int      `json:"winnerIndex"`
	DurationMs  int      `json:"durationMs"`
	HoldMs      int      `json:"holdMs"`
	FPS         int      `json:"fps"`
	Size        int      `json:"size"`
	Theme       string   `json:"theme"`
}

type wheelAnimation struct {
	Data     []byte
	Duration int
	Width    int
	Height   int
}

func newWheelAPIClientFromEnv() wheelAPIClient {
	return wheelAPIClient{
		URL:   strings.TrimSpace(os.Getenv(wheelOfNamesAPIURLEnv)),
		Token: strings.TrimSpace(os.Getenv(wheelOfNamesAPITokenEnv)),
	}
}

func (c wheelAPIClient) Render(ctx context.Context, options []string, winner int) ([]byte, error) {
	endpoint, err := wheelAPIEndpoint(c.URL)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("wheelofnames api options empty")
	}
	if winner < 0 || winner >= len(options) {
		return nil, fmt.Errorf("wheelofnames api winner index %d out of range %d", winner, len(options))
	}

	body, err := json.Marshal(wheelAPIRequest{
		Options:     options,
		WinnerIndex: winner,
		DurationMs:  wheelRemoteDurationMs,
		HoldMs:      wheelRemoteHoldMs,
		FPS:         wheelRemoteFPS,
		Size:        wheelRemoteSize,
		Theme:       wheelRemoteTheme,
	})
	if err != nil {
		return nil, fmt.Errorf("wheelofnames api request encode failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wheelofnames api request build failed: %w", err)
	}
	req.Header.Set("Accept", "image/gif")
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, errors.New("wheelofnames api request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wheelofnames api status %d", resp.StatusCode)
	}
	if err := requireWheelGIFContentType(resp.Header.Get("Content-Type")); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, wheelRemoteMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("wheelofnames api response read failed: %w", err)
	}
	if len(data) > wheelRemoteMaxBytes {
		return nil, fmt.Errorf("wheelofnames api response too large")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("wheelofnames api response empty")
	}
	if !isWheelGIF(data) {
		return nil, fmt.Errorf("wheelofnames api response is not a gif")
	}
	return data, nil
}

func (c wheelAPIClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: wheelRemoteTimeout}
}

func wheelAPIEndpoint(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errWheelAPINotConfigured
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("wheelofnames api url invalid")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("wheelofnames api url scheme %q unsupported", endpoint.Scheme)
	}
	return endpoint, nil
}

func requireWheelGIFContentType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "image/gif" {
		return fmt.Errorf("wheelofnames api content type %q unsupported", contentType)
	}
	return nil
}

func isWheelGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

func renderWheelOfNamesAnimation(ctx context.Context, options []string, winner int) (wheelAnimation, error) {
	client := newWheelAPIClientFromEnv()
	data, err := client.Render(ctx, options, winner)
	if err != nil {
		return wheelAnimation{}, err
	}
	return wheelAnimation{
		Data:     data,
		Duration: wheelRemoteDuration,
		Width:    wheelRemoteSize,
		Height:   wheelRemoteSize,
	}, nil
}
