package stats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/systemstate"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

// newStatsCounter returns a fresh counter backed by an in-memory collection.
func newStatsCounter() *counter {
	col := storage.NewMemoryProvider().Collection("stats")
	return newCounter(col)
}

func TestNew_RegistersExpectedCommands(t *testing.T) {
	deps := modules.Deps{Store: storage.NewMemoryProvider().Collection("stats")}
	mod := New(deps)

	if len(mod.Commands) != 1 {
		t.Fatalf("commands count = %d, want 1", len(mod.Commands))
	}
	cmd := mod.Commands[0]
	if cmd.Name != "stats" {
		t.Errorf("command name = %q, want %q", cmd.Name, "stats")
	}
	if cmd.Visibility != modules.VisibilityPublic {
		t.Errorf("command visibility = %d, want Public", cmd.Visibility)
	}
	if cmd.Handler == nil {
		t.Error("command handler is nil")
	}
	if mod.CommandHook == nil {
		t.Error("CommandHook is nil")
	}
}

// updateFrom builds a minimal update carrying the sender ID + optional
// username. Pass username="" to exercise the "no Telegram username" branch.
func updateFrom(id int64, username string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{ID: id, Username: username},
		},
	}
}

func TestInc_PersistsCountInStore(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	c := newCounter(provider.Collection("stats"))
	docs := storage.Typed[usageEntry](provider.Collection("stats"))

	c.Inc(ctx, "ping", nil)
	c.Inc(ctx, "ping", nil)
	c.Inc(ctx, "wordle", nil)

	entry, _, err := docs.Get(ctx, usageKey("ping", 0))
	if err != nil {
		t.Fatalf("Get ping: %v", err)
	}
	if entry.Cmd != "ping" || entry.N != 2 || entry.UserID != 0 || entry.Username != "" {
		t.Errorf("ping entry = %+v, want anonymous ping count 2", entry)
	}

	entry2, _, err := docs.Get(ctx, usageKey("wordle", 0))
	if err != nil {
		t.Fatalf("Get wordle: %v", err)
	}
	if entry2.Cmd != "wordle" || entry2.N != 1 {
		t.Errorf("wordle entry = %+v, want count 1", entry2)
	}
}

func TestInc_WithUsernameWritesPairEntry(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	c := newCounter(provider.Collection("stats"))
	docs := storage.Typed[usageEntry](provider.Collection("stats"))

	c.Inc(ctx, "ping", updateFrom(42, "alice"))
	c.Inc(ctx, "ping", updateFrom(42, "alice"))

	entry, _, err := docs.Get(ctx, usageKey("ping", 42))
	if err != nil {
		t.Fatalf("ping:42: %v", err)
	}
	if entry.Cmd != "ping" || entry.UserID != 42 || entry.Username != "alice" || entry.N != 2 {
		t.Errorf("ping:42 = %+v, want alice pair count 2", entry)
	}

	if _, _, err := docs.Get(ctx, usageKey("ping", 0)); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("named user should not create anonymous ping bucket, got err=%v", err)
	}
}

func TestInc_EmptyUsernameSkipsUserAndPair(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	c := newCounter(provider.Collection("stats"))
	docs := storage.Typed[usageEntry](provider.Collection("stats"))

	c.Inc(ctx, "ping", updateFrom(42, ""))

	entry, _, err := docs.Get(ctx, usageKey("ping", 0))
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if entry.N != 1 || entry.UserID != 0 || entry.Username != "" {
		t.Errorf("anonymous ping entry = %+v, want count 1 without user", entry)
	}

	if _, _, err := docs.Get(ctx, usageKey("ping", 42)); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("ping:42 should be absent, got err=%v", err)
	}
}

func TestInc_RefreshesUsernameOnRename(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	c := newCounter(provider.Collection("stats"))
	docs := storage.Typed[usageEntry](provider.Collection("stats"))

	c.Inc(ctx, "ping", updateFrom(42, "alice"))
	c.Inc(ctx, "wordle", updateFrom(42, "alice"))
	c.Inc(ctx, "ping", updateFrom(42, "alice2"))

	ping, _, err := docs.Get(ctx, usageKey("ping", 42))
	if err != nil {
		t.Fatalf("ping:42: %v", err)
	}
	if ping.Username != "alice2" || ping.N != 2 {
		t.Errorf("ping:42 = %+v, want alice2 count 2", ping)
	}
	wordle, _, err := docs.Get(ctx, usageKey("wordle", 42))
	if err != nil {
		t.Fatalf("wordle:42: %v", err)
	}
	if wordle.Username != "alice2" || wordle.N != 1 {
		t.Errorf("wordle:42 = %+v, want refreshed alice2 count 1", wordle)
	}
}

func installStats(t *testing.T) (*testutil.RecordingBot, *counter) {
	t.Helper()
	rb := testutil.NewRecordingBot(t)
	c := newStatsCounter()
	mod := modules.Module{
		Commands: []modules.Command{statsCommand(c)},
	}
	reg := &modules.Registry{
		AllCommands: map[string]modules.Command{},
	}
	for _, cmd := range mod.Commands {
		reg.AllCommands[cmd.Name] = cmd
	}
	modules.Install(rb.Bot, reg, modules.Auth{})
	return rb, c
}

func TestStats_NoDataRepliesEmpty(t *testing.T) {
	rb, _ := installStats(t)
	rb.Bot.ProcessUpdate(context.Background(), testutil.NewPrivateMessage(1, "/stats"))

	got := rb.LastSent().Text()
	if got != "No command stats yet." {
		t.Errorf("empty stats reply = %q, want 'No command stats yet.'", got)
	}
}

func TestStats_ShowsCountsSortedByPopularity(t *testing.T) {
	ctx := context.Background()
	rb, c := installStats(t)

	c.Inc(ctx, "ping", nil)
	c.Inc(ctx, "wordle", nil)
	c.Inc(ctx, "wordle", nil)
	c.Inc(ctx, "wordle", nil)
	c.Inc(ctx, "loldle", nil)
	c.Inc(ctx, "loldle", nil)

	rb.Bot.ProcessUpdate(ctx, testutil.NewPrivateMessage(1, "/stats"))
	got := rb.LastSent().Text()

	if !strings.HasPrefix(got, "Command usage:") {
		t.Errorf("reply missing header: %q", got)
	}
	wordlePos := strings.Index(got, "/wordle:")
	loLdlePos := strings.Index(got, "/loldle:")
	pingPos := strings.Index(got, "/ping:")
	if wordlePos < 0 || loLdlePos < 0 || pingPos < 0 {
		t.Fatalf("reply missing expected commands: %q", got)
	}
	if wordlePos >= loLdlePos || loLdlePos >= pingPos {
		t.Errorf("commands not in descending count order: wordle=%d loldle=%d ping=%d in %q",
			wordlePos, loLdlePos, pingPos, got)
	}
}

func TestCommandHook_FiredThroughModulesBuild(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()

	reg, err := modules.Build(
		[]string{"stats"},
		map[string]modules.Factory{"stats": New},
		provider,
		modules.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(reg.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(reg.Modules))
	}

	rb := testutil.NewRecordingBot(t)
	modules.Install(rb.Bot, reg, modules.Auth{})

	reg.RunCommandHooks(ctx, "ping", nil)

	// Verify by reading back via typed store (the same collection Build gave to stats).
	statsStore := storage.Typed[usageEntry](provider.Collection("stats"))
	entry, _, err := statsStore.Get(ctx, usageKey("ping", 0))
	if err != nil {
		t.Fatalf("expected ping in store after hook: %v", err)
	}
	if entry.N != 1 {
		t.Errorf("ping = %d, want 1", entry.N)
	}
}

func TestInitStore_MigratesLegacyStatsOnceAndDeletesOldKeys(t *testing.T) {
	ctx := context.Background()
	provider := storage.NewMemoryProvider()
	statsColl := provider.Collection("stats")
	systemColl := provider.Collection(systemstate.CollectionName)
	legacyCounts := storage.Typed[legacyCountEntry](statsColl)
	legacyUsers := storage.Typed[legacyUserEntry](statsColl)

	if err := legacyCounts.Put(ctx, legacyCountPrefix+"ping", legacyCountEntry{N: 6}); err != nil {
		t.Fatalf("legacy count ping: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyCountPrefix+"wordle", legacyCountEntry{N: 2}); err != nil {
		t.Fatalf("legacy count wordle: %v", err)
	}
	if err := legacyUsers.Put(ctx, legacyUserPrefix+"1", legacyUserEntry{Username: "alice", N: 4}); err != nil {
		t.Fatalf("legacy user alice: %v", err)
	}
	if err := legacyUsers.Put(ctx, legacyUserPrefix+"2", legacyUserEntry{Username: "bob", N: 3}); err != nil {
		t.Fatalf("legacy user bob: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"ping:1", legacyCountEntry{N: 3}); err != nil {
		t.Fatalf("legacy pair ping alice: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"ping:2", legacyCountEntry{N: 2}); err != nil {
		t.Fatalf("legacy pair ping bob: %v", err)
	}
	if err := legacyCounts.Put(ctx, legacyPairPrefix+"wordle:2", legacyCountEntry{N: 2}); err != nil {
		t.Fatalf("legacy pair wordle bob: %v", err)
	}

	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	usageDocs := storage.Typed[usageEntry](statsColl)
	assertUsageEntry(t, usageDocs, usageKey("ping", 1), usageEntry{Cmd: "ping", UserID: 1, Username: "alice", N: 3})
	assertUsageEntry(t, usageDocs, usageKey("ping", 2), usageEntry{Cmd: "ping", UserID: 2, Username: "bob", N: 2})
	assertUsageEntry(t, usageDocs, usageKey("wordle", 2), usageEntry{Cmd: "wordle", UserID: 2, Username: "bob", N: 2})
	assertUsageEntry(t, usageDocs, usageKey("ping", 0), usageEntry{Cmd: "ping", N: 1})

	for _, key := range []string{
		legacyCountPrefix + "ping",
		legacyCountPrefix + "wordle",
		legacyPairPrefix + "ping:1",
		legacyPairPrefix + "ping:2",
		legacyPairPrefix + "wordle:2",
		legacyUserPrefix + "1",
		legacyUserPrefix + "2",
	} {
		if _, _, err := legacyCounts.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("legacy key %s should be deleted, got err=%v", key, err)
		}
	}

	sys := systemstate.New(systemColl)
	rec, ok, err := sys.Get(ctx, usageMigrationKey)
	if err != nil || !ok {
		t.Fatalf("migration marker ok=%v err=%v", ok, err)
	}
	if rec.Status != "done" || rec.Count != 4 {
		t.Fatalf("migration marker = %+v, want done count 4", rec)
	}

	if err := legacyCounts.Put(ctx, legacyCountPrefix+"coin", legacyCountEntry{N: 99}); err != nil {
		t.Fatalf("legacy count after marker: %v", err)
	}
	if err := InitStore(ctx, statsColl, systemColl); err != nil {
		t.Fatalf("InitStore second run: %v", err)
	}
	if _, _, err := usageDocs.Get(ctx, usageKey("coin", 0)); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("second InitStore should skip migration after marker, got err=%v", err)
	}
}

func assertUsageEntry(t *testing.T, docs storage.DocStore[usageEntry], key string, want usageEntry) {
	t.Helper()
	got, _, err := docs.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("usage entry %s: %v", key, err)
	}
	if got != want {
		t.Fatalf("usage entry %s = %+v, want %+v", key, got, want)
	}
}

// renderStats is the asserted-on surface for the subcommand views, since it
// returns the reply string without touching *bot.Bot. seedFixture below mirrors
// what Inc would have written across several users + commands.
func seedFixture(t *testing.T, c *counter) {
	t.Helper()
	ctx := context.Background()
	// alice (id=1): /ping x3, /wordle x1
	// bob   (id=2): /ping x1, /wordle x2
	// carol (id=3): /ping x1                        (username later cleared to test skip)
	for i := 0; i < 3; i++ {
		c.Inc(ctx, "ping", updateFrom(1, "alice"))
	}
	c.Inc(ctx, "wordle", updateFrom(1, "alice"))
	c.Inc(ctx, "ping", updateFrom(2, "bob"))
	c.Inc(ctx, "wordle", updateFrom(2, "bob"))
	c.Inc(ctx, "wordle", updateFrom(2, "bob"))
	c.Inc(ctx, "ping", updateFrom(3, "carol"))
}

func TestRenderStats_TopCommands(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "")
	if !strings.HasPrefix(got, "Command usage:\n") {
		t.Errorf("missing header: %q", got)
	}
	// totals: ping=5, wordle=3
	pingPos := strings.Index(got, "/ping: 5")
	wordlePos := strings.Index(got, "/wordle: 3")
	if pingPos < 0 || wordlePos < 0 {
		t.Fatalf("missing totals: %q", got)
	}
	if pingPos >= wordlePos {
		t.Errorf("ping should sort before wordle: %q", got)
	}
}

func TestRenderStats_Users(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "users")
	if !strings.HasPrefix(got, "Top users:\n") {
		t.Errorf("missing header: %q", got)
	}
	// totals: alice=4, bob=3, carol=1
	alicePos := strings.Index(got, "@alice: 4")
	bobPos := strings.Index(got, "@bob: 3")
	carolPos := strings.Index(got, "@carol: 1")
	if alicePos < 0 || bobPos < 0 || carolPos < 0 {
		t.Fatalf("missing users in %q", got)
	}
	if alicePos >= bobPos || bobPos >= carolPos {
		t.Errorf("not sorted desc: alice=%d bob=%d carol=%d in %q", alicePos, bobPos, carolPos, got)
	}
}

func TestRenderStats_UserCommands(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "user @alice")
	if !strings.HasPrefix(got, "Commands by @alice:\n") {
		t.Errorf("missing header: %q", got)
	}
	if !strings.Contains(got, "/ping: 3") {
		t.Errorf("missing /ping: 3 in %q", got)
	}
	if !strings.Contains(got, "/wordle: 1") {
		t.Errorf("missing /wordle: 1 in %q", got)
	}
	// bob should not appear in alice's view
	if strings.Contains(got, "@bob") {
		t.Errorf("bob leaked into alice view: %q", got)
	}
}

// Pins the leading-colon disambiguation in viewUserCommands: a user ID
// suffix like ":2" must not falsely match a pair key for a different user
// whose ID happens to end in "2" (e.g. 12, 22, 42, 142). Both reviewers
// flagged this as a potential bug; this test proves the absence of the bug.
func TestRenderStats_UserCommands_IDSuffixDoesNotFalseMatch(t *testing.T) {
	ctx := context.Background()
	c := newStatsCounter()
	// user 2 ("two") calls /ping; user 12 ("twelve") calls /wordle.
	c.Inc(ctx, "ping", updateFrom(2, "two"))
	c.Inc(ctx, "wordle", updateFrom(12, "twelve"))

	got := renderStats(ctx, c, "user @two")
	if !strings.HasPrefix(got, "Commands by @two:\n") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "/ping: 1") {
		t.Errorf("missing /ping for user 2: %q", got)
	}
	if strings.Contains(got, "/wordle") {
		t.Errorf("user 12's /wordle leaked into user 2's view via :2 suffix-match: %q", got)
	}
}

func TestRenderStats_UserCommands_BareNameAlsoWorks(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "user alice")
	if !strings.HasPrefix(got, "Commands by @alice:\n") {
		t.Errorf("missing header for bare-name lookup: %q", got)
	}
}

func TestRenderStats_UserNotFound(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "user @ghost")
	if got != "User @ghost not found." {
		t.Errorf("got %q, want not-found", got)
	}
}

func TestRenderStats_CmdUsers(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "cmd wordle")
	if !strings.HasPrefix(got, "Users of /wordle:\n") {
		t.Errorf("missing header: %q", got)
	}
	// /wordle: bob=2, alice=1
	bobPos := strings.Index(got, "@bob: 2")
	alicePos := strings.Index(got, "@alice: 1")
	if bobPos < 0 || alicePos < 0 {
		t.Fatalf("missing users in %q", got)
	}
	if bobPos >= alicePos {
		t.Errorf("not sorted desc: bob=%d alice=%d in %q", bobPos, alicePos, got)
	}
	if strings.Contains(got, "@carol") {
		t.Errorf("carol leaked into /wordle view (never invoked it): %q", got)
	}
}

func TestRenderStats_CmdNotFound(t *testing.T) {
	c := newStatsCounter()
	seedFixture(t, c)

	got := renderStats(context.Background(), c, "cmd nonexistent")
	if got != "Command /nonexistent has no users yet." {
		t.Errorf("got %q, want not-found", got)
	}
}

func TestRenderStats_UnknownSubcommand(t *testing.T) {
	c := newStatsCounter()
	got := renderStats(context.Background(), c, "bogus")
	if !strings.HasPrefix(got, "Usage:") {
		t.Errorf("expected usage string, got %q", got)
	}
}

func TestRenderStats_MissingRequiredArg(t *testing.T) {
	c := newStatsCounter()
	if got := renderStats(context.Background(), c, "user"); !strings.HasPrefix(got, "Usage:") {
		t.Errorf("user (no arg) should return usage, got %q", got)
	}
	if got := renderStats(context.Background(), c, "cmd"); !strings.HasPrefix(got, "Usage:") {
		t.Errorf("cmd (no arg) should return usage, got %q", got)
	}
}

func TestRenderTopN_Truncates(t *testing.T) {
	rows := make([]row, 0, 400)
	for i := 0; i < 400; i++ {
		rows = append(rows, row{display: fmt.Sprintf("/cmd%04d", i), n: int64(400 - i)})
	}
	got := renderTopN("Header:", rows, 0) // k=0 means no top-K cap
	if !strings.HasSuffix(got, "…(truncated)") {
		t.Errorf("expected truncation marker, got tail %q", tail(got))
	}
	if len(got) > telegramMaxLen+len("\n…(truncated)") {
		t.Errorf("output length %d exceeds telegramMaxLen+marker", len(got))
	}
}

func TestRenderTopN_RespectsK(t *testing.T) {
	rows := []row{
		{display: "/a", n: 10},
		{display: "/b", n: 9},
		{display: "/c", n: 8},
	}
	got := renderTopN("Header:", rows, 2)
	if strings.Contains(got, "/c:") {
		t.Errorf("k=2 should drop /c, got %q", got)
	}
	if !strings.Contains(got, "/a:") || !strings.Contains(got, "/b:") {
		t.Errorf("k=2 should keep /a and /b, got %q", got)
	}
}

func tail(s string) string {
	const n = 40
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
