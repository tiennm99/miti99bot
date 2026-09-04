package sticker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
)

// The download URL is "https://api.telegram.org/file/bot<TOKEN>/<path>", and
// every transport failure from http.Client.Do is a *url.Error whose Error()
// embeds it in full. The dispatcher logs a handler's returned error verbatim,
// so an error that carried the URL would print the bot token to stdout and
// every log shipper downstream.
//
// This asserts the property directly rather than trusting the discipline: a
// forced transport failure must produce an error that contains neither the
// token, nor "bot", nor any part of the URL.
func TestDownloadFile_ErrorNeverLeaksTokenOrURL(t *testing.T) {
	const token = "123456:SUPER-SECRET-BOT-TOKEN"

	// A server that accepts getFile, then hangs up mid-download.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getFile") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f1","file_unique_id":"u1","file_size":10,"file_path":"photos/file_1.jpg"}}`))
			return
		}
		// The file fetch: close the connection without a response.
		hj, ok := w.(http.Hijacker)
		if !ok {
			srv.CloseClientConnections()
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()

	b, err := bot.New(token, bot.WithSkipGetMe(), bot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}

	_, err = downloadFile(context.Background(), b, "f1", maxSourceBytes)
	if err == nil {
		t.Fatal("downloadFile succeeded against a hung-up server; want an error")
	}
	if !errors.Is(err, errDownloadFailed) {
		t.Errorf("err = %v, want it to be errDownloadFailed", err)
	}

	text := err.Error()
	for _, forbidden := range []string{token, "SUPER-SECRET", "bot", "http", srv.URL} {
		if strings.Contains(text, forbidden) {
			t.Errorf("error text %q contains %q — the token or URL can reach the logs", text, forbidden)
		}
	}
	// Unwrapping must not reach the original either: %v on a wrapped *url.Error
	// would put the URL back.
	if inner := errors.Unwrap(err); inner != nil && strings.Contains(inner.Error(), token) {
		t.Errorf("unwrapped error %q still carries the token", inner)
	}
}

// classify must reduce an error to a fixed label. It inspects only the error's
// type, never its text, so there is no path by which a URL can ride along.
func TestClassify_ReturnsFixedLabels(t *testing.T) {
	allowed := map[string]bool{"none": true, "timeout": true, "cancelled": true, "transport": true, "unknown": true}
	cases := []error{
		nil,
		context.DeadlineExceeded,
		context.Canceled,
		errors.New("https://api.telegram.org/file/bot123:SECRET/x.jpg refused"),
	}
	for _, err := range cases {
		got := classify(err)
		if !allowed[got] {
			t.Errorf("classify(%v) = %q, which is not one of the fixed labels", err, got)
		}
		if strings.Contains(got, "SECRET") || strings.Contains(got, "http") {
			t.Errorf("classify leaked error text: %q", got)
		}
	}
}

// The size guard runs on the metadata getFile returns, so an oversized file
// costs zero bytes of transfer.
func TestDownloadFile_RejectsOversizedBeforeFetching(t *testing.T) {
	var fetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getFile") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f1","file_unique_id":"u1","file_size":99999999,"file_path":"photos/huge.jpg"}}`))
			return
		}
		fetches++
		_, _ = w.Write([]byte("should never be fetched"))
	}))
	defer srv.Close()

	b, err := bot.New("t:t", bot.WithSkipGetMe(), bot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}

	if _, err := downloadFile(context.Background(), b, "f1", maxSourceBytes); err == nil {
		t.Fatal("downloadFile accepted an oversized file")
	}
	if fetches != 0 {
		t.Errorf("made %d HTTP fetches for an oversized file, want 0", fetches)
	}
}

// Content-Length is attacker-controlled; the reader itself is what bounds the
// transfer.
func TestDownloadFile_BoundsBodyRegardlessOfContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getFile") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f1","file_unique_id":"u1","file_size":10,"file_path":"photos/lie.jpg"}}`))
			return
		}
		// Claims to be tiny, sends far more.
		w.Header().Set("Content-Length", "10")
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		chunk := make([]byte, 64<<10)
		for written := 0; written < maxSourceBytes+(128<<10); written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	b, err := bot.New("t:t", bot.WithSkipGetMe(), bot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}

	data, err := downloadFile(context.Background(), b, "f1", maxSourceBytes)
	if err == nil {
		t.Fatalf("downloadFile accepted %d bytes despite the %d-byte cap", len(data), maxSourceBytes)
	}
}
