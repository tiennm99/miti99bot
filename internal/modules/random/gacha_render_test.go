package random

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRendererClient_EndpointJoinsAPIRoute(t *testing.T) {
	for raw, want := range map[string]string{
		"http://renderer:3000":            "http://renderer:3000/api/gacha",
		"http://renderer:3000/":           "http://renderer:3000/api/gacha",
		"https://render.example/proxied":  "https://render.example/proxied/api/gacha",
		"https://render.example/proxied/": "https://render.example/proxied/api/gacha",
	} {
		got, err := rendererClient{BaseURL: raw}.endpoint("gacha")
		if err != nil {
			t.Fatalf("endpoint(%q): %v", raw, err)
		}
		if got.String() != want {
			t.Errorf("endpoint(%q) = %q, want %q", raw, got, want)
		}
	}
	if _, err := (rendererClient{}).endpoint("gacha"); err != errRendererNotConfigured {
		t.Fatalf("empty url err = %v, want errRendererNotConfigured", err)
	}
}

func TestRendererClient_RenderGachaRejectsInvalidInput(t *testing.T) {
	client := rendererClient{BaseURL: "https://example.com"}
	for _, tc := range []struct {
		name   string
		label  string
		rarity int
	}{
		{name: "blank label", label: " ", rarity: 3},
		{name: "rarity too low", label: "a", rarity: 2},
		{name: "rarity too high", label: "a", rarity: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.RenderGacha(context.Background(), "gacha", tc.label, tc.rarity); err == nil {
				t.Fatal("RenderGacha returned nil error")
			}
		})
	}
}

func TestRendererClient_RenderGachaReturnsErrorsForBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        []byte
	}{
		{name: "server error", status: http.StatusInternalServerError, contentType: "text/plain", body: []byte("bad")},
		{name: "gif instead of mp4", status: http.StatusOK, contentType: "image/gif", body: []byte("GIF89a")},
		{name: "mislabeled mp4", status: http.StatusOK, contentType: "video/mp4", body: []byte("not an mp4")},
		{name: "empty mp4", status: http.StatusOK, contentType: "video/mp4", body: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()

			client := rendererClient{HTTP: server.Client(), BaseURL: server.URL}
			if _, err := client.RenderGacha(context.Background(), "gacha", "a", 3); err == nil {
				t.Fatal("RenderGacha returned nil error")
			}
		})
	}
}
