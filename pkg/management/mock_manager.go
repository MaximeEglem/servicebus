package management

import "context"

type MockManager struct {
	Queues        []QueueInfo
	Topics        []TopicInfo
	Subscriptions []SubscriptionInfo
	Rules         []RuleInfo
	Err           error
}

func (m *MockManager) ListQueues(_ context.Context, _ ListOptions) ([]QueueInfo, error) {
	return m.Queues, m.Err
}
func (m *MockManager) GetQueue(_ context.Context, name string) (*QueueInfo, error) {
	for _, q := range m.Queues {
		if q.Name == name {
			return &q, m.Err
		}
	}
	return nil, ErrNotFound(name)
}
func (m *MockManager) CreateQueue(_ context.Context, _ string, _ CreateQueueOptions) error {
	return m.Err
}
func (m *MockManager) DeleteQueue(_ context.Context, _ string) error { return m.Err }

func (m *MockManager) ListTopics(_ context.Context, _ ListOptions) ([]TopicInfo, error) {
	return m.Topics, m.Err
}
func (m *MockManager) GetTopic(_ context.Context, name string) (*TopicInfo, error) {
	for _, t := range m.Topics {
		if t.Name == name {
			return &t, m.Err
		}
	}
	return nil, ErrNotFound(name)
}
func (m *MockManager) CreateTopic(_ context.Context, _ string, _ CreateTopicOptions) error {
	return m.Err
}
func (m *MockManager) DeleteTopic(_ context.Context, _ string) error { return m.Err }

func (m *MockManager) ListSubscriptions(_ context.Context, _ string, _ ListOptions) ([]SubscriptionInfo, error) {
	return m.Subscriptions, m.Err
}
func (m *MockManager) GetSubscription(_ context.Context, _, name string) (*SubscriptionInfo, error) {
	for _, s := range m.Subscriptions {
		if s.Name == name {
			return &s, m.Err
		}
	}
	return nil, ErrNotFound(name)
}
func (m *MockManager) CreateSubscription(_ context.Context, _, _ string, _ CreateSubscriptionOptions) error {
	return m.Err
}
func (m *MockManager) DeleteSubscription(_ context.Context, _, _ string) error { return m.Err }

func (m *MockManager) ListRules(_ context.Context, _, _ string) ([]RuleInfo, error) {
	return m.Rules, m.Err
}
func (m *MockManager) CreateRule(_ context.Context, _, _, _, _ string) error { return m.Err }
func (m *MockManager) DeleteRule(_ context.Context, _, _, _ string) error    { return m.Err }

type NotFoundError struct{ Name string }

func (e NotFoundError) Error() string { return "not found: " + e.Name }
func ErrNotFound(name string) error   { return NotFoundError{Name: name} }
