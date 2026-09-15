package blacklist

import (
	"slices"
	"strings"
)

// Verdict is the outcome of checking one text against one thread's rules.
//
// Entry is set whenever some blacklist entry occurred in the text, blocked or
// not: a rescued match is worth naming, because it is the only way a user can
// confirm an exception they added is doing anything.
type Verdict struct {
	Blocked   bool
	Entry     string // the blacklist entry that decided the verdict; "" when none matched
	RescuedBy string // the whitelist entry covering Entry; set only when !Blocked && Entry != ""
}

// span is a half-open byte range within the text being checked.
type span struct{ start, end int }

// allowSpan is a whitelist occurrence, carrying the entry that produced it so a
// rescue can be reported by name.
type allowSpan struct {
	span
	entry string
}

// Check judges normText against two lists of normalized entries.
//
// The rule is span containment, not mere presence: a whitelist entry rescues a
// blacklist match only when it occurs in the text at a range that contains that
// match. The tempting shortcut — allow the text if any whitelist entry appears
// anywhere in it — handles "assassin" correctly and then lets "I met an
// assassin, dumbass" through, because the rescuing span never reaches the
// second match. A whitelist entry that merely overlaps a match without
// containing it does not rescue it.
//
// Both slices are copied and sorted before scanning. Callers read them from
// DocStore.List, which guarantees no ordering, so without this the same text
// could be reported against a different entry on each invocation.
func Check(normText string, blacklist, whitelist []string) Verdict {
	if normText == "" {
		return Verdict{}
	}

	allowed := allowSpans(normText, whitelist)

	// The first rescued match, kept in case no unrescued one is ever found.
	var rescued Verdict

	for _, entry := range slices.Sorted(slices.Values(blacklist)) {
		if entry == "" {
			continue
		}
		for _, s := range occurrences(normText, entry) {
			by, ok := coveredBy(s, allowed)
			if !ok {
				return Verdict{Blocked: true, Entry: entry}
			}
			if rescued.Entry == "" {
				rescued = Verdict{Entry: entry, RescuedBy: by}
			}
		}
	}
	return rescued
}

// allowSpans collects every occurrence of every whitelist entry.
func allowSpans(text string, whitelist []string) []allowSpan {
	var out []allowSpan
	for _, entry := range slices.Sorted(slices.Values(whitelist)) {
		if entry == "" {
			continue
		}
		for _, s := range occurrences(text, entry) {
			out = append(out, allowSpan{span: s, entry: entry})
		}
	}
	return out
}

// occurrences returns every range at which sub appears in s.
//
// The scan advances one byte past each hit rather than past the whole match, so
// overlapping occurrences are all reported — "aa" occurs twice in "aaa", and a
// containment test that saw only the first would be wrong.
func occurrences(s, sub string) []span {
	var out []span
	for off := 0; off < len(s); {
		i := strings.Index(s[off:], sub)
		if i < 0 {
			break
		}
		start := off + i
		out = append(out, span{start: start, end: start + len(sub)})
		off = start + 1
	}
	return out
}

// coveredBy reports the whitelist entry whose span contains s, if any.
func coveredBy(s span, allowed []allowSpan) (string, bool) {
	for _, a := range allowed {
		if a.start <= s.start && s.end <= a.end {
			return a.entry, true
		}
	}
	return "", false
}
