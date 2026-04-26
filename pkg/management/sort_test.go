package management_test

import (
	"testing"
	"time"

	"github.com/maxime/sb/pkg/management"
)

func TestSortQueuesByActiveMessagesDesc(t *testing.T) {
	queues := []management.QueueInfo{
		{Name: "a", ActiveMessages: 5},
		{Name: "b", ActiveMessages: 20},
		{Name: "c", ActiveMessages: 1},
	}
	management.SortQueues(queues, "ActiveMessages", "dsc")
	if queues[0].Name != "b" || queues[2].Name != "c" {
		t.Errorf("got %v %v %v", queues[0].Name, queues[1].Name, queues[2].Name)
	}
}

func TestSortQueuesByLastAccessAsc(t *testing.T) {
	now := time.Now()
	queues := []management.QueueInfo{
		{Name: "newer", AccessedAt: now},
		{Name: "older", AccessedAt: now.Add(-48 * time.Hour)},
	}
	management.SortQueues(queues, "LastAccessDate", "asc")
	if queues[0].Name != "older" {
		t.Errorf("first = %q, want older", queues[0].Name)
	}
}

func TestSortTopicsBySizeDesc(t *testing.T) {
	topics := []management.TopicInfo{
		{Name: "small", SizeInMegabytes: 10},
		{Name: "big", SizeInMegabytes: 500},
	}
	management.SortTopics(topics, "SizeInMegabytes", "dsc")
	if topics[0].Name != "big" {
		t.Errorf("first = %q, want big", topics[0].Name)
	}
}
