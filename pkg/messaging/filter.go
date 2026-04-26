package messaging

import (
	"strings"

	"github.com/maxime/sb/pkg/management"
)

// ApplyMessageFilters filters messages using the same Filter type as management.
// All filters must match (AND).
func ApplyMessageFilters(msgs []Message, filters []management.Filter) []Message {
	if len(filters) == 0 {
		return msgs
	}
	var result []Message
	for _, m := range msgs {
		if msgMatchesAll(m, filters) {
			result = append(result, m)
		}
	}
	return result
}

func msgMatchesAll(m Message, filters []management.Filter) bool {
	for _, f := range filters {
		if !msgMatchesFilter(m, f) {
			return false
		}
	}
	return true
}

func msgMatchesFilter(m Message, f management.Filter) bool {
	switch f.Kind {
	case management.FilterKindKeyword:
		return strings.Contains(strings.ToLower(m.Body), f.Keyword) ||
			strings.Contains(strings.ToLower(m.DeadLetterReason), f.Keyword) ||
			strings.Contains(strings.ToLower(m.DeadLetterErrorDescription), f.Keyword)
	case management.FilterKindSequenceRange:
		return m.SequenceNumber >= f.SeqMin && m.SequenceNumber <= f.SeqMax
	case management.FilterKindDLQDate:
		return !m.EnqueuedAt.Before(f.MinDate) && m.EnqueuedAt.Before(f.MaxDate)
	case management.FilterKindDateField:
		return !m.EnqueuedAt.Before(f.MinDate) && m.EnqueuedAt.Before(f.MaxDate)
	case management.FilterKindErrorField:
		switch f.Field {
		case "ErrorType":
			return strings.EqualFold(m.DeadLetterReason, f.Keyword)
		case "ErrorMessage":
			return strings.Contains(strings.ToLower(m.DeadLetterErrorDescription), strings.ToLower(f.Keyword))
		}
	}
	return true
}
