package messaging

import "time"

type Message struct {
	MessageID                  string                 `json:"MessageID"  yaml:"message-id"`
	SequenceNumber             int64                  `json:"SequenceNumber"  yaml:"sequence-number"`
	Body                       string                 `json:"Body"  yaml:"body"`
	Properties                 map[string]interface{} `json:"Properties,omitempty"  yaml:"properties,omitempty"`
	EnqueuedAt                 time.Time              `json:"EnqueuedAt"  yaml:"enqueued-at"`
	DeadLetterReason           string                 `json:"DeadLetterReason,omitempty"  yaml:"dead-letter-reason,omitempty"`
	DeadLetterErrorDescription string                 `json:"DeadLetterErrorDescription,omitempty"  yaml:"dead-letter-error-description,omitempty"`
}

type SendOptions struct {
	Body       []byte
	Properties map[string]interface{}
	DelayMins  int
	Count      int // 0 or 1 = send once; >1 = repeat
}

type DLOptions struct {
	Max       int // 0 = all
	DelayMins int // for replay
	TTLDays   int // for replay
}
