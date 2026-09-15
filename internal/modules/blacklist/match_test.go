package blacklist

import "testing"

// The case the whole containment rule exists for. A whitelist entry rescues
// only the match it spans, so a text carrying both a rescued and an unrescued
// occurrence is still blocked.
func TestCheck_WhitelistRescuesOnlyWhatItSpans(t *testing.T) {
	black := []string{"ass"}
	white := []string{"assassin"}

	tests := []struct {
		name      string
		text      string
		blocked   bool
		entry     string
		rescuedBy string
	}{
		{name: "rescued", text: "assassin", blocked: false, entry: "ass", rescuedBy: "assassin"},
		{name: "not rescued", text: "dumbass", blocked: true, entry: "ass"},
		{name: "one of each", text: "i met an assassin, dumbass", blocked: true, entry: "ass"},
		{name: "no match at all", text: "hello there"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.text, black, white)
			if got.Blocked != tc.blocked || got.Entry != tc.entry || got.RescuedBy != tc.rescuedBy {
				t.Fatalf("Check(%q) = %+v; want blocked=%v entry=%q rescuedBy=%q",
					tc.text, got, tc.blocked, tc.entry, tc.rescuedBy)
			}
		})
	}
}

func TestCheck_WithoutWhitelistNothingIsRescued(t *testing.T) {
	if got := Check("assassin", []string{"ass"}, nil); !got.Blocked {
		t.Fatalf("Check = %+v; want blocked", got)
	}
}

func TestCheck_IdenticalWhitelistEntryRescuesWholeText(t *testing.T) {
	got := Check("cat", []string{"cat"}, []string{"cat"})
	if got.Blocked || got.Entry != "cat" || got.RescuedBy != "cat" {
		t.Fatalf("Check = %+v; want rescued by an identical entry", got)
	}
}

// A whitelist entry that overlaps a match without containing it is not a
// rescue: "bca" covers only the tail of the "ab" at index 0.
func TestCheck_OverlapWithoutContainmentDoesNotRescue(t *testing.T) {
	if got := Check("abca", []string{"ab"}, []string{"bca"}); !got.Blocked {
		t.Fatalf("Check = %+v; want blocked", got)
	}
}

// Overlapping occurrences of one entry must all be considered — scanning by
// whole-match strides would miss the second "aa" here and wrongly allow it.
func TestCheck_FindsOverlappingOccurrences(t *testing.T) {
	// "xaa" is whitelisted, spanning the first "aa" only.
	if got := Check("xaaa", []string{"aa"}, []string{"xaa"}); !got.Blocked {
		t.Fatalf("Check = %+v; want blocked by the second, unspanned occurrence", got)
	}
}

func TestCheck_EmptyTextMatchesNothing(t *testing.T) {
	if got := Check("", []string{"cat"}, nil); got != (Verdict{}) {
		t.Fatalf("Check = %+v; want zero verdict", got)
	}
}

// DocStore.List gives no ordering guarantee, so the verdict must not depend on
// the order entries arrive in.
func TestCheck_IsOrderIndependent(t *testing.T) {
	text := "the cat and the dog"
	forward := Check(text, []string{"cat", "dog"}, nil)
	reverse := Check(text, []string{"dog", "cat"}, nil)
	if forward != reverse {
		t.Fatalf("order changed the verdict: %+v vs %+v", forward, reverse)
	}
	if forward.Entry != "cat" {
		t.Fatalf("Entry = %q; want the first entry in sorted order", forward.Entry)
	}
}
