package management

import (
	"context"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"servicebus/pkg/auth"
)

type azureManager struct {
	client *admin.Client
}

// NewManager creates a Manager backed by Azure Service Bus.
func NewManager(cred *auth.Credentials) (Manager, error) {
	var client *admin.Client
	var err error
	if cred.ConnectionString != "" {
		client, err = admin.NewClientFromConnectionString(cred.ConnectionString, nil)
	} else {
		client, err = admin.NewClient(cred.Namespace, cred.Token, nil)
	}
	if err != nil {
		return nil, err
	}
	return &azureManager{client: client}, nil
}

func (m *azureManager) ListQueues(ctx context.Context, opts ListOptions) ([]QueueInfo, error) {
	// Fetch runtime always; props only when caller needs static fields.
	var (
		propMap map[string]*admin.QueueProperties
		rtMap   map[string]*admin.QueueRuntimeProperties
		propErr error
		rtErr   error
		wg      sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		rtMap = map[string]*admin.QueueRuntimeProperties{}
		rtPager := m.client.NewListQueuesRuntimePropertiesPager(nil)
		for rtPager.More() {
			page, err := rtPager.NextPage(ctx)
			if err != nil {
				rtErr = err
				return
			}
			for _, q := range page.QueueRuntimeProperties {
				name := q.QueueName
				rp := q.QueueRuntimeProperties
				rtMap[name] = &rp
			}
		}
	}()

	if !opts.RuntimeOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			propMap = map[string]*admin.QueueProperties{}
			pager := m.client.NewListQueuesPager(nil)
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					propErr = err
					return
				}
				for _, q := range page.Queues {
					name := q.QueueName
					props := q.QueueProperties
					propMap[name] = &props
				}
			}
		}()
	}
	wg.Wait()
	if rtErr != nil {
		return nil, rtErr
	}
	if propErr != nil {
		return nil, propErr
	}

	// When runtime-only, iterate runtime map (it has every queue).
	names := propMap
	if opts.RuntimeOnly {
		names = make(map[string]*admin.QueueProperties, len(rtMap))
		for n := range rtMap {
			names[n] = nil
		}
	}

	result := make([]QueueInfo, 0, len(names))
	for name, props := range names {
		result = append(result, queueInfoFromProps(name, props, rtMap[name]))
	}
	return result, nil
}

func (m *azureManager) GetQueue(ctx context.Context, name string) (*QueueInfo, error) {
	resp, err := m.client.GetQueue(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	rt, err := m.client.GetQueueRuntimeProperties(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	var rp *admin.QueueRuntimeProperties
	if rt != nil {
		rp = &rt.QueueRuntimeProperties
	}
	info := queueInfoFromProps(name, &resp.QueueProperties, rp)
	return &info, nil
}

func (m *azureManager) CreateQueue(ctx context.Context, name string, opts CreateQueueOptions) error {
	props := &admin.QueueProperties{}
	if opts.MaxSizeInMegabytes > 0 {
		v := int32(opts.MaxSizeInMegabytes)
		props.MaxSizeInMegabytes = &v
	}
	if opts.MaxDeliveryCount > 0 {
		props.MaxDeliveryCount = &opts.MaxDeliveryCount
	}
	_, err := m.client.CreateQueue(ctx, name, &admin.CreateQueueOptions{Properties: props})
	return err
}

func (m *azureManager) DeleteQueue(ctx context.Context, name string) error {
	_, err := m.client.DeleteQueue(ctx, name, nil)
	return err
}

func queueInfoFromProps(name string, props *admin.QueueProperties, rt *admin.QueueRuntimeProperties) QueueInfo {
	info := QueueInfo{Name: name}
	if props != nil {
		if props.MaxSizeInMegabytes != nil {
			info.MaxSizeInMegabytes = int64(*props.MaxSizeInMegabytes)
		}
		if props.MaxDeliveryCount != nil {
			info.MaxDeliveryCount = *props.MaxDeliveryCount
		}
	}
	if rt != nil {
		info.ActiveMessages = int64(rt.ActiveMessageCount)
		info.DeadLetterMessages = int64(rt.DeadLetterMessageCount)
		info.SizeInMegabytes = rt.SizeInBytes / (1024 * 1024)
		info.CreatedAt = rt.CreatedAt
		info.UpdatedAt = rt.UpdatedAt
		info.AccessedAt = rt.AccessedAt
		if info.MaxSizeInMegabytes > 0 {
			info.SizePercentage = float64(info.SizeInMegabytes) / float64(info.MaxSizeInMegabytes) * 100
		}
	}
	return info
}

func (m *azureManager) ListTopics(ctx context.Context, opts ListOptions) ([]TopicInfo, error) {
	var (
		propMap map[string]*admin.TopicProperties
		rtMap   map[string]*admin.TopicRuntimeProperties
		propErr error
		rtErr   error
		wg      sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		rtMap = map[string]*admin.TopicRuntimeProperties{}
		rtPager := m.client.NewListTopicsRuntimePropertiesPager(nil)
		for rtPager.More() {
			page, err := rtPager.NextPage(ctx)
			if err != nil {
				rtErr = err
				return
			}
			for _, t := range page.TopicRuntimeProperties {
				name := t.TopicName
				rp := t.TopicRuntimeProperties
				rtMap[name] = &rp
			}
		}
	}()

	if !opts.RuntimeOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			propMap = map[string]*admin.TopicProperties{}
			pager := m.client.NewListTopicsPager(nil)
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					propErr = err
					return
				}
				for _, t := range page.Topics {
					name := t.TopicName
					props := t.TopicProperties
					propMap[name] = &props
				}
			}
		}()
	}
	wg.Wait()
	if rtErr != nil {
		return nil, rtErr
	}
	if propErr != nil {
		return nil, propErr
	}

	names := propMap
	if opts.RuntimeOnly {
		names = make(map[string]*admin.TopicProperties, len(rtMap))
		for n := range rtMap {
			names[n] = nil
		}
	}

	result := make([]TopicInfo, 0, len(names))
	for name, props := range names {
		result = append(result, topicInfoFromProps(name, props, rtMap[name]))
	}
	return result, nil
}

func (m *azureManager) GetTopic(ctx context.Context, name string) (*TopicInfo, error) {
	resp, err := m.client.GetTopic(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	rt, err := m.client.GetTopicRuntimeProperties(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	var rp *admin.TopicRuntimeProperties
	if rt != nil {
		rp = &rt.TopicRuntimeProperties
	}
	info := topicInfoFromProps(name, &resp.TopicProperties, rp)
	return &info, nil
}

func (m *azureManager) CreateTopic(ctx context.Context, name string, opts CreateTopicOptions) error {
	props := &admin.TopicProperties{}
	if opts.MaxSizeInMegabytes > 0 {
		v := int32(opts.MaxSizeInMegabytes)
		props.MaxSizeInMegabytes = &v
	}
	_, err := m.client.CreateTopic(ctx, name, &admin.CreateTopicOptions{Properties: props})
	return err
}

func (m *azureManager) DeleteTopic(ctx context.Context, name string) error {
	_, err := m.client.DeleteTopic(ctx, name, nil)
	return err
}

func topicInfoFromProps(name string, props *admin.TopicProperties, rt *admin.TopicRuntimeProperties) TopicInfo {
	info := TopicInfo{Name: name}
	if props != nil && props.MaxSizeInMegabytes != nil {
		info.MaxSizeInMegabytes = int64(*props.MaxSizeInMegabytes)
	}
	if rt != nil {
		info.SizeInMegabytes = rt.SizeInBytes / (1024 * 1024)
		info.SubscriptionCount = rt.SubscriptionCount
		info.CreatedAt = rt.CreatedAt
		info.UpdatedAt = rt.UpdatedAt
		info.AccessedAt = rt.AccessedAt
		if info.MaxSizeInMegabytes > 0 {
			info.SizePercentage = float64(info.SizeInMegabytes) / float64(info.MaxSizeInMegabytes) * 100
		}
	}
	return info
}

func (m *azureManager) ListSubscriptions(ctx context.Context, topic string, opts ListOptions) ([]SubscriptionInfo, error) {
	var result []SubscriptionInfo
	pager := m.client.NewListSubscriptionsRuntimePropertiesPager(topic, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.SubscriptionRuntimeProperties {
			info := SubscriptionInfo{
				TopicName:          topic,
				Name:               s.SubscriptionName,
				ActiveMessages:     int64(s.ActiveMessageCount),
				DeadLetterMessages: int64(s.DeadLetterMessageCount),
				CreatedAt:          s.CreatedAt,
				UpdatedAt:          s.UpdatedAt,
				AccessedAt:         s.AccessedAt,
			}
			result = append(result, info)
		}
	}
	return result, nil
}

func (m *azureManager) GetSubscription(ctx context.Context, topic, name string) (*SubscriptionInfo, error) {
	rt, err := m.client.GetSubscriptionRuntimeProperties(ctx, topic, name, nil)
	if err != nil {
		return nil, err
	}
	info := SubscriptionInfo{
		TopicName:          topic,
		Name:               name,
		ActiveMessages:     int64(rt.ActiveMessageCount),
		DeadLetterMessages: int64(rt.DeadLetterMessageCount),
		CreatedAt:          rt.CreatedAt,
		UpdatedAt:          rt.UpdatedAt,
		AccessedAt:         rt.AccessedAt,
	}
	return &info, nil
}

func (m *azureManager) CreateSubscription(ctx context.Context, topic, name string, opts CreateSubscriptionOptions) error {
	props := &admin.SubscriptionProperties{}
	if opts.MaxDeliveryCount > 0 {
		props.MaxDeliveryCount = &opts.MaxDeliveryCount
	}
	_, err := m.client.CreateSubscription(ctx, topic, name, &admin.CreateSubscriptionOptions{Properties: props})
	return err
}

func (m *azureManager) DeleteSubscription(ctx context.Context, topic, name string) error {
	_, err := m.client.DeleteSubscription(ctx, topic, name, nil)
	return err
}

func (m *azureManager) ListRules(ctx context.Context, topic, sub string) ([]RuleInfo, error) {
	var result []RuleInfo
	pager := m.client.NewListRulesPager(topic, sub, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Rules {
			info := RuleInfo{Name: r.Name}
			if f, ok := r.Filter.(*admin.SQLFilter); ok {
				info.Filter = f.Expression
			}
			result = append(result, info)
		}
	}
	return result, nil
}

func (m *azureManager) CreateRule(ctx context.Context, topic, sub, name, sqlFilter string) error {
	_, err := m.client.CreateRule(ctx, topic, sub, &admin.CreateRuleOptions{
		Name:   &name,
		Filter: &admin.SQLFilter{Expression: sqlFilter},
	})
	return err
}

func (m *azureManager) DeleteRule(ctx context.Context, topic, sub, name string) error {
	_, err := m.client.DeleteRule(ctx, topic, sub, name, nil)
	return err
}
