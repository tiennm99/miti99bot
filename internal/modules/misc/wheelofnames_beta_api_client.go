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

	"github.com/tiennm99/miti99bot/internal/log"
)

const (
	wheelOfNamesBetaAPIURLEnv   = "WHEELOFNAMES_API_URL"
	wheelOfNamesBetaAPITokenEnv = "WHEELOFNAMES_API_TOKEN"

	wheelBetaRemoteDurationMs = 6000
	wheelBetaRemoteHoldMs     = 1000
	wheelBetaRemoteFPS        = 20
	wheelBetaRemoteSize       = 512
	wheelBetaRemoteTheme      = "classic"
	wheelBetaRemoteDuration   = (wheelBetaRemoteDurationMs + wheelBetaRemoteHoldMs) / 1000
	wheelBetaRemoteMaxBytes   = 12 << 20
	wheelBetaRemoteTimeout    = 30 * time.Second
)

var errWheelBetaAPINotConfigured = errors.New("wheelofnamesbeta api not configured")

type wheelBetaAPIClient struct {
	HTTP  *http.Client
	URL   string
	Token string
}

type wheelBetaAPIRequest struct {
	Options     []string `json:"options"`
	WinnerIndex int      `json:"winnerIndex"`
	DurationMs  int      `json:"durationMs"`
	HoldMs      int      `json:"holdMs"`
	FPS         int      `json:"fps"`
	Size        int      `json:"size"`
	Theme       string   `json:"theme"`
}

type wheelBetaAnimation struct {
	Data     []byte
	Duration int
	Width    int
	Height   int
}

func newWheelBetaAPIClientFromEnv() wheelBetaAPIClient {
	return wheelBetaAPIClient{
		URL:   strings.TrimSpace(os.Getenv(wheelOfNamesBetaAPIURLEnv)),
		Token: strings.TrimSpace(os.Getenv(wheelOfNamesBetaAPITokenEnv)),
	}
}

func (c wheelBetaAPIClient) Render(ctx context.Context, options []string, winner int) ([]byte, error) {
	endpoint, err := wheelBetaAPIEndpoint(c.URL)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("wheelofnamesbeta api options empty")
	}
	if winner < 0 || winner >= len(options) {
		return nil, fmt.Errorf("wheelofnamesbeta api winner index %d out of range %d", winner, len(options))
	}

	body, err := json.Marshal(wheelBetaAPIRequest{
		Options:     options,
		WinnerIndex: winner,
		DurationMs:  wheelBetaRemoteDurationMs,
		HoldMs:      wheelBetaRemoteHoldMs,
		FPS:         wheelBetaRemoteFPS,
		Size:        wheelBetaRemoteSize,
		Theme:       wheelBetaRemoteTheme,
	})
	if err != nil {
		return nil, fmt.Errorf("wheelofnamesbeta api request encode failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wheelofnamesbeta api request build failed: %w", err)
	}
	req.Header.Set("Accept", "image/gif")
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, errors.New("wheelofnamesbeta api request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wheelofnamesbeta api status %d", resp.StatusCode)
	}
	if err := requireWheelBetaGIFContentType(resp.Header.Get("Content-Type")); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, wheelBetaRemoteMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("wheelofnamesbeta api response read failed: %w", err)
	}
	if len(data) > wheelBetaRemoteMaxBytes {
		return nil, fmt.Errorf("wheelofnamesbeta api response too large")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("wheelofnamesbeta api response empty")
	}
	if !isWheelBetaGIF(data) {
		return nil, fmt.Errorf("wheelofnamesbeta api response is not a gif")
	}
	return data, nil
}

func (c wheelBetaAPIClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: wheelBetaRemoteTimeout}
}

func wheelBetaAPIEndpoint(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errWheelBetaAPINotConfigured
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("wheelofnamesbeta api url invalid")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("wheelofnamesbeta api url scheme %q unsupported", endpoint.Scheme)
	}
	return endpoint, nil
}

func requireWheelBetaGIFContentType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "image/gif" {
		return fmt.Errorf("wheelofnamesbeta api content type %q unsupported", contentType)
	}
	return nil
}

func isWheelBetaGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

func renderWheelOfNamesBetaAnimation(ctx context.Context, options []string, winner int) (wheelBetaAnimation, error) {
	client := newWheelBetaAPIClientFromEnv()
	if data, err := client.Render(ctx, options, winner); err == nil {
		return wheelBetaAnimation{
			Data:     data,
			Duration: wheelBetaRemoteDuration,
			Width:    wheelBetaRemoteSize,
			Height:   wheelBetaRemoteSize,
		}, nil
	} else if !errors.Is(err, errWheelBetaAPINotConfigured) {
		log.Warn("wheelofnamesbeta remote render failed", "err", err)
	}

	data, err := renderWheelOfNamesBetaGIF(options, winner)
	if err != nil {
		return wheelBetaAnimation{}, err
	}
	return wheelBetaAnimation{
		Data:     data,
		Duration: wheelBetaDuration,
		Width:    wheelBetaSize,
		Height:   wheelBetaSize,
	}, nil
}
