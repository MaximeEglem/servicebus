package management_test

import (
	"testing"
	"time"

	"servicebus/pkg/management"
)

func TestParseFilterKeyword(t *testing.T) {
	f, err := management.ParseFilter("order")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindKeyword || f.Keyword != "order" {
		t.Errorf("got %+v", f)
	}
}

func TestParseFilterNumericField(t *testing.T) {
	f, err := management.ParseFilter("ActiveMessages:1-10")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindNumericField || f.Field != "ActiveMessages" {
		t.Errorf("got %+v", f)
	}
	if f.MinValue != 1 || f.MaxValue != 10 {
		t.Errorf("range = %v-%v, want 1-10", f.MinValue, f.MaxValue)
	}
}

func TestParseFilterDateField(t *testing.T) {
	f, err := management.ParseFilter("CreationDate:01/05/2024-01/05/2025")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindDateField || f.Field != "CreationDate" {
		t.Errorf("got %+v", f)
	}
	if f.MinDate.Year() != 2024 || f.MaxDate.Year() != 2025 {
		t.Errorf("date range %v-%v", f.MinDate, f.MaxDate)
	}
}

func TestParseFilterErrorField(t *testing.T) {
	f, err := management.ParseFilter("ErrorType:NullRefException")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindErrorField || f.Field != "ErrorType" || f.Keyword != "NullRefException" {
		t.Errorf("got %+v", f)
	}
}

func TestParseFilterSequenceRange(t *testing.T) {
	f, err := management.ParseFilter("100-200")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindSequenceRange || f.SeqMin != 100 || f.SeqMax != 200 {
		t.Errorf("got %+v", f)
	}
}

func TestParseFilterSingleSequence(t *testing.T) {
	f, err := management.ParseFilter("200")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindSequenceRange || f.SeqMin != 200 || f.SeqMax != 200 {
		t.Errorf("got %+v", f)
	}
}

func TestParseFilterDLQDate(t *testing.T) {
	f, err := management.ParseFilter("01/04/2025")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != management.FilterKindDLQDate {
		t.Errorf("got kind %v, want DLQDate", f.Kind)
	}
	if f.MinDate.Year() != 2025 {
		t.Errorf("date = %v", f.MinDate)
	}
}

func TestApplyQueueFiltersKeyword(t *testing.T) {
	queues := []management.QueueInfo{
		{Name: "order-service"},
		{Name: "email-sender"},
		{Name: "order-notifications"},
	}
	f, _ := management.ParseFilter("order")
	result := management.ApplyQueueFilters(queues, []management.Filter{f})
	if len(result) != 2 {
		t.Errorf("got %d results, want 2", len(result))
	}
}

func TestApplyQueueFiltersNumeric(t *testing.T) {
	queues := []management.QueueInfo{
		{Name: "q1", ActiveMessages: 5},
		{Name: "q2", ActiveMessages: 15},
		{Name: "q3", ActiveMessages: 0},
	}
	f, _ := management.ParseFilter("ActiveMessages:1-10")
	result := management.ApplyQueueFilters(queues, []management.Filter{f})
	if len(result) != 1 || result[0].Name != "q1" {
		t.Errorf("got %+v", result)
	}
}

func TestApplyQueueFiltersDate(t *testing.T) {
	now := time.Now()
	queues := []management.QueueInfo{
		{Name: "recent", AccessedAt: now},
		{Name: "old", AccessedAt: now.AddDate(-5, 0, 0)},
	}
	// Filter to last 2 years: only "recent" should match
	from := now.AddDate(-2, 0, 0)
	to := now.AddDate(1, 0, 0)
	fromStr := from.Format("02/01/2006")
	toStr := to.Format("02/01/2006")
	f, err := management.ParseFilter("LastAccessDate:" + fromStr + "-" + toStr)
	if err != nil {
		t.Fatal(err)
	}
	result := management.ApplyQueueFilters(queues, []management.Filter{f})
	if len(result) != 1 || result[0].Name != "recent" {
		t.Errorf("got %v, want only [recent]", result)
	}
}

func TestParseFilterInvalidNumeric(t *testing.T) {
	_, err := management.ParseFilter("ActiveMessages:notanumber")
	if err == nil {
		t.Error("expected error for non-numeric value")
	}
}

func TestParseFilterInvalidDate(t *testing.T) {
	_, err := management.ParseFilter("CreationDate:baddate")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}
