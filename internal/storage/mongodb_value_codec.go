package storage

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// This file converts between the KVStore's opaque JSON []byte contract and a
// native BSON representation, so values render as expandable documents in the
// Atlas/Compass UI (and are queryable) rather than as opaque blobs.
//
// Fidelity: JSON is decoded with json.Number so integral numbers persist as
// BSON int64 (NOT float64) — a struct's int64 field round-trips without
// precision loss. Only JSON objects and arrays become native; bare scalars and
// non-JSON values (e.g. a plain date string) fall back to a BSON string.

// encodeValue produces the stored BSON representation of a value. A JSON object
// or array → native (expandable/queryable); anything else → a string (the
// human-readable fallback, also covers non-UTF-8-free callers identically to
// the previous string encoding).
func encodeValue(val []byte) any {
	if native, ok := jsonToNative(val); ok {
		return native
	}
	return string(val)
}

// jsonToNative decodes val into a native BSON value (bson.M / bson.A) when it is
// a JSON object or array, preserving integers as int64. Returns ok=false for
// scalars, non-JSON, or decode errors (caller stores those as a string).
func jsonToNative(val []byte) (any, bool) {
	trimmed := bytes.TrimSpace(val)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(val))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	// Reject trailing garbage after the JSON value.
	if dec.More() {
		return nil, false
	}
	return numbersToBSON(v), true
}

// numbersToBSON walks a json.Unmarshal(UseNumber) tree, converting maps/slices
// to bson.M/bson.A and json.Number to int64 (when integral) else float64.
func numbersToBSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(bson.M, len(t))
		for k, val := range t {
			m[k] = numbersToBSON(val)
		}
		return m
	case []any:
		a := make(bson.A, len(t))
		for i, val := range t {
			a[i] = numbersToBSON(val)
		}
		return a
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	default:
		return v // string, bool, nil
	}
}

// nativeToJSON re-serializes a natively-stored value (bson.M / bson.A / bson.D
// / scalar) back to JSON bytes. bson.M is map[string]any and bson.A is []any,
// both of which json.Marshal handles directly; bson.D (ordered) is normalized
// to a map first so it marshals as an object, not an array of key/value pairs.
func nativeToJSON(raw any) ([]byte, error) {
	out, err := json.Marshal(normalizeBSON(raw))
	if err != nil {
		return nil, fmt.Errorf("mongo: re-encode native value: %w", err)
	}
	return out, nil
}

// normalizeBSON converts bson.D to bson.M recursively so json.Marshal emits an
// object. bson.M/bson.A children are recursed; scalars pass through.
func normalizeBSON(v any) any {
	switch t := v.(type) {
	case bson.D:
		m := make(bson.M, len(t))
		for _, e := range t {
			m[e.Key] = normalizeBSON(e.Value)
		}
		return m
	case bson.M:
		for k, val := range t {
			t[k] = normalizeBSON(val)
		}
		return t
	case bson.A:
		for i, val := range t {
			t[i] = normalizeBSON(val)
		}
		return t
	default:
		return v
	}
}
