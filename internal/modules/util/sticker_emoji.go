package util

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	// defaultEmoji is used when a sticker is added with no emoji given and the
	// source carries none. Telegram requires at least one.
	defaultEmoji = "⭐"
	// maxEmojiPerSticker mirrors the documented emoji_list range of 1-20.
	maxEmojiPerSticker = 20
)

// Code points that bind to the cluster before them rather than starting one.
// Written as hex because the literals are invisible in source.
const (
	zwj                 = rune(0x200D)  // zero-width joiner: 👩‍👩‍👧 is one emoji
	variationSelector16 = rune(0xFE0F)  // forces emoji presentation
	variationSelector15 = rune(0xFE0E)  // forces text presentation
	keycapCombining     = rune(0x20E3)  // the enclosing box of 1️⃣
	tagLow              = rune(0xE0020) // tag block: 🏴 + tags spells a subdivision flag
	tagHigh             = rune(0xE007F)
)

// parseEmoji splits an emoji argument run into individual emoji, accepting both
// "😂 🔥" and "😂🔥".
//
// Go has no stdlib grapheme segmentation, and a dependency for this one job is
// disproportionate — so this hand-rolls the subset of UAX #29 that emoji need:
// ZWJ sequences, variation selectors, skin-tone modifiers, regional-indicator
// pairs, and keycaps. A sequence that splits wrongly is a test case to add, not
// a redesign.
//
// /addsticker takes no argument but emoji, so every argument reaching here is
// meant to be one: a stray word fails loudly rather than being silently
// reinterpreted.
func parseEmoji(args []string) ([]string, error) {
	joined := strings.Join(args, "")
	joined = strings.TrimSpace(joined)
	if joined == "" {
		return nil, nil
	}

	var out []string
	for _, cluster := range splitClusters(joined) {
		if isSpace(cluster) {
			continue
		}
		if !isEmojiCluster(cluster) {
			return nil, refuse(fmt.Sprintf("%q is not an emoji. Give one or more emoji, like 😂🔥.", cluster))
		}
		out = append(out, cluster)
	}
	if len(out) > maxEmojiPerSticker {
		return nil, refuse(fmt.Sprintf("At most %d emoji per sticker.", maxEmojiPerSticker))
	}
	return out, nil
}

// splitClusters breaks s into emoji-aware clusters.
func splitClusters(s string) []string {
	runes := []rune(s)
	var out []string
	for i := 0; i < len(runes); {
		start := i
		i++
		// A regional indicator pairs with a following one to form a flag.
		if isRegionalIndicator(runes[start]) && i < len(runes) && isRegionalIndicator(runes[i]) {
			i++
			out = append(out, string(runes[start:i]))
			continue
		}
		// Absorb everything that binds leftward: modifiers, variation
		// selectors, combining marks, keycaps, and ZWJ-joined continuations.
		for i < len(runes) {
			r := runes[i]
			switch {
			case r == zwj:
				// Absorb the joiner and its continuation only when what follows
				// can actually continue an emoji sequence. Taking the next rune
				// unconditionally let "😀<ZWJ>🇻🇳" swallow the flag's first
				// half and emit the orphaned second half as its own cluster —
				// two invalid emoji, both of which passed validation.
				if i+1 < len(runes) && isEmojiRune(runes[i+1]) && !isRegionalIndicator(runes[i+1]) {
					i += 2
					continue
				}
				// A dangling joiner. Absorb it so it cannot start a cluster of
				// its own; trimJoiners drops it from the emitted cluster.
				i++
				goto done
			case isBinding(r):
				i++
			default:
				goto done
			}
		}
	done:
		// A cluster ending in a joiner is incomplete. "😀<ZWJ>" is not an emoji
		// and Telegram rejects it, but it used to pass validation because the
		// cluster's first rune looked fine.
		if cluster := trimJoiners(string(runes[start:i])); cluster != "" {
			out = append(out, cluster)
		}
	}
	return out
}

// trimJoiners strips leading and trailing zero-width joiners from a cluster.
func trimJoiners(cluster string) string {
	return strings.Trim(cluster, string(zwj))
}

// isBinding reports whether r attaches to the preceding cluster.
func isBinding(r rune) bool {
	switch {
	case r == variationSelector16, r == variationSelector15, r == keycapCombining:
		return true
	case r >= tagLow && r <= tagHigh:
		// Subdivision flags (🏴 + "gbeng" + terminator) are one emoji. Without
		// this the base flag split from its tags and the tags, being invisible,
		// produced a refusal quoting characters the user could not see.
		return true
	case isSkinTone(r):
		return true
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r):
		return true
	}
	return false
}

func isSkinTone(r rune) bool          { return r >= 0x1F3FB && r <= 0x1F3FF }
func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isSpace(cluster string) bool { return strings.TrimSpace(cluster) == "" }

// isEmojiCluster reports whether a cluster is emoji rather than ordinary text.
//
// The test is on the cluster's first rune, since that is what determines the
// cluster's identity — the rest is bound modifiers. Keycaps are the exception:
// "1️⃣" starts with an ASCII digit, so a cluster carrying the keycap combining
// mark counts regardless of its base.
func isEmojiCluster(cluster string) bool {
	runes := []rune(cluster)
	if len(runes) == 0 {
		return false
	}
	if strings.ContainsRune(cluster, keycapCombining) {
		return true
	}
	if isRegionalIndicator(runes[0]) {
		// A flag is exactly two regional indicators. An odd count leaves a lone
		// one at the end, which is not an emoji — Telegram rejects it, and it
		// used to pass because isEmojiRune accepts the block.
		return len(runes) == 2 && isRegionalIndicator(runes[1])
	}
	return isEmojiRune(runes[0])
}

// emojiRanges are the Unicode blocks Telegram's emoji actually come from.
// Deliberately ranges rather than a property lookup: Go's unicode package
// exposes no Emoji property, and the blocks are stable.
var emojiRanges = [...]struct{ lo, hi rune }{
	{0x1F300, 0x1FAFF}, // pictographs, emoticons, transport, symbols, extended-A
	{0x1F000, 0x1F2FF}, // mahjong, dominoes, playing cards, enclosed
	{0x2600, 0x27BF},   // misc symbols + dingbats
	{0x2B00, 0x2BFF},   // arrows and stars (⭐ lives here)
	{0x2190, 0x21FF},   // arrows
	{0x2300, 0x23FF},   // misc technical (⌚, ⏰)
	{0x25A0, 0x25FF},   // geometric shapes
	{0x1F1E6, 0x1F1FF}, // regional indicators; isEmojiCluster requires a pair
}

// emojiSingletons are emoji stranded between the blocks above.
//
// Listed one by one rather than by widening a range, because their neighbours
// are not emoji: Ⓜ sits in enclosed alphanumerics next to circled digits, and ©
// and ® sit in Latin-1 next to ordinary punctuation. Widening to cover them
// would start accepting text as emoji, which Telegram then rejects.
var emojiSingletons = map[rune]bool{
	0x203C: true, // ‼
	0x2049: true, // ⁉
	0x2122: true, // ™
	0x2139: true, // ℹ
	0x00A9: true, // ©
	0x00AE: true, // ®
	0x2934: true, // ⤴
	0x2935: true, // ⤵
	0x24C2: true, // Ⓜ
	0x3030: true, // 〰
	0x303D: true, // 〽
	0x3297: true, // ㊗
	0x3299: true, // ㊙
}

func isEmojiRune(r rune) bool {
	if emojiSingletons[r] {
		return true
	}
	for _, block := range emojiRanges {
		if r >= block.lo && r <= block.hi {
			return true
		}
	}
	return false
}
