package messaging

import (
	"context"

	"servicebus/pkg/auth"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// Peeker is a read-only view over a queue, subscription or dead-letter sub-queue.
// It uses the SDK's PeekMessages API which does NOT lock or consume messages,
// so it never interferes with real consumers.
type Peeker interface {
	// PeekFrom returns up to max messages whose SequenceNumber is >= fromSeq.
	// fromSeq <= 0 means "from the earliest available message".
	PeekFrom(ctx context.Context, fromSeq int64, max int) ([]Message, error)
	Close(ctx context.Context) error
}

type azurePeeker struct {
	client   *azservicebus.Client
	receiver *azservicebus.Receiver
}

// PeekTarget describes which entity to peek.
type PeekTarget struct {
	Queue        string // queue name (set Queue OR Topic+Subscription)
	Topic        string
	Subscription string
	DeadLetter   bool // peek the DLQ sub-queue instead of the main entity
}

// NewPeeker builds a Peeker for the given target. Queue and (Topic,Subscription)
// are mutually exclusive.
func NewPeeker(cred *auth.Credentials, t PeekTarget) (Peeker, error) {
	client, err := newClient(cred)
	if err != nil {
		return nil, err
	}
	var opts *azservicebus.ReceiverOptions
	if t.DeadLetter {
		opts = &azservicebus.ReceiverOptions{SubQueue: azservicebus.SubQueueDeadLetter}
	}
	var recv *azservicebus.Receiver
	switch {
	case t.Queue != "":
		recv, err = client.NewReceiverForQueue(t.Queue, opts)
	case t.Topic != "" && t.Subscription != "":
		recv, err = client.NewReceiverForSubscription(t.Topic, t.Subscription, opts)
	default:
		client.Close(context.Background())
		return nil, errInvalidPeekTarget
	}
	if err != nil {
		client.Close(context.Background())
		return nil, err
	}
	return &azurePeeker{client: client, receiver: recv}, nil
}

func (p *azurePeeker) PeekFrom(ctx context.Context, fromSeq int64, max int) ([]Message, error) {
	if max <= 0 {
		max = 250
	}
	const pageSize = 250
	var (
		all       []Message
		cursor    *int64
		remaining = max
	)
	if fromSeq > 0 {
		v := fromSeq
		cursor = &v
	}
	for remaining > 0 {
		batch := pageSize
		if remaining < batch {
			batch = remaining
		}
		opts := &azservicebus.PeekMessagesOptions{}
		if cursor != nil {
			opts.FromSequenceNumber = cursor
		}
		peeked, err := p.receiver.PeekMessages(ctx, batch, opts)
		if err != nil {
			return all, err
		}
		if len(peeked) == 0 {
			break
		}
		all = append(all, convertMessages(peeked)...)
		last := peeked[len(peeked)-1]
		if last.SequenceNumber == nil {
			break
		}
		next := *last.SequenceNumber + 1
		cursor = &next
		remaining -= len(peeked)
		if len(peeked) < batch {
			break
		}
	}
	return all, nil
}

func (p *azurePeeker) Close(ctx context.Context) error {
	if err := p.receiver.Close(ctx); err != nil {
		_ = p.client.Close(ctx)
		return err
	}
	return p.client.Close(ctx)
}

var errInvalidPeekTarget = peekTargetError("PeekTarget requires Queue or (Topic + Subscription)")

type peekTargetError string

func (e peekTargetError) Error() string { return string(e) }
