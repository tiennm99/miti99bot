package blacklist

import (
	"strconv"
	"strings"
)

// The two list tags. They are single letters because they sit inside every
// storage key, and because the ':' after them is what ends the key prefix.
const (
	listBlack = "b"
	listWhite = "w"
)

// maxEntryBytes caps one entry, measured on the normalized text before key
// encoding.
//
// Storage keys are limited to 1500 bytes (internal/storage/keys.go). Worst-case
// encoding triples an entry to 600 bytes, which with a prefix well under 30
// bytes stays far inside that. Two hundred bytes is roughly 65 Vietnamese
// characters — ample for a phrase, and a deliberate refusal to let a pasted
// essay become a key.
const maxEntryBytes = 200

// scopePrefix is the key prefix for one list of one thread.
//
// A thread is (Chat.ID, MessageThreadID), the same pair the lol module uses for
// subscriptions. A zero thread id means a non-forum group, a forum's General
// topic, or a DM — a real scope, not a missing value.
//
// Both numbers are decimal and the list tag is a single letter, so the third
// ':' always ends the prefix even though entry text may contain ':' of its own.
func scopePrefix(chatID int64, threadID int, list string) string {
	return strconv.FormatInt(chatID, 10) + ":" + strconv.Itoa(threadID) + ":" + list + ":"
}

// entryKey names one entry. normText must already have been through Normalize.
func entryKey(chatID int64, threadID int, list, normText string) string {
	return scopePrefix(chatID, threadID, list) + encodeKeyText(normText)
}

// encodeKeyText escapes the one character that cannot appear literally in a
// storage key.
//
// '%' is escaped first so the escape marker itself stays unambiguous: reversing
// the order would make an entry containing the literal text "%2F" decode back
// as "/". Because every literal '%' becomes "%25", every '%' in the result
// starts an escape, so decoding can never match one that spans two of them.
//
// The other key rules need no work here: "." , ".." and the __namespace__
// pattern are all unreachable behind the numeric scope prefix.
func encodeKeyText(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	return strings.ReplaceAll(s, "/", "%2F")
}

// decodeKeyText reverses encodeKeyText. The order mirrors it: "%2F" can only
// have come from a real '/', so it is consumed before "%25" reconstitutes the
// '%' characters that could otherwise form a spurious escape.
func decodeKeyText(s string) string {
	s = strings.ReplaceAll(s, "%2F", "/")
	return strings.ReplaceAll(s, "%25", "%")
}
