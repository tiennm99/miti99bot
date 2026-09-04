package telegram

import (
	"slices"
	"testing"
)

// pollingAllowedUpdates is a silent gate: Telegram filters getUpdates on its
// side, so a handler for a kind missing from this list is never called and
// leaves no log line, no error, and nothing to debug from. Inline queries were
// dropped exactly this way once — the handler was registered and the list was
// not updated.
//
// This test is the tripwire. A module that starts handling a new update kind
// belongs in both places, and removing a kind here must be deliberate enough to
// require editing this list too.
func TestPollingAllowedUpdates_CoversEveryHandledKind(t *testing.T) {
	// message       — every /command, and the alias fallback
	// callback_query — inline-keyboard buttons (stock dividends, loldle)
	// inline_query   — "@botname <prefix>" alias lookups
	want := []string{"message", "callback_query", "inline_query"}

	for _, kind := range want {
		if !slices.Contains(pollingAllowedUpdates, kind) {
			t.Errorf("%q missing from pollingAllowedUpdates; its handlers will never fire", kind)
		}
	}
	if len(pollingAllowedUpdates) != len(want) {
		t.Errorf("pollingAllowedUpdates = %v, want exactly %v — an unhandled kind costs bandwidth for nothing",
			pollingAllowedUpdates, want)
	}
}
