package messaging

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
)

// GroupKey selects how dead-letter messages are grouped for statistics.
type GroupKey string

const (
	GroupByReason      GroupKey = "reason"      // DeadLetterReason (a.k.a. ErrorType)
	GroupByDescription GroupKey = "description" // DeadLetterErrorDescription
	GroupByBodyHash    GroupKey = "body-hash"   // SHA-1 of the message body
)

// ParseGroupKey accepts the user-supplied --by value and normalises it.
func ParseGroupKey(s string) (GroupKey, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "reason", "errortype", "error-type":
		return GroupByReason, true
	case "description", "desc", "errordescription":
		return GroupByDescription, true
	case "body", "body-hash", "hash":
		return GroupByBodyHash, true
	}
	return "", false
}

// DLQStat is a single row in the grouped statistics output.
type DLQStat struct {
	Key       string  `json:"Key"        yaml:"key"`
	Count     int     `json:"Count"      yaml:"count"`
	Pct       float64 `json:"Pct"        yaml:"pct"`
	FirstSeq  int64   `json:"FirstSeq"   yaml:"first-seq"`
	LastSeq   int64   `json:"LastSeq"    yaml:"last-seq"`
	SampleMsg string  `json:"SampleMsg,omitempty" yaml:"sample-msg,omitempty"`
}

// TableColumns implements format.TableColumner so the table renderer
// shows a useful default ordering with the long sample column last.
func (DLQStat) TableColumns() []string {
	return []string{"Key", "Count", "Pct", "FirstSeq", "LastSeq", "SampleMsg"}
}

// GroupMessages aggregates msgs by the requested key and returns the result
// sorted by Count desc (stable secondary sort by Key for determinism).
// When top > 0, only the first top rows are returned.
func GroupMessages(msgs []Message, key GroupKey, top int) []DLQStat {
	if len(msgs) == 0 {
		return nil
	}
	type acc struct {
		count    int
		firstSeq int64
		lastSeq  int64
		sample   string
	}
	buckets := make(map[string]*acc)
	total := 0
	for _, m := range msgs {
		k := bucketKey(m, key)
		b, ok := buckets[k]
		if !ok {
			b = &acc{firstSeq: m.SequenceNumber, lastSeq: m.SequenceNumber, sample: sampleFor(m, key)}
			buckets[k] = b
		}
		b.count++
		if m.SequenceNumber < b.firstSeq {
			b.firstSeq = m.SequenceNumber
		}
		if m.SequenceNumber > b.lastSeq {
			b.lastSeq = m.SequenceNumber
		}
		total++
	}

	stats := make([]DLQStat, 0, len(buckets))
	for k, b := range buckets {
		pct := 0.0
		if total > 0 {
			pct = float64(b.count) * 100.0 / float64(total)
		}
		stats = append(stats, DLQStat{
			Key:       k,
			Count:     b.count,
			Pct:       pct,
			FirstSeq:  b.firstSeq,
			LastSeq:   b.lastSeq,
			SampleMsg: b.sample,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Key < stats[j].Key
	})
	if top > 0 && len(stats) > top {
		stats = stats[:top]
	}
	return stats
}

func bucketKey(m Message, key GroupKey) string {
	switch key {
	case GroupByDescription:
		if m.DeadLetterErrorDescription == "" {
			return "(none)"
		}
		// Trim long stack-trace style descriptions to keep the bucket meaningful.
		return truncate(strings.TrimSpace(m.DeadLetterErrorDescription), 200)
	case GroupByBodyHash:
		sum := sha1.Sum([]byte(m.Body))
		return hex.EncodeToString(sum[:8]) // 16 hex chars is plenty for grouping
	default: // GroupByReason
		if m.DeadLetterReason == "" {
			return "(none)"
		}
		return m.DeadLetterReason
	}
}

func sampleFor(m Message, key GroupKey) string {
	switch key {
	case GroupByBodyHash:
		return truncate(strings.TrimSpace(m.Body), 80)
	case GroupByReason:
		return truncate(strings.TrimSpace(m.DeadLetterErrorDescription), 80)
	default:
		return ""
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
