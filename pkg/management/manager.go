package management

import "context"

type Manager interface {
	ListQueues(ctx context.Context, opts ListOptions) ([]QueueInfo, error)
	GetQueue(ctx context.Context, name string) (*QueueInfo, error)
	CreateQueue(ctx context.Context, name string, opts CreateQueueOptions) error
	DeleteQueue(ctx context.Context, name string) error

	ListTopics(ctx context.Context, opts ListOptions) ([]TopicInfo, error)
	GetTopic(ctx context.Context, name string) (*TopicInfo, error)
	CreateTopic(ctx context.Context, name string, opts CreateTopicOptions) error
	DeleteTopic(ctx context.Context, name string) error

	ListSubscriptions(ctx context.Context, topic string, opts ListOptions) ([]SubscriptionInfo, error)
	GetSubscription(ctx context.Context, topic, name string) (*SubscriptionInfo, error)
	CreateSubscription(ctx context.Context, topic, name string, opts CreateSubscriptionOptions) error
	DeleteSubscription(ctx context.Context, topic, name string) error

	ListRules(ctx context.Context, topic, sub string) ([]RuleInfo, error)
	CreateRule(ctx context.Context, topic, sub, name, sqlFilter string) error
	DeleteRule(ctx context.Context, topic, sub, name string) error
}
