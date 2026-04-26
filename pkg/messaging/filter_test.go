package messaging_test

import (
	"testing"
	"time"

	"servicebus/pkg/management"
	"servicebus/pkg/messaging"
)

func TestApplyMessageFiltersKeyword(t *testing.T) {
	msgs := []messaging.Message{
		{Body: "order placed", SequenceNumber: 1},
		{Body: "email sent", SequenceNumber: 2},
	}
	f, _ := management.ParseFilter("order")
	result := messaging.ApplyMessageFilters(msgs, []management.Filter{f})
	if len(result) != 1 || result[0].SequenceNumber != 1 {
		t.Errorf("got %+v", result)
	}
}

func TestApplyMessageFiltersSequenceRange(t *testing.T) {
	msgs := []messaging.Message{
		{SequenceNumber: 50},
		{SequenceNumber: 150},
		{SequenceNumber: 250},
	}
	f, _ := management.ParseFilter("100-200")
	result := messaging.ApplyMessageFilters(msgs, []management.Filter{f})
	if len(result) != 1 || result[0].SequenceNumber != 150 {
		t.Errorf("got %+v", result)
	}
}

func TestApplyMessageFiltersDate(t *testing.T) {
	msgs := []messaging.Message{
		{SequenceNumber: 1, EnqueuedAt: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)},
		{SequenceNumber: 2, EnqueuedAt: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	f, _ := management.ParseFilter("01/04/2025")
	result := messaging.ApplyMessageFilters(msgs, []management.Filter{f})
	if len(result) != 1 || result[0].SequenceNumber != 1 {
		t.Errorf("got %+v", result)
	}
}

func TestApplyMessageFiltersErrorType(t *testing.T) {
	msgs := []messaging.Message{
		{SequenceNumber: 1, DeadLetterReason: "NullReferenceException"},
		{SequenceNumber: 2, DeadLetterReason: "TimeoutException"},
	}
	f, _ := management.ParseFilter("ErrorType:NullReferenceException")
	result := messaging.ApplyMessageFilters(msgs, []management.Filter{f})
	if len(result) != 1 || result[0].SequenceNumber != 1 {
		t.Errorf("got %+v", result)
	}
}
