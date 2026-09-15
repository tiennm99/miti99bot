package blacklist

import (
	"strings"
	"testing"
)

func TestScopePrefix_EndsAfterThreeColons(t *testing.T) {
	got := scopePrefix(-1001234567890, 0, listBlack)
	if want := "-1001234567890:0:b:"; got != want {
		t.Fatalf("scopePrefix = %q, want %q", got, want)
	}
	if strings.Count(got, ":") != 3 {
		t.Fatalf("prefix %q does not carry exactly three colons", got)
	}
}

// Two threads of one chat must produce prefixes where neither is a prefix of
// the other, or a List for one thread would return the other's entries.
func TestScopePrefix_ThreadsDoNotNest(t *testing.T) {
	a := scopePrefix(-100, 1, listBlack)
	b := scopePrefix(-100, 12, listBlack)
	if strings.HasPrefix(b, a) || strings.HasPrefix(a, b) {
		t.Fatalf("thread prefixes nest: %q and %q", a, b)
	}
}

func TestScopePrefix_ListsDoNotCollide(t *testing.T) {
	if scopePrefix(-100, 0, listBlack) == scopePrefix(-100, 0, listWhite) {
		t.Fatal("blacklist and whitelist share a prefix")
	}
}

func TestEncodeKeyText_RoundTrips(t *testing.T) {
	for _, in := range []string{
		"plain",
		"a/b",
		"100%",
		"%2F", // the literal text that a naive decoder turns into "/"
		"%25", // likewise for "%"
		"a:b", // the key delimiter, which needs no escaping
		"a\nb",
		"%2F/%25%", // every hazard at once
	} {
		enc := encodeKeyText(in)
		if strings.Contains(enc, "/") {
			t.Fatalf("encodeKeyText(%q) = %q still contains '/'", in, enc)
		}
		if got := decodeKeyText(enc); got != in {
			t.Fatalf("round trip of %q gave %q (encoded %q)", in, got, enc)
		}
	}
}

func TestEntryKey_StaysWithinStorageLimits(t *testing.T) {
	// Worst case: an entry at the cap made entirely of characters that encode
	// to three bytes each.
	key := entryKey(-1001234567890, 999999, listBlack, strings.Repeat("/", maxEntryBytes))
	if len(key) > 1500 {
		t.Fatalf("worst-case key is %d bytes, over the 1500-byte storage limit", len(key))
	}
}
