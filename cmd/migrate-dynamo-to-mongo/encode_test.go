package main

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestPayloadForItem_Object(t *testing.T) {
	got, err := payloadForItem("coin", "user:1", []byte(`{"bal":100,"meta":{"createdAt":5}}`))
	if err != nil {
		t.Fatalf("payloadForItem: %v", err)
	}
	if got["bal"] != int64(100) {
		t.Errorf("bal = %v (%T), want int64(100)", got["bal"], got["bal"])
	}
	if _, ok := got["value"]; ok {
		t.Error("payload must not contain a 'value' envelope field")
	}
	meta, ok := got["meta"].(bson.M)
	if !ok || meta["createdAt"] != int64(5) {
		t.Errorf("nested meta = %v, want {createdAt: int64(5)}", got["meta"])
	}
}

func TestPayloadForItem_LolscheduleSubscribers(t *testing.T) {
	got, err := payloadForItem("lolschedule", "subscribers", []byte(`[{"chatId":1},{"chatId":2}]`))
	if err != nil {
		t.Fatalf("payloadForItem: %v", err)
	}
	arr, ok := got["subscribers"].(bson.A)
	if !ok || len(arr) != 2 {
		t.Fatalf("subscribers = %v (%T), want 2-element bson.A", got["subscribers"], got["subscribers"])
	}
}

func TestPayloadForItem_LolscheduleLastPushRawString(t *testing.T) {
	// last-push date is stored as a bare (non-JSON) string in DynamoDB.
	got, err := payloadForItem("lolschedule", "daily_push:last_date", []byte(`2026-06-28`))
	if err != nil {
		t.Fatalf("payloadForItem: %v", err)
	}
	if got["date"] != "2026-06-28" {
		t.Errorf("date = %v, want 2026-06-28", got["date"])
	}
}

func TestPayloadForItem_UnknownNonObjectFailsLoud(t *testing.T) {
	if _, err := payloadForItem("coin", "user:1", []byte(`"a-bare-string"`)); err == nil {
		t.Error("non-object value with no wrap rule must fail loud")
	}
	if _, err := payloadForItem("coin", "user:1", []byte(`[1,2,3]`)); err == nil {
		t.Error("array value with no wrap rule must fail loud")
	}
}

func TestPayloadForItem_ReservedKeyCollision(t *testing.T) {
	if _, err := payloadForItem("coin", "user:1", []byte(`{"version":7}`)); err == nil {
		t.Error("payload key colliding with reserved root field must fail loud")
	}
}

// TestPayloadForItem_CamelCaseFidelity guards the invariant that lets migration
// work: the migrator preserves the original (camelCase) JSON keys, so a typed
// store struct must declare bson tags matching those names. A struct whose bson
// tags drifted to the driver's lowercased default would read these back empty.
func TestPayloadForItem_CamelCaseFidelity(t *testing.T) {
	payload, err := payloadForItem("lolschedule", "events", []byte(`{"startTime":"t","gameWins":3,"blockName":"Week 1"}`))
	if err != nil {
		t.Fatalf("payloadForItem: %v", err)
	}
	raw, err := bson.Marshal(payload)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var got struct {
		StartTime string `bson:"startTime"`
		GameWins  int    `bson:"gameWins"`
		BlockName string `bson:"blockName"`
	}
	if err := bson.Unmarshal(raw, &got); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}
	if got.StartTime != "t" || got.GameWins != 3 || got.BlockName != "Week 1" {
		t.Fatalf("camelCase round-trip lost data: %+v", got)
	}
}
