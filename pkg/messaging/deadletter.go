package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/maxime/sb/pkg/auth"
	"github.com/maxime/sb/pkg/management"
)

// DLReceiver manages dead letter sub-queues.
type DLReceiver interface {
	Peek(ctx context.Context, max int) ([]Message, error)
	Purge(ctx context.Context, filters []management.Filter, max int) (int, error)
	Replay(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error)
	Export(ctx context.Context, filters []management.Filter, max int, outPath string) error
}

type azureDLReceiver struct {
	receiver *azservicebus.Receiver
	sender   *azservicebus.Sender
}

func NewDLReceiverForQueue(cred *auth.Credentials, queueName string) (DLReceiver, error) {
	client, err := newClient(cred)
	if err != nil {
		return nil, err
	}
	recv, err := client.NewReceiverForQueue(queueName, &azservicebus.ReceiverOptions{
		SubQueue: azservicebus.SubQueueDeadLetter,
	})
	if err != nil {
		return nil, err
	}
	send, err := client.NewSender(queueName, nil)
	if err != nil {
		return nil, err
	}
	return &azureDLReceiver{receiver: recv, sender: send}, nil
}

func NewDLReceiverForSubscription(cred *auth.Credentials, topic, sub string) (DLReceiver, error) {
	client, err := newClient(cred)
	if err != nil {
		return nil, err
	}
	recv, err := client.NewReceiverForSubscription(topic, sub, &azservicebus.ReceiverOptions{
		SubQueue: azservicebus.SubQueueDeadLetter,
	})
	if err != nil {
		return nil, err
	}
	send, err := client.NewSender(topic, nil)
	if err != nil {
		return nil, err
	}
	return &azureDLReceiver{receiver: recv, sender: send}, nil
}

func newClient(cred *auth.Credentials) (*azservicebus.Client, error) {
	if cred.ConnectionString != "" {
		return azservicebus.NewClientFromConnectionString(cred.ConnectionString, nil)
	}
	return azservicebus.NewClient(cred.Namespace, cred.Token, nil)
}

func (r *azureDLReceiver) Peek(ctx context.Context, max int) ([]Message, error) {
	if max <= 0 {
		max = 1000
	}
	peeked, err := r.receiver.PeekMessages(ctx, max, nil)
	if err != nil {
		return nil, err
	}
	return convertMessages(peeked), nil
}

func (r *azureDLReceiver) Purge(ctx context.Context, filters []management.Filter, max int) (int, error) {
	if max <= 0 {
		max = 1000
	}
	msgs, err := r.receiver.ReceiveMessages(ctx, max, nil)
	if err != nil {
		return 0, err
	}
	converted := convertReceivedMessages(msgs)
	matched := ApplyMessageFilters(converted, filters)
	seqSet := map[int64]bool{}
	for _, m := range matched {
		seqSet[m.SequenceNumber] = true
	}
	count := 0
	for _, m := range msgs {
		seq := int64(0)
		if m.SequenceNumber != nil {
			seq = *m.SequenceNumber
		}
		if seqSet[seq] {
			if err := r.receiver.CompleteMessage(ctx, m, nil); err != nil {
				return count, fmt.Errorf("completing seq %d: %w", seq, err)
			}
			count++
		} else {
			r.receiver.AbandonMessage(ctx, m, nil)
		}
	}
	return count, nil
}

func (r *azureDLReceiver) Replay(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error) {
	max := opts.Max
	if max <= 0 {
		max = 1000
	}
	msgs, err := r.receiver.ReceiveMessages(ctx, max, nil)
	if err != nil {
		return 0, err
	}
	converted := convertReceivedMessages(msgs)
	matched := ApplyMessageFilters(converted, filters)
	seqSet := map[int64]bool{}
	for _, m := range matched {
		seqSet[m.SequenceNumber] = true
	}

	count := 0
	for _, m := range msgs {
		seq := int64(0)
		if m.SequenceNumber != nil {
			seq = *m.SequenceNumber
		}
		if !seqSet[seq] {
			r.receiver.AbandonMessage(ctx, m, nil)
			continue
		}
		newMsg := &azservicebus.Message{
			Body:                  m.Body,
			ApplicationProperties: m.ApplicationProperties,
		}
		if opts.TTLDays > 0 {
			ttl := time.Duration(opts.TTLDays) * 24 * time.Hour
			newMsg.TimeToLive = &ttl
		}
		var sendErr error
		if opts.DelayMins > 0 {
			at := time.Now().Add(time.Duration(opts.DelayMins) * time.Minute)
			_, sendErr = r.sender.ScheduleMessages(ctx, []*azservicebus.Message{newMsg}, at, nil)
		} else {
			sendErr = r.sender.SendMessage(ctx, newMsg, nil)
		}
		if sendErr != nil {
			r.receiver.AbandonMessage(ctx, m, nil)
			return count, sendErr
		}
		if err := r.receiver.CompleteMessage(ctx, m, nil); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (r *azureDLReceiver) Export(ctx context.Context, filters []management.Filter, max int, outPath string) error {
	msgs, err := r.Peek(ctx, max)
	if err != nil {
		return err
	}
	msgs = ApplyMessageFilters(msgs, filters)
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0644)
}

func convertMessages(msgs []*azservicebus.ReceivedMessage) []Message {
	result := make([]Message, len(msgs))
	for i, m := range msgs {
		result[i] = toMessage(m)
	}
	return result
}

func convertReceivedMessages(msgs []*azservicebus.ReceivedMessage) []Message {
	return convertMessages(msgs)
}

func toMessage(m *azservicebus.ReceivedMessage) Message {
	msg := Message{
		Body:       string(m.Body),
		MessageID:  m.MessageID,
		Properties: m.ApplicationProperties,
	}
	if m.SequenceNumber != nil {
		msg.SequenceNumber = *m.SequenceNumber
	}
	if m.EnqueuedTime != nil {
		msg.EnqueuedAt = *m.EnqueuedTime
	}
	if m.DeadLetterReason != nil {
		msg.DeadLetterReason = *m.DeadLetterReason
	}
	if m.DeadLetterErrorDescription != nil {
		msg.DeadLetterErrorDescription = *m.DeadLetterErrorDescription
	}
	return msg
}
