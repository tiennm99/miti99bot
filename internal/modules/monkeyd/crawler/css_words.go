package crawler

import (
	"regexp"
	"strconv"
	"strings"
)

// The site hides part of every chapter behind CSS rather than putting it in the
// markup. Chapter HTML carries empty elements such as
//
//	Nghe <span class="t-3e625e..."></span> trưởng tử
//
// and the page stylesheet supplies the missing word:
//
//	.t-3e625e...:before { content: "vị"; }
//
// Reading DOM text alone therefore drops hundreds of words per chapter without
// any visible error. wordRule finds those rules so the words can be put back.
var wordRule = regexp.MustCompile(`\.([A-Za-z0-9_-]+)\s*::?before\s*\{[^}]*?content\s*:\s*"((?:[^"\\]|\\.)*)"`)

// cssEscape matches a CSS character escape: a hex code point, optionally
// followed by one whitespace terminator, or an escaped literal character.
var cssEscape = regexp.MustCompile(`\\([0-9A-Fa-f]{1,6})\s?|\\(.)`)

// ParseWordClasses maps CSS class name to the word its :before rule injects.
func ParseWordClasses(page []byte) map[string]string {
	words := make(map[string]string)
	for _, m := range wordRule.FindAllSubmatch(page, -1) {
		words[string(m[1])] = decodeCSSString(string(m[2]))
	}
	return words
}

// decodeCSSString resolves the escape sequences allowed inside a CSS string.
func decodeCSSString(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	return cssEscape.ReplaceAllStringFunc(s, func(esc string) string {
		m := cssEscape.FindStringSubmatch(esc)
		if m[1] != "" {
			if cp, err := strconv.ParseInt(m[1], 16, 32); err == nil && cp > 0 {
				return string(rune(cp))
			}
			return ""
		}
		return m[2]
	})
}
