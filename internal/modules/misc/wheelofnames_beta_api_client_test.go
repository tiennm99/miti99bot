package misc

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

func TestWheelBetaAPIClient_RenderValidRequest(t *testing.T) {
	var got wheelBetaAPIRequest
	var gotAccept string
	var gotAuthorization string
	var gotContentType string
	var gotMethod string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotAuthorization = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("Decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a-remote"))
	}))
	defer server.Close()

	client := wheelBetaAPIClient{
		HTTP:  server.Client(),
		URL:   server.URL + "/api/gif",
		Token: "secret-token",
	}
	data, err := client.Render(context.Background(), []string{"alice", "bob", "carol"}, 1)
	if err != nil {
		t.Fatalf("Render: %v", err)
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
	if gotAuthorization != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuthorization)
	}
	if !slices.Equal(got.Options, []string{"alice", "bob", "carol"}) {
		t.Fatalf("options = %#v, want original options", got.Options)
	}
	if got.WinnerIndex != 1 {
		t.Fatalf("winnerIndex = %d, want 1", got.WinnerIndex)
	}
	assertWheelBetaRemoteDefaults(t, got)
}

func TestWheelBetaAPIClient_RenderWithoutTokenOmitsAuthorization(t *testing.T) {
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte("GIF89a"))
	}))
	defer server.Close()

	client := wheelBetaAPIClient{HTTP: server.Client(), URL: server.URL + "/api/gif"}
	if _, err := client.Render(context.Background(), []string{"alice"}, 0); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if gotAuthorization != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuthorization)
	}
}

func TestWheelBetaAPIClient_RenderNotConfigured(t *testing.T) {
	client := wheelBetaAPIClient{}
	_, err := client.Render(context.Background(), []string{"alice"}, 0)
	if !errors.Is(err, errWheelBetaAPINotConfigured) {
		t.Fatalf("Render error = %v, want errWheelBetaAPINotConfigured", err)
	}
}

func TestWheelBetaAPIClient_RenderRejectsInvalidInput(t *testing.T) {
	client := wheelBetaAPIClient{URL: "https://example.com/api/gif"}
	for _, tc := range []struct {
		name    string
		url     string
		options []string
		winner  int
	}{
		{name: "bad scheme", url: "ftp://example.com/api/gif", options: []string{"alice"}, winner: 0},
		{name: "empty options", url: "https://example.com/api/gif", options: nil, winner: 0},
		{name: "winner out of range", url: "https://example.com/api/gif", options: []string{"alice"}, winner: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client.URL = tc.url
			if _, err := client.Render(context.Background(), tc.options, tc.winner); err == nil {
				t.Fatalf("Render returned nil error")
			}
		})
	}
}

func TestWheelBetaAPIClient_RenderReturnsErrorsForBadResponses(t *testing.T) {
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

			client := wheelBetaAPIClient{HTTP: server.Client(), URL: server.URL + "/api/gif"}
			if _, err := client.Render(context.Background(), []string{"alice"}, 0); err == nil {
				t.Fatalf("Render returned nil error")
			}
		})
	}
}

func TestWheelBetaAPIClient_RenderRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write(bytes.Repeat([]byte("a"), int(wheelBetaRemoteMaxBytes)+1))
	}))
	defer server.Close()

	client := wheelBetaAPIClient{HTTP: server.Client(), URL: server.URL + "/api/gif"}
	if _, err := client.Render(context.Background(), []string{"alice"}, 0); err == nil {
		t.Fatalf("Render returned nil error")
	}
}

func TestWheelBetaAPIClient_DefaultHTTPClientHasTimeout(t *testing.T) {
	client := wheelBetaAPIClient{}
	if got := client.httpClient().Timeout; got != wheelBetaRemoteTimeout {
		t.Fatalf("timeout = %s, want %s", got, wheelBetaRemoteTimeout)
	}
}

func assertWheelBetaRemoteDefaults(t *testing.T, got wheelBetaAPIRequest) {
	t.Helper()
	if got.DurationMs != wheelBetaRemoteDurationMs {
		t.Fatalf("durationMs = %d, want %d", got.DurationMs, wheelBetaRemoteDurationMs)
	}
	if got.HoldMs != wheelBetaRemoteHoldMs {
		t.Fatalf("holdMs = %d, want %d", got.HoldMs, wheelBetaRemoteHoldMs)
	}
	if got.FPS != wheelBetaRemoteFPS {
		t.Fatalf("fps = %d, want %d", got.FPS, wheelBetaRemoteFPS)
	}
	if got.Size != wheelBetaRemoteSize {
		t.Fatalf("size = %d, want %d", got.Size, wheelBetaRemoteSize)
	}
	if got.Theme != wheelBetaRemoteTheme {
		t.Fatalf("theme = %q, want %q", got.Theme, wheelBetaRemoteTheme)
	}
}
