package api

import (
	"encoding/json"
	"testing"
)

func TestJSONSliceEncodesNilAsEmptyArray(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"items": jsonSlice([]string(nil))})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if string(payload) != `{"items":[]}` {
		t.Fatalf("expected empty JSON array, got %s", payload)
	}
}

func TestJSONSlicePreservesValues(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"items": jsonSlice([]string{"one"})})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if string(payload) != `{"items":["one"]}` {
		t.Fatalf("expected value JSON array, got %s", payload)
	}
}
