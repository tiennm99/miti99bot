package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteWebhookAt_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/deleteWebhook") {
			t.Errorf("path = %s, want suffix /deleteWebhook", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	if err := deleteWebhookAt(context.Background(), srv.URL, "token"); err != nil {
		t.Fatalf("deleteWebhookAt: %v", err)
	}
}

func TestDeleteWebhookAt_EmptyBody(t *testing.T) {
	// Reproduces the failing environment: 200 with an empty body. Must surface
	// as an error so the caller retries / warns rather than assuming success.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // no body
	}))
	defer srv.Close()

	if err := deleteWebhookAt(context.Background(), srv.URL, "token"); err == nil {
		t.Fatal("expected error on empty body, got nil")
	}
}

func TestDeleteWebhookAt_APIRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer srv.Close()

	err := deleteWebhookAt(context.Background(), srv.URL, "token")
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("want rejection error mentioning Unauthorized, got %v", err)
	}
}
