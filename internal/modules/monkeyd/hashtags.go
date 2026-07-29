package monkeyd

import (
	"strings"
	"unicode"
)

// sourceHashtag leads every tag line so the source is visible when the line is
// pasted somewhere on its own.
const sourceHashtag = "#MonkeyD"

// tagsMessage renders the tag line followed by the novel URL, separated by a
// blank line. Plain text: the caller decides how to present it.
func tagsMessage(tags []string, novelURL string) string {
	return hashtagLine(tags) + "\n\n" + novelURL
}

// hashtagLine turns genre labels into a single space-separated line of
// hashtags, led by sourceHashtag. Labels that cannot form a usable hashtag are
// dropped, and duplicates collapse — two labels differing only in punctuation
// or spacing produce the same hashtag.
func hashtagLine(tags []string) string {
	parts := []string{sourceHashtag}
	seen := map[string]bool{sourceHashtag: true}
	for _, tag := range tags {
		body := hashtag(tag)
		if body == "" {
			continue
		}
		tag := "#" + body
		if seen[tag] {
			continue
		}
		seen[tag] = true
		parts = append(parts, tag)
	}
	return strings.Join(parts, " ")
}

// hashtag converts a genre label into a hashtag body: "Cổ Đại" -> "CổĐại".
//
// Telegram ends a hashtag at the first character that is not a letter, digit or
// underscore, so spaces and punctuation are removed rather than replaced, and
// each word is capitalised to keep the boundaries readable. Diacritics survive:
// Telegram accepts non-ASCII letters in hashtags.
//
// Returns "" for a label with no letters. A digits-only hashtag is not linkified
// by Telegram, so emitting one would only add noise.
func hashtag(tag string) string {
	var b strings.Builder
	b.Grow(len(tag))

	hasLetter := false
	wordStart := true
	for _, r := range tag {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
			if wordStart {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			wordStart = false
		case unicode.IsDigit(r):
			b.WriteRune(r)
			wordStart = false
		default:
			// Dropped, but it still ends the word, so the next letter is
			// capitalised: "sci-fi" -> "SciFi".
			wordStart = true
		}
	}
	if !hasLetter {
		return ""
	}
	return b.String()
}
