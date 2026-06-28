package storage

import (
	"fmt"
	"regexp"
	"strings"
)

// maxKeyLen bounds a key's byte length. Originally Firestore's document-id cap;
// retained as a backend-neutral guard so keys behave identically across all
// backends (len(string) is bytes, not runes — do NOT switch to RuneCount).
const maxKeyLen = 1500

// collectionNameRe is the canonical module-name alphabet (mirrors
// modules.moduleNameRe). Providers re-validate module names against it as
// defense-in-depth before using a name as a collection/partition.
var collectionNameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// validateKey enforces key constraints up-front so callers get a clear error
// instead of an opaque backend error. Kept uniform across backends for parity.
//
// Forbidden: empty; longer than maxKeyLen bytes; contains '/'; "." or "..";
// leading+trailing "__" (reserved namespace).
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("storage: key is empty")
	}
	if len(key) > maxKeyLen {
		return fmt.Errorf("storage: key exceeds %d bytes", maxKeyLen)
	}
	if strings.Contains(key, "/") {
		return fmt.Errorf("storage: key contains '/'")
	}
	if key == "." || key == ".." {
		return fmt.Errorf("storage: key %q is reserved", key)
	}
	if strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__") {
		return fmt.Errorf("storage: key %q uses reserved __namespace__ pattern", key)
	}
	return nil
}

// validatePrefix runs validateKey but allows the empty string (List with an
// empty prefix scans the whole collection/partition).
func validatePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	return validateKey(prefix)
}

// prefixSuccessor returns the smallest string strictly greater than every
// string with the given prefix, for half-open range scans on keys. "abc" → "abd".
// Trailing 0xFF bytes are stripped and the last < 0xFF byte incremented. An
// all-0xFF prefix has no same-length successor and is returned unchanged
// (callers using such keys get an empty range — acceptable).
func prefixSuccessor(prefix string) string {
	b := []byte(prefix)
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < 0xFF {
			b[i]++
			return string(b[:i+1])
		}
	}
	return prefix
}
