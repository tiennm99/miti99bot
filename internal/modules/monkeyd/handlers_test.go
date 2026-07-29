package monkeyd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiennm99/monkeyd-crawler/export"
	"github.com/tiennm99/monkeyd-crawler/pdfout"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

const testNovelURL = "https://monkeydd.com/tro-lai-nam-thang-cu.html"

// install wires the module to a recording bot. The returned runner is the one
// behind the registered command, so tests can substitute its exporter and run
// the export synchronously instead of on a detached goroutine.
//
// ownerID is permitted, which /monkeyd_crawl requires: the command is
// Protected, and the dispatcher drops unauthorized calls silently.
func install(t *testing.T, ownerID int64) (*testutil.RecordingBot, *runner) {
	t.Helper()
	rb := testutil.NewRecordingBot(t)
	r := newRunner()
	// Run the export inline so assertions do not race a detached goroutine.
	r.launch = func(job func()) { job() }
	mod := newModule(r)

	reg := &modules.Registry{
		Modules:     []modules.Module{{Name: "monkeyd", Commands: mod.Commands}},
		AllCommands: map[string]modules.Command{},
	}
	for _, c := range mod.Commands {
		reg.AllCommands[c.Name] = c
	}
	modules.Install(rb.Bot, reg, modules.Auth{BotOwnerID: ownerID})
	return rb, r
}

// stubPDF creates a file of the given size standing in for a rendered book and
// returns a Result describing it, as export.Export would. The file is sized by
// truncation so an over-the-limit case costs no real bytes.
func stubPDF(t *testing.T, dir, name string, size int64) *export.Result {
	t.Helper()
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create stub pdf: %v", err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatalf("size stub pdf: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close stub pdf: %v", err)
	}
	return &export.Result{
		Path:      path,
		Title:     "Example Novel",
		SourceURL: testNovelURL,
		Chapters:  3,
		Words:     1200,
		Page:      pdfout.Presets["phone"],
	}
}

func TestCrawl_NoArgumentRepliesUsage(t *testing.T) {
	rb, _ := install(t, 999)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(999, "/monkeyd_crawl"))

	if got := rb.LastSent().Text(); !strings.Contains(got, usage) {
		t.Errorf("reply = %q, want it to contain %q", got, usage)
	}
}

func TestCrawl_RejectsDisallowedHost(t *testing.T) {
	rb, r := install(t, 999)
	called := false
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		called = true
		return nil, nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl https://example.com/novel.html"))

	if called {
		t.Error("exporter ran for a disallowed host")
	}
	if got := rb.LastSent().Text(); !strings.Contains(got, AllowedHostsHint) {
		t.Errorf("reply = %q, want it to name %q", got, AllowedHostsHint)
	}
}

func TestCrawl_SendsPDFOnSuccess(t *testing.T) {
	rb, r := install(t, 999)
	dir := t.TempDir()

	var gotRequest export.Request
	r.exporter = func(_ context.Context, req export.Request) (*export.Result, error) {
		gotRequest = req
		return stubPDF(t, dir, "Example-Novel.pdf", 2048), nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

	if gotRequest.NovelURL != testNovelURL {
		t.Errorf("exporter got NovelURL %q, want %q", gotRequest.NovelURL, testNovelURL)
	}
	if gotRequest.OutDir == "" {
		t.Error("exporter got an empty OutDir; the PDF would land in the working directory")
	}

	calls := rb.Sent()
	if len(calls) < 2 {
		t.Fatalf("expected an acknowledgement and a document, got %d calls: %+v", len(calls), calls)
	}
	if first := calls[0]; !strings.Contains(first.Text(), testNovelURL) {
		t.Errorf("first reply = %q, want it to name the novel URL", first.Text())
	}
	last := calls[len(calls)-1]
	if last.Method != "sendDocument" {
		t.Fatalf("last call = %q, want sendDocument", last.Method)
	}
	if caption := last.Form["caption"]; !strings.Contains(caption, "Example Novel") {
		t.Errorf("caption = %q, want it to name the novel", caption)
	}
}

func TestCrawl_PassesRequestedFontSizeToExporter(t *testing.T) {
	rb, r := install(t, 999)
	dir := t.TempDir()

	var gotRequest export.Request
	r.exporter = func(_ context.Context, req export.Request) (*export.Result, error) {
		gotRequest = req
		return stubPDF(t, dir, "Example-Novel.pdf", 2048), nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL+" 14"))

	if gotRequest.FontSize != 14 {
		t.Errorf("exporter got FontSize %v, want 14", gotRequest.FontSize)
	}
	if caption := rb.LastSent().Form["caption"]; !strings.Contains(caption, "14pt") {
		t.Errorf("caption = %q, want it to report 14pt", caption)
	}
}

// Omitting the argument must leave Export to apply its own default rather than
// the bot inventing one, so the two cannot disagree.
func TestCrawl_OmittedFontSizeLeavesRequestZero(t *testing.T) {
	rb, r := install(t, 999)
	dir := t.TempDir()

	var gotRequest export.Request
	r.exporter = func(_ context.Context, req export.Request) (*export.Result, error) {
		gotRequest = req
		return stubPDF(t, dir, "Example-Novel.pdf", 2048), nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

	if gotRequest.FontSize != 0 {
		t.Errorf("exporter got FontSize %v, want 0 (defer to Export)", gotRequest.FontSize)
	}
	// The caption still has to name a real size, not "0pt".
	want := fmt.Sprintf("%gpt", export.DefaultFontSize)
	if caption := rb.LastSent().Form["caption"]; !strings.Contains(caption, want) {
		t.Errorf("caption = %q, want it to report %s", caption, want)
	}
}

func TestCrawl_RejectsBadFontSize(t *testing.T) {
	rb, r := install(t, 999)
	called := false
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		called = true
		return nil, nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL+" 100"))

	if called {
		t.Error("exporter ran despite an out-of-range font size")
	}
	if got := rb.LastSent().Text(); !strings.Contains(got, "between 6 and 24") {
		t.Errorf("reply = %q, want it to state the font size bounds", got)
	}
}

func TestCrawl_ReportsExportFailure(t *testing.T) {
	rb, r := install(t, 999)
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		return nil, errors.New("fetch failed: 404")
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

	last := rb.LastSent()
	if last.Method == "sendDocument" {
		t.Fatal("a document was sent despite the export failing")
	}
	if got := last.Text(); !strings.Contains(got, "404") {
		t.Errorf("failure reply = %q, want it to carry the underlying error", got)
	}
}

func TestCrawl_RefusesOversizedPDF(t *testing.T) {
	rb, r := install(t, 999)
	dir := t.TempDir()
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		return stubPDF(t, dir, "Huge-Novel.pdf", maxDocumentBytes+1), nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

	last := rb.LastSent()
	if last.Method == "sendDocument" {
		t.Fatal("an oversized document was uploaded instead of being refused")
	}
	if got := last.Text(); !strings.Contains(got, "limit") {
		t.Errorf("reply = %q, want it to explain the size limit", got)
	}
}

// The single export slot must be released whether the run succeeded or failed,
// otherwise the command is dead until the process restarts.
func TestCrawl_ReleasesSlotAfterRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exporter func(t *testing.T) func(context.Context, export.Request) (*export.Result, error)
	}{
		{
			name: "after success",
			exporter: func(t *testing.T) func(context.Context, export.Request) (*export.Result, error) {
				dir := t.TempDir()
				return func(context.Context, export.Request) (*export.Result, error) {
					return stubPDF(t, dir, "Example-Novel.pdf", 2048), nil
				}
			},
		},
		{
			name: "after failure",
			exporter: func(*testing.T) func(context.Context, export.Request) (*export.Result, error) {
				return func(context.Context, export.Request) (*export.Result, error) {
					return nil, errors.New("boom")
				}
			},
		},
		{
			name: "after panic",
			exporter: func(*testing.T) func(context.Context, export.Request) (*export.Result, error) {
				return func(context.Context, export.Request) (*export.Result, error) {
					panic("boom")
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rb, r := install(t, 999)
			r.exporter = tc.exporter(t)

			rb.Bot.ProcessUpdate(context.Background(),
				testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

			r.mu.Lock()
			running := r.running
			r.mu.Unlock()
			if running {
				t.Error("export slot still held after the run finished")
			}
		})
	}
}

func TestCrawl_RepliesBusyWhileAnotherExportRuns(t *testing.T) {
	rb, r := install(t, 999)
	// Simulate an in-flight export rather than racing a real one.
	if ok, _ := r.begin("https://monkeydd.com/other-novel.html"); !ok {
		t.Fatal("could not claim the export slot")
	}

	called := false
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		called = true
		return nil, nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewPrivateMessage(999, "/monkeyd_crawl "+testNovelURL))

	if called {
		t.Error("a second export started while one was already running")
	}
	got := rb.LastSent().Text()
	if !strings.Contains(got, "other-novel") {
		t.Errorf("busy reply = %q, want it to name the in-flight novel", got)
	}
}

// The command is public: a sender who is neither owner nor admin must still get
// a reply, including in a group.
func TestCrawl_AvailableToAnySender(t *testing.T) {
	rb, r := install(t, 999)
	dir := t.TempDir()
	r.exporter = func(context.Context, export.Request) (*export.Result, error) {
		return stubPDF(t, dir, "Example-Novel.pdf", 2048), nil
	}

	rb.Bot.ProcessUpdate(context.Background(),
		testutil.NewGroupMessage(-100, 12345, "/monkeyd_crawl "+testNovelURL))

	calls := rb.Sent()
	if len(calls) == 0 {
		t.Fatal("a non-admin sender got no reply from a public command")
	}
	if last := calls[len(calls)-1]; last.Method != "sendDocument" {
		t.Errorf("last call = %q, want sendDocument", last.Method)
	}
}

func TestRegistration(t *testing.T) {
	mod := New(modules.Deps{Store: storage.NewMemoryProvider().Collection("monkeyd")})
	if len(mod.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(mod.Commands))
	}
	cmd := mod.Commands[0]
	if cmd.Name != commandName {
		t.Errorf("Name = %q, want %q", cmd.Name, commandName)
	}
	if cmd.Visibility != modules.VisibilityPublic {
		t.Errorf("Visibility = %v, want Public", cmd.Visibility)
	}
	if cmd.Parameters != "<url> [font_size]" {
		t.Errorf("Parameters = %q, want %q", cmd.Parameters, "<url> [font_size]")
	}
	if cmd.Description == "" {
		t.Error("Description is empty; command discovery requires one")
	}
	if cmd.Handler == nil {
		t.Error("Handler is nil")
	}
	if len(mod.Crons) != 0 || len(mod.Callbacks) != 0 {
		t.Errorf("expected no crons or callbacks, got %d crons and %d callbacks",
			len(mod.Crons), len(mod.Callbacks))
	}
}
