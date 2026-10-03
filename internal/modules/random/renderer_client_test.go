package random

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestRendererClient_RenderValidRequest(t *testing.T) {
	var got wheelRenderRequest
	var gotAccept string
	var gotContentType string
	var gotMethod string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("Decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a-remote"))
	}))
	defer server.Close()

	client := rendererClient{
		HTTP:    server.Client(),
		BaseURL: server.URL,
	}
	data, err := client.RenderWheel(context.Background(), []string{"alice", "bob", "carol"}, 1)
	if err != nil {
		t.Fatalf("RenderWheel: %v", err)
	}
	if !bytes.Equal(data, []byte("GIF89a-remote")) {
		t.Fatalf("data = %q, want remote GIF bytes", data)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/gif" {
		t.Fatalf("path = %q, want /api/gif", gotPath)
	}
	if gotAccept != "image/gif" {
		t.Fatalf("Accept = %q, want image/gif", gotAccept)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if !slices.Equal(got.Options, []string{"alice", "bob", "carol"}) {
		t.Fatalf("options = %#v, want original options", got.Options)
	}
	if got.WinnerIndex != 1 {
		t.Fatalf("winnerIndex = %d, want 1", got.WinnerIndex)
	}
	assertWheelRemoteDefaults(t, got)
}

// The renderer is internal to the compose network; the bot sends no credentials.
func TestRendererClient_RenderSendsNoAuthorization(t *testing.T) {
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a"))
	}))
	defer server.Close()

	client := rendererClient{HTTP: server.Client(), BaseURL: server.URL}
	if _, err := client.RenderWheel(context.Background(), []string{"alice"}, 0); err != nil {
		t.Fatalf("RenderWheel: %v", err)
	}
	if gotAuthorization != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuthorization)
	}
}

func TestRendererClient_RenderNotConfigured(t *testing.T) {
	client := rendererClient{}
	_, err := client.RenderWheel(context.Background(), []string{"alice"}, 0)
	if !errors.Is(err, errRendererNotConfigured) {
		t.Fatalf("RenderWheel error = %v, want errRendererNotConfigured", err)
	}
}

func TestRendererClient_RenderRejectsInvalidInput(t *testing.T) {
	client := rendererClient{BaseURL: "https://example.com"}
	for _, tc := range []struct {
		name    string
		url     string
		options []string
		winner  int
	}{
		{name: "bad scheme", url: "ftp://example.com", options: []string{"alice"}, winner: 0},
		{name: "empty options", url: "https://example.com", options: nil, winner: 0},
		{name: "winner out of range", url: "https://example.com", options: []string{"alice"}, winner: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client.BaseURL = tc.url
			if _, err := client.RenderWheel(context.Background(), tc.options, tc.winner); err == nil {
				t.Fatalf("RenderWheel returned nil error")
			}
		})
	}
}

func TestRendererClient_RenderReturnsErrorsForBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        []byte
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, contentType: "text/plain", body: []byte("no")},
		{name: "server error", status: http.StatusInternalServerError, contentType: "text/plain", body: []byte("bad")},
		{name: "non gif", status: http.StatusOK, contentType: "text/plain", body: []byte("not gif")},
		{name: "empty gif", status: http.StatusOK, contentType: "image/gif", body: nil},
		{name: "mislabeled gif", status: http.StatusOK, contentType: "image/gif", body: []byte("not gif")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()

			client := rendererClient{HTTP: server.Client(), BaseURL: server.URL}
			if _, err := client.RenderWheel(context.Background(), []string{"alice"}, 0); err == nil {
				t.Fatalf("RenderWheel returned nil error")
			}
		})
	}
}

func TestRendererClient_RenderRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write(bytes.Repeat([]byte("a"), int(rendererMaxBytes)+1))
	}))
	defer server.Close()

	client := rendererClient{HTTP: server.Client(), BaseURL: server.URL}
	if _, err := client.RenderWheel(context.Background(), []string{"alice"}, 0); err == nil {
		t.Fatalf("RenderWheel returned nil error")
	}
}

func TestRendererClient_DefaultHTTPClientHasTimeout(t *testing.T) {
	client := rendererClient{}
	if got := client.httpClient().Timeout; got != rendererTimeout {
		t.Fatalf("timeout = %s, want %s", got, rendererTimeout)
	}
}

func assertWheelRemoteDefaults(t *testing.T, got wheelRenderRequest) {
	t.Helper()
	if got.DurationMs != wheelRemoteDurationMs {
		t.Fatalf("durationMs = %d, want %d", got.DurationMs, wheelRemoteDurationMs)
	}
	if got.HoldMs != wheelRemoteHoldMs {
		t.Fatalf("holdMs = %d, want %d", got.HoldMs, wheelRemoteHoldMs)
	}
	if got.FPS != wheelRemoteFPS {
		t.Fatalf("fps = %d, want %d", got.FPS, wheelRemoteFPS)
	}
	if got.Size != wheelRemoteSize {
		t.Fatalf("size = %d, want %d", got.Size, wheelRemoteSize)
	}
	if got.Theme != wheelRemoteTheme {
		t.Fatalf("theme = %q, want %q", got.Theme, wheelRemoteTheme)
	}
}
