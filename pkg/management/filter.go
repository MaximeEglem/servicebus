package management

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reDateRange   = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4})-(\d{2}/\d{2}/\d{4})$`)
	reSingleDate  = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)
	reNumRange    = regexp.MustCompile(`^(\d+)-(\d+)$`)
	reSingleNum   = regexp.MustCompile(`^\d+$`)
	dateLayout    = "02/01/2006"
	errorFields   = map[string]bool{"ErrorType": true, "ErrorMessage": true, "ErrorDate": true}
	numericFields = map[string]bool{
		"ActiveMessages": true, "DeadLetterMessages": true, "SizeInMegabytes": true,
	}
	dateFields = map[string]bool{"CreationDate": true, "LastAccessDate": true, "ErrorDate": true}
)

// ParseFilter parses a single -f value into a Filter.
func ParseFilter(s string) (Filter, error) {
	if idx := strings.Index(s, ":"); idx > 0 {
		field := s[:idx]
		value := s[idx+1:]
		return parseFieldFilter(field, value)
	}
	if reSingleDate.MatchString(s) {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			return Filter{}, err
		}
		return Filter{Kind: FilterKindDLQDate, MinDate: d, MaxDate: d.Add(24 * time.Hour)}, nil
	}
	if m := reNumRange.FindStringSubmatch(s); m != nil {
		min, _ := strconv.ParseInt(m[1], 10, 64)
		max, _ := strconv.ParseInt(m[2], 10, 64)
		return Filter{Kind: FilterKindSequenceRange, SeqMin: min, SeqMax: max}, nil
	}
	if reSingleNum.MatchString(s) {
		n, _ := strconv.ParseInt(s, 10, 64)
		return Filter{Kind: FilterKindSequenceRange, SeqMin: n, SeqMax: n}, nil
	}
	return Filter{Kind: FilterKindKeyword, Keyword: strings.ToLower(s)}, nil
}

func parseFieldFilter(field, value string) (Filter, error) {
	if errorFields[field] {
		if field == "ErrorDate" {
			return parseDateRangeFilter(field, value)
		}
		return Filter{Kind: FilterKindErrorField, Field: field, Keyword: value}, nil
	}
	if numericFields[field] {
		return parseNumericFilter(field, value)
	}
	if dateFields[field] {
		return parseDateRangeFilter(field, value)
	}
	return Filter{Kind: FilterKindKeyword, Keyword: strings.ToLower(field + ":" + value)}, nil
}

func parseNumericFilter(field, value string) (Filter, error) {
	if m := reNumRange.FindStringSubmatch(value); m != nil {
		min, _ := strconv.ParseFloat(m[1], 64)
		max, _ := strconv.ParseFloat(m[2], 64)
		return Filter{Kind: FilterKindNumericField, Field: field, MinValue: min, MaxValue: max}, nil
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return Filter{}, fmt.Errorf("invalid numeric filter %q: %w", value, err)
	}
	return Filter{Kind: FilterKindNumericField, Field: field, MinValue: n, MaxValue: n}, nil
}

func parseDateRangeFilter(field, value string) (Filter, error) {
	if m := reDateRange.FindStringSubmatch(value); m != nil {
		min, err := time.Parse(dateLayout, m[1])
		if err != nil {
			return Filter{}, err
		}
		max, err := time.Parse(dateLayout, m[2])
		if err != nil {
			return Filter{}, err
		}
		return Filter{Kind: FilterKindDateField, Field: field, MinDate: min, MaxDate: max.Add(24 * time.Hour)}, nil
	}
	d, err := time.Parse(dateLayout, value)
	if err != nil {
		return Filter{}, fmt.Errorf("invalid date %q: expected dd/mm/yyyy", value)
	}
	return Filter{Kind: FilterKindDateField, Field: field, MinDate: d, MaxDate: d.Add(24 * time.Hour)}, nil
}

// ApplyQueueFilters returns queues matching ALL provided filters (AND logic).
func ApplyQueueFilters(queues []QueueInfo, filters []Filter) []QueueInfo {
	if len(filters) == 0 {
		return queues
	}
	var result []QueueInfo
	for _, q := range queues {
		if queueMatchesAll(q, filters) {
			result = append(result, q)
		}
	}
	return result
}

func queueMatchesAll(q QueueInfo, filters []Filter) bool {
	for _, f := range filters {
		if !queueMatchesFilter(q, f) {
			return false
		}
	}
	return true
}

func queueMatchesFilter(q QueueInfo, f Filter) bool {
	switch f.Kind {
	case FilterKindKeyword:
		return strings.Contains(strings.ToLower(q.Name), f.Keyword)
	case FilterKindNumericField:
		val := queueNumericField(q, f.Field)
		return val >= f.MinValue && val <= f.MaxValue
	case FilterKindDateField:
		d := queueDateField(q, f.Field)
		return !d.Before(f.MinDate) && d.Before(f.MaxDate)
	}
	return true
}

func queueNumericField(q QueueInfo, field string) float64 {
	switch field {
	case "ActiveMessages":
		return float64(q.ActiveMessages)
	case "DeadLetterMessages":
		return float64(q.DeadLetterMessages)
	case "SizeInMegabytes":
		return float64(q.SizeInMegabytes)
	}
	return 0
}

func queueDateField(q QueueInfo, field string) time.Time {
	switch field {
	case "CreationDate":
		return q.CreatedAt
	case "LastAccessDate":
		return q.AccessedAt
	}
	return time.Time{}
}

// ApplyTopicFilters mirrors ApplyQueueFilters for topics.
func ApplyTopicFilters(topics []TopicInfo, filters []Filter) []TopicInfo {
	if len(filters) == 0 {
		return topics
	}
	var result []TopicInfo
	for _, t := range topics {
		if topicMatchesAll(t, filters) {
			result = append(result, t)
		}
	}
	return result
}

func topicMatchesAll(t TopicInfo, filters []Filter) bool {
	for _, f := range filters {
		if !topicMatchesFilter(t, f) {
			return false
		}
	}
	return true
}

func topicMatchesFilter(t TopicInfo, f Filter) bool {
	switch f.Kind {
	case FilterKindKeyword:
		return strings.Contains(strings.ToLower(t.Name), f.Keyword)
	case FilterKindNumericField:
		switch f.Field {
		case "SizeInMegabytes":
			return float64(t.SizeInMegabytes) >= f.MinValue && float64(t.SizeInMegabytes) <= f.MaxValue
		}
	case FilterKindDateField:
		d := topicDateField(t, f.Field)
		return !d.Before(f.MinDate) && d.Before(f.MaxDate)
	}
	return true
}

func topicDateField(t TopicInfo, field string) time.Time {
	switch field {
	case "CreationDate":
		return t.CreatedAt
	case "LastAccessDate":
		return t.AccessedAt
	}
	return time.Time{}
}

// ApplySubscriptionFilters mirrors ApplyQueueFilters for subscriptions.
func ApplySubscriptionFilters(subs []SubscriptionInfo, filters []Filter) []SubscriptionInfo {
	if len(filters) == 0 {
		return subs
	}
	var result []SubscriptionInfo
	for _, s := range subs {
		if subMatchesAll(s, filters) {
			result = append(result, s)
		}
	}
	return result
}

func subMatchesAll(s SubscriptionInfo, filters []Filter) bool {
	for _, f := range filters {
		if !subMatchesFilter(s, f) {
			return false
		}
	}
	return true
}

func subMatchesFilter(s SubscriptionInfo, f Filter) bool {
	switch f.Kind {
	case FilterKindKeyword:
		return strings.Contains(strings.ToLower(s.Name), f.Keyword)
	case FilterKindNumericField:
		switch f.Field {
		case "ActiveMessages":
			return float64(s.ActiveMessages) >= f.MinValue && float64(s.ActiveMessages) <= f.MaxValue
		case "DeadLetterMessages":
			return float64(s.DeadLetterMessages) >= f.MinValue && float64(s.DeadLetterMessages) <= f.MaxValue
		}
	case FilterKindDateField:
		d := subDateField(s, f.Field)
		return !d.Before(f.MinDate) && d.Before(f.MaxDate)
	}
	return true
}

func subDateField(s SubscriptionInfo, field string) time.Time {
	switch field {
	case "CreationDate":
		return s.CreatedAt
	case "LastAccessDate":
		return s.AccessedAt
	}
	return time.Time{}
}
