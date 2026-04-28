package messaging
package messaging

import (
	"strings"
	"testing"
)

func TestGroupMessagesByReason(t *testing.T) {
	msgs := []Message{
		{SequenceNumber: 1, DeadLetterReason: "NullRef"},
		{SequenceNumber: 2, DeadLetterReason: "Timeout"},
		{SequenceNumber: 3, DeadLetterReason: "NullRef"},
		{SequenceNumber: 4, DeadLetterReason: "NullRef"},
		{SequenceNumber: 5, DeadLetterReason: ""},
	}
	stats := GroupMessages(msgs, GroupByReason, 0)
	if len(stats) != 3 {
		t.Fatalf("want 3 buckets, got %d", len(stats))
	}
	if stats[0].Key != "NullRef" || stats[0].Count != 3 {
		t.Errorf("first bucket: got %+v", stats[0])
	}
	if stats[0].FirstSeq != 1 || stats[0].LastSeq != 4 {
		t.Errorf("seq range wrong: %+v", stats[0])
	}
	if stats[0].Pct < 59.9 || stats[0].Pct > 60.1 {
		t.Errorf("pct should be 60, got %v", stats[0].Pct)
	}
}

func TestGroupMessagesTop(t *testing.T) {
	msgs := []Message{
		{SequenceNumber: 1, DeadLetterReason: "A"},
		{SequenceNumber: 2, DeadLetterReason: "B"},
		{SequenceNumber: 3, DeadLetterReason: "A"},
		{SequenceNumber: 4, DeadLetterReason: "C"},
	}
	stats := GroupMessages(msgs, GroupByReason, 1)
	if len(stats) != 1 || stats[0].Key != "A" {
		t.Errorf("expected only top bucket A, got %+v", stats)
	}
}

func TestGroupMessagesByBodyHash(t *testing.T) {
	msgs := []Message{
		{SequenceNumber: 1, Body: "same"},
		{SequenceNumber: 2, Body: "same"},
		{SequenceNumber: 3, Body: "different"},
	}
	stats := GroupMessages(msgs, GroupByBodyHash, 0)
	if len(stats) != 2 {
		t.Fatalf("want 2 buckets, got %d", len(stats))
	}
	if stats[0].Count != 2 {
		t.Errorf("top bucket should have 2: %+v", stats[0])
	}
}

func TestParseGroupKey(t *testing.T) {
	cases := map[string]GroupKey{
		"":             GroupByReason,
		"reason":       GroupByReason,
		"ErrorType":    GroupByReason,
		"description":  GroupByDescription,
		"body":         GroupByBodyHash,
		"body-hash":    GroupByBodyHash,
	}
	for in, want := range cases {
		got, ok := ParseGroupKey(in)
		if !ok || got != want {
			t.Errorf("ParseGroupKey(%q) = %v,%v; want %v", in, got, ok, want)
		}
	}
	if _, ok := ParseGroupKey("nope"); ok {
		t.Error("nope should be invalid")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); !strings.HasSuffix(got, "…") {
		t.Errorf("truncate did not append ellipsis: %q", got)
	}
	if got := truncate("ok", 5); got != "ok" {
		t.Errorf("short string changed: %q", got)
	}
}
