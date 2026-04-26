package management

import "time"

type QueueInfo struct {
	Name               string    `json:"Name"   yaml:"name"`
	ActiveMessages     int64     `json:"ActiveMessages"   yaml:"active-messages"`
	DeadLetterMessages int64     `json:"DeadLetterMessages"   yaml:"dead-letter-messages"`
	SizeInMegabytes    int64     `json:"SizeInMegabytes"   yaml:"size-in-megabytes"`
	MaxSizeInMegabytes int64     `json:"MaxSizeInMegabytes"   yaml:"max-size-in-megabytes"`
	SizePercentage     float64   `json:"SizePercentage"   yaml:"size-percentage"`
	CreatedAt          time.Time `json:"CreatedAt"   yaml:"created-at"`
	UpdatedAt          time.Time `json:"UpdatedAt"   yaml:"updated-at"`
	AccessedAt         time.Time `json:"AccessedAt"   yaml:"accessed-at"`
	MaxDeliveryCount   int32     `json:"MaxDeliveryCount"   yaml:"max-delivery-count"`
	LockDuration       string    `json:"LockDuration"   yaml:"lock-duration"`
	DefaultMessageTTL  string    `json:"DefaultMessageTTL"   yaml:"default-message-ttl"`
}

type TopicInfo struct {
	Name               string    `json:"Name"   yaml:"name"`
	SizeInMegabytes    int64     `json:"SizeInMegabytes"   yaml:"size-in-megabytes"`
	MaxSizeInMegabytes int64     `json:"MaxSizeInMegabytes"   yaml:"max-size-in-megabytes"`
	SizePercentage     float64   `json:"SizePercentage"   yaml:"size-percentage"`
	SubscriptionCount  int32     `json:"SubscriptionCount"   yaml:"subscription-count"`
	CreatedAt          time.Time `json:"CreatedAt"   yaml:"created-at"`
	UpdatedAt          time.Time `json:"UpdatedAt"   yaml:"updated-at"`
	AccessedAt         time.Time `json:"AccessedAt"   yaml:"accessed-at"`
	DefaultMessageTTL  string    `json:"DefaultMessageTTL"   yaml:"default-message-ttl"`
}

type SubscriptionInfo struct {
	TopicName          string    `json:"TopicName"   yaml:"topic-name"`
	Name               string    `json:"Name"   yaml:"name"`
	ActiveMessages     int64     `json:"ActiveMessages"   yaml:"active-messages"`
	DeadLetterMessages int64     `json:"DeadLetterMessages"   yaml:"dead-letter-messages"`
	CreatedAt          time.Time `json:"CreatedAt"   yaml:"created-at"`
	UpdatedAt          time.Time `json:"UpdatedAt"   yaml:"updated-at"`
	AccessedAt         time.Time `json:"AccessedAt"   yaml:"accessed-at"`
	LockDuration       string    `json:"LockDuration"   yaml:"lock-duration"`
	DefaultMessageTTL  string    `json:"DefaultMessageTTL"   yaml:"default-message-ttl"`
	MaxDeliveryCount   int32     `json:"MaxDeliveryCount"   yaml:"max-delivery-count"`
}

type RuleInfo struct {
	Name   string `json:"Name"   yaml:"name"`
	Filter string `json:"Filter"   yaml:"filter"`
}

type FilterKind int

const (
	FilterKindKeyword       FilterKind = iota
	FilterKindNumericField
	FilterKindDateField
	FilterKindErrorField
	FilterKindDLQDate
	FilterKindSequenceRange
)

type Filter struct {
	Kind     FilterKind
	Keyword  string
	Field    string
	MinValue float64
	MaxValue float64
	MinDate  time.Time
	MaxDate  time.Time
	SeqMin   int64
	SeqMax   int64
}

type ListOptions struct {
	Filters []Filter
	SortBy  string
	SortDir string
	Max     int
	All     bool
}

type CreateQueueOptions struct {
	MaxSizeInMegabytes int64
	DefaultMessageTTL  time.Duration
	LockDuration       time.Duration
	MaxDeliveryCount   int32
}

type CreateTopicOptions struct {
	MaxSizeInMegabytes int64
	DefaultMessageTTL  time.Duration
}

type CreateSubscriptionOptions struct {
	DefaultMessageTTL time.Duration
	LockDuration      time.Duration
	MaxDeliveryCount  int32
}
