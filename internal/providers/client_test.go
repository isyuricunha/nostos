package providers

import "testing"

func TestParseStreamChunkContentToolAndUsage(t *testing.T) {
	events := ParseStreamChunk([]byte(`{
		"choices": [{
			"delta": {
				"content": "Hello",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "lookup", "arguments": "{\"q\":\"nostos\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
	}`))

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d: %#v", len(events), events)
	}
	if events[0].Type != "usage" || events[0].Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage event: %#v", events[0])
	}
	if events[1].Type != "content_delta" || events[1].Content != "Hello" {
		t.Fatalf("unexpected content event: %#v", events[1])
	}
	if events[2].Type != "tool_call_delta" || events[2].ToolCall.Function.Name != "lookup" {
		t.Fatalf("unexpected tool event: %#v", events[2])
	}
	if events[3].Type != "tool_call_ready" {
		t.Fatalf("unexpected final event: %#v", events[3])
	}
}
