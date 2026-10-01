package misc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGachaAPIEndpoint_SwapsLastPathSegment(t *testing.T) {
	for raw, want := range map[string]string{
		"https://render.example/api/gif":  "https://render.example/api/gacha",
		"https://render.example/api/gif/": "https://render.example/api/gacha",
		"http://10.0.0.5:3000/api/gif":    "http://10.0.0.5:3000/api/gacha",
	} {
		got, err := gachaAPIEndpoint(raw)
		if err != nil {
			t.Fatalf("gachaAPIEndpoint(%q): %v", raw, err)
		}
		if got.String() != want {
			t.Errorf("gachaAPIEndpoint(%q) = %q, want %q", raw, got, want)
		}
	}
	if _, err := gachaAPIEndpoint(""); err != errWheelAPINotConfigured {
		t.Fatalf("empty url err = %v, want errWheelAPINotConfigured", err)
	}
}

func TestWheelAPIClient_RenderGachaRejectsInvalidInput(t *testing.T) {
	client := wheelAPIClient{URL: "https://example.com/api/gif"}
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
			if _, err := client.RenderGacha(context.Background(), tc.label, tc.rarity); err == nil {
				t.Fatal("RenderGacha returned nil error")
			}
		})
	}
}

func TestWheelAPIClient_RenderGachaReturnsErrorsForBadResponses(t *testing.T) {
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

			client := wheelAPIClient{HTTP: server.Client(), URL: server.URL + "/api/gif"}
			if _, err := client.RenderGacha(context.Background(), "a", 3); err == nil {
				t.Fatal("RenderGacha returned nil error")
			}
		})
	}
}
