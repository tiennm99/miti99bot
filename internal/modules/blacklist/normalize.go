package blacklist

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Normalize converts user text to the single form used both as a storage key
// and as matcher input. It returns ok=false when nothing comparable is left.
//
// The three steps, in this order:
//
//  1. NFKC. Canonical composition is what makes Vietnamese portable: some
//     clients send "má" as "m" + "a" + combining acute (NFD) while others send
//     the precomposed character (NFC), and without this they are different
//     strings that would never match each other. The compatibility half also
//     folds full-width and other presentation variants onto their plain forms.
//     It does not strip combining marks — "ma" and "má" stay distinct entries,
//     which is the behaviour this module wants.
//  2. Lowercase, applied after normalization because case folding can otherwise
//     interact with composition.
//  3. Whitespace collapse, so "cat  dog" and "cat dog" are one rule and a
//     newline is just a separator.
func Normalize(s string) (string, bool) {
	fields := strings.Fields(strings.ToLower(norm.NFKC.String(s)))
	if len(fields) == 0 {
		return "", false
	}
	return strings.Join(fields, " "), true
}
