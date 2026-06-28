package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// reservedRootFields are owned by the storage layer (see internal/storage); a
// flattened object payload must not collide with them.
var reservedRootFields = map[string]bool{"_id": true, "version": true, "updatedAt": true}

// wrapRule names the root field a non-object DynamoDB value must be wrapped in
// to match the typed Mongo store's named-struct shape. rawString treats the
// DynamoDB value as a bare (non-JSON) string; otherwise it is JSON-decoded.
type wrapRule struct {
	field     string
	rawString bool
}

// wrapRules covers the only KV entries whose value is not a JSON object. They
// mirror the named-struct wrappers the lolschedule module persists:
//   - subscribers: a JSON array → {subscribers: [...]}
//   - daily_push:last_date: a bare date string → {date: "..."}
//
// Any other non-object value fails loud in payloadForItem so a missing rule is
// obvious rather than silently dropped.
var wrapRules = map[[2]string]wrapRule{
	{"lolschedule", "subscribers"}:          {field: "subscribers"},
	{"lolschedule", "daily_push:last_date"}: {field: "date", rawString: true},
}

// payloadForItem converts one migrated KV row's value into the flattened payload
// map the typed Mongo store stores at the document root. The store adds _id,
// version, and updatedAt; this returns only the payload fields.
func payloadForItem(module, key string, value []byte) (bson.M, error) {
	if rule, ok := wrapRules[[2]string{module, key}]; ok {
		if rule.rawString {
			return bson.M{rule.field: string(value)}, nil
		}
		decoded, err := decodeJSONNumber(value)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: decode value: %w", module, key, err)
		}
		return bson.M{rule.field: decoded}, nil
	}

	decoded, err := decodeJSONNumber(value)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: decode value: %w", module, key, err)
	}
	obj, ok := decoded.(bson.M)
	if !ok {
		return nil, fmt.Errorf("%s/%s: value is not a JSON object (type %T) and has no wrap rule — add one to wrapRules", module, key, decoded)
	}
	for k := range obj {
		if reservedRootFields[k] {
			return nil, fmt.Errorf("%s/%s: payload key %q collides with a reserved root field — add a wrap rule", module, key, k)
		}
	}
	return obj, nil
}

// decodeJSONNumber decodes JSON into a BSON-native value, preserving integral
// numbers as int64 (UseNumber) so migrated numbers keep int64 fidelity, matching
// the live store's value codec.
func decodeJSONNumber(value []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return numberToBSON(v), nil
}

// numberToBSON walks a json.Unmarshal(UseNumber) tree into bson.M/bson.A,
// converting json.Number to int64 when integral, else float64.
func numberToBSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(bson.M, len(t))
		for k, val := range t {
			m[k] = numberToBSON(val)
		}
		return m
	case []any:
		a := make(bson.A, len(t))
		for i, val := range t {
			a[i] = numberToBSON(val)
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
		return v
	}
}
