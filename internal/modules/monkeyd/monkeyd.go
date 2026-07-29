// Package monkeyd exports a monkeydd.com novel as a PDF and sends it back as a
// Telegram document. The crawling and rendering live in the monkeyd-crawler
// submodule (third_party/monkeyd-crawler); this module is the Telegram surface
// around it: argument validation, one-at-a-time scheduling, and delivery.
package monkeyd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/monkeyd-crawler/export"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

// commandName is the single command this module exposes.
const commandName = "monkeyd_crawl"

// usage is shown when the command arrives without a usable URL. It repeats the
// Parameters syntax so the error and the command menu agree.
const usage = "Usage: /" + commandName + " <url>"

// New is the module Factory. The module keeps no persistent state — an export
// is a one-shot job — so deps.Store is unused.
func New(_ modules.Deps) modules.Module {
	return newModule(newRunner())
}

// newModule builds the module around a given runner, which is how tests supply
// one with a stubbed exporter and a synchronous launch.
func newModule(r *runner) modules.Module {
	return modules.Module{
		Commands: []modules.Command{
			{
				Name:       commandName,
				Visibility: modules.VisibilityProtected,
				// One invocation makes hundreds of outbound requests over
				// several minutes, so it stays off the public surface.
				Description: "Export a " + AllowedHostsHint + " novel as a PDF",
				Parameters:  "<url>",
				Handler:     r.handle,
			},
		},
	}
}

// runner serialises exports. The crawler spaces its own requests out per run,
// so two concurrent crawls would double the request rate against the site —
// and a novel is minutes of work, which makes queueing pointless. One at a
// time, globally, with a clear reply to anyone who asks meanwhile.
type runner struct {
	mu      sync.Mutex
	running bool
	current string // novel URL of the in-flight export, for the busy reply

	// exporter is the crawl-and-render step. It is a field so tests can
	// exercise scheduling and delivery without network access.
	exporter func(context.Context, export.Request) (*export.Result, error)

	// launch runs an export job. Production detaches it onto its own
	// goroutine; tests substitute a synchronous run for determinism.
	launch func(job func())
}

func newRunner() *runner {
	return &runner{
		exporter: export.Export,
		launch: func(job func()) {
			// Detached from the handler context on purpose: handlers run one
			// at a time (the bot is built WithNotAsyncHandlers), so crawling
			// inline would block every other command for minutes. The job
			// owns its own timeout and recovers its own panics.
			go job() //nolint:gosec // G118: intentional; see runner.export
		},
	}
}

// begin claims the single export slot, reporting the in-flight URL when it is
// already taken.
func (r *runner) begin(novelURL string) (ok bool, inFlight string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return false, r.current
	}
	r.running = true
	r.current = novelURL
	return true, ""
}

func (r *runner) end() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.current = ""
}

func (r *runner) handle(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	arg := chathelper.ArgAfterCommand(msg.Text)
	if arg == "" {
		return chathelper.Reply(ctx, b, msg, usage)
	}
	// Telegram may hand the URL over with trailing punctuation or a stray
	// second word; only the first token can be the URL.
	if fields := strings.Fields(arg); len(fields) > 0 {
		arg = fields[0]
	}

	novelURL, err := normalizeNovelURL(arg)
	if err != nil {
		return chathelper.Reply(ctx, b, msg, fmt.Sprintf("%s.\n%s", capitalize(err.Error()), usage))
	}

	ok, inFlight := r.begin(novelURL)
	if !ok {
		return chathelper.Reply(ctx, b, msg,
			"Already exporting "+inFlight+". Try again once it finishes.")
	}

	// Reply before starting so the user knows the wait is expected. If the
	// reply cannot be delivered, drop the slot rather than crawling for a chat
	// that will never hear the result.
	if err := chathelper.Reply(ctx, b, msg,
		"Exporting "+novelURL+" — this takes a few minutes. I will send the PDF here when it is ready."); err != nil {
		r.end()
		return err
	}

	r.launch(func() { r.export(b, msg, novelURL) })
	return nil
}

// capitalize upper-cases the first letter so a lower-case error string reads as
// a sentence in a Telegram reply.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
