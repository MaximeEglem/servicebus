package management_test

import (
	"testing"

	"servicebus/pkg/management"
)

func TestCompareQueuesFindsOnlyInOne(t *testing.T) {
	contexts := map[string][]management.QueueInfo{
		"dev":  {{Name: "orders"}, {Name: "emails"}},
		"prod": {{Name: "orders"}},
	}
	results := management.CompareQueues(contexts)
	if len(results) == 0 {
		t.Fatal("expected at least one compare result")
	}
	var found bool
	for _, r := range results {
		if r.Name == "emails" && r.PresentIn["dev"] && !r.PresentIn["prod"] {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'emails' missing in prod; results: %+v", results)
	}
}

func TestCompareQueuesFindsDivergentCounts(t *testing.T) {
	contexts := map[string][]management.QueueInfo{
		"dev":  {{Name: "orders", ActiveMessages: 0}},
		"prod": {{Name: "orders", ActiveMessages: 500}},
	}
	results := management.CompareQueues(contexts)
	var found bool
	for _, r := range results {
		if r.Name == "orders" && r.HasIssue {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'orders' to have issue; results: %+v", results)
	}
}
