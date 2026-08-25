package sticker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-telegram/bot"

	"github.com/tiennm99/miti99bot/internal/log"
)

const (
	// maxSourceBytes bounds what this module will pull over the network.
	// Telegram-compressed photo sizes are typically well under 500 KB.
	maxSourceBytes = 2 << 20

	// downloadTimeout is this module's own ceiling. The library's shared HTTP
	// client allows 60s, which is far too long to inherit on a dispatcher where
	// one slow handler stalls every other user.
	downloadTimeout = 8 * time.Second
)

// errDownloadFailed replaces every error from the download path.
//
// This is a security boundary, not tidiness. FileDownloadLink returns
// "https://api.telegram.org/file/bot<TOKEN>/<path>", and every transport
// failure from http.Client.Do is a *url.Error whose Error() embeds the full
// URL — which the dispatcher then logs verbatim. A mid-transfer timeout, which
// is trivially reachable, would print the bot token to stdout and every log
// shipper downstream.
var errDownloadFailed = errors.New("sticker: download failed")

// downloadClient is separate from the library's so its timeout is ours.
var downloadClient = &http.Client{Timeout: downloadTimeout}

// downloadFile fetches a Telegram file by ID, bounded in bytes and time.
//
// The original error is discarded rather than wrapped: wrapping would keep the
// URL reachable through errors.Unwrap and %v, which defeats the point.
func downloadFile(ctx context.Context, b *bot.Bot, fileID string) ([]byte, error) {
	f, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		log.Error("sticker_getfile", "file_id", fileID, "reason", classify(err))
		return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
	}
	if f.FileSize > maxSourceBytes {
		return nil, refuse(fmt.Sprintf("That image is too large — keep it under %d MB.", maxSourceBytes>>20))
	}

	link := b.FileDownloadLink(f)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		log.Error("sticker_download_request", "file_id", fileID, "reason", classify(err))
		return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		log.Error("sticker_download", "file_id", fileID, "reason", classify(err))
		return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Error("sticker_download", "file_id", fileID, "reason", "status", "status", resp.StatusCode)
		return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
	}

	// Never trust Content-Length; bound the reader itself. One extra byte is
	// read so an oversized body is detected rather than silently truncated.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
	if err != nil {
		log.Error("sticker_download_read", "file_id", fileID, "reason", classify(err))
		return nil, fmt.Errorf("file_id=%s: %w", fileID, errDownloadFailed)
	}
	if len(data) > maxSourceBytes {
		return nil, refuse(fmt.Sprintf("That image is too large — keep it under %d MB.", maxSourceBytes>>20))
	}
	return data, nil
}

// classify reduces an error to a coarse label that cannot contain a URL.
//
// It deliberately inspects only the error's *type*, never its text: any path
// that formats the original error risks carrying the token along with it.
func classify(err error) string {
	var urlErr *url.Error
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.As(err, &urlErr):
		if urlErr.Timeout() {
			return "timeout"
		}
		return "transport"
	}
	return "unknown"
}
