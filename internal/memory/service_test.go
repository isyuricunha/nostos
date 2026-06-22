package memory

import (
	"testing"

	"github.com/isyuricunha/nostos/internal/chat"
)

func TestRankMemoriesPrefersPinnedAndRelevant(t *testing.T) {
	memories := []Memory{
		{ID: "a", Title: "Generic", Content: "Unrelated content", Scope: "global", Importance: 50, Active: true},
		{ID: "b", Title: "Bifrost provider", Content: "Use Bifrost for OpenAI-compatible routing.", Tags: []string{"bifrost"}, Scope: "global", Importance: 80, Pinned: true, Active: true},
		{ID: "c", Title: "Old", Content: "Another provider", Scope: "global", Importance: 20, Active: true},
	}
	ranked := RankMemories(memories, chat.MemoryRequest{AccessMode: "relevant", Query: "Which bifrost provider should I use?"})
	if len(ranked) == 0 {
		t.Fatal("expected ranked memories")
	}
	if ranked[0].ID != "b" {
		t.Fatalf("expected pinned relevant memory first, got %#v", ranked)
	}
}

func TestPinnedOnlyModeFiltersUnpinned(t *testing.T) {
	memories := []Memory{
		{ID: "a", Title: "Relevant", Content: "bifrost", Scope: "global", Importance: 100, Active: true},
		{ID: "b", Title: "Pinned", Content: "short", Scope: "global", Importance: 10, Pinned: true, Active: true},
	}
	ranked := RankMemories(memories, chat.MemoryRequest{AccessMode: "pinned_only", Query: "bifrost"})
	if len(ranked) != 1 || ranked[0].ID != "b" {
		t.Fatalf("expected only pinned memory, got %#v", ranked)
	}
}
