package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"servicebus/pkg/auth"
	"servicebus/pkg/management"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// DLReceiver manages dead letter sub-queues.
type DLReceiver interface {
	Peek(ctx context.Context, max int) ([]Message, error)
	Purge(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error)
	Replay(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error)
	Export(ctx context.Context, filters []management.Filter, max int, outPath string) error
}

type azureDLReceiver struct {
	receiver *azservicebus.Receiver
	sender   *azservicebus.Sender
}

const (
	defaultDLQBatchSize = 200
	defaultDLQWorkers   = 8
	defaultRetryDelay   = 200 * time.Millisecond
	defaultRetryMax     = 5 * time.Second
	defaultRetryCount   = 5
)

type dlqRuntimeOptions struct {
	max           int
	batchSize     int
	workers       int
	retryCount    int
	retryDelay    time.Duration
	retryMax      time.Duration
	progressEvery int
	checkpoint    string
	resume        bool
	delayMins     int
	ttlDays       int
}

type checkpointState struct {
	LastSequence int64     `json:"last_sequence"`
	Successful   int       `json:"successful"`
	UpdatedAt    time.Time `json:"updated_at"`
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

func (r *azureDLReceiver) Purge(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error) {
	rt := normalizeDLOptions(opts)
	processed := 0
	state, err := loadCheckpoint(rt.checkpoint, rt.resume)
	if err != nil {
		return 0, err
	}
	resumeFromSeq := state.LastSequence
	nextProgress := nextProgressMark(processed, rt.progressEvery)

	for {
		batchSize := rt.batchSize
		if rt.max > 0 {
			remaining := rt.max - processed
			if remaining <= 0 {
				return processed, nil
			}
			if remaining < batchSize {
				batchSize = remaining
			}
		}

		var msgs []*azservicebus.ReceivedMessage
		err = withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
			var recvErr error
			msgs, recvErr = r.receiver.ReceiveMessages(ctx, batchSize, nil)
			return recvErr
		})
		if err != nil {
			return processed, err
		}
		if len(msgs) == 0 {
			return processed, nil
		}

		converted := convertMessages(msgs)
		matched := ApplyMessageFilters(converted, filters)
		seqSet := make(map[int64]bool, len(matched))
		for _, m := range matched {
			seqSet[m.SequenceNumber] = true
		}

		purged, maxSeq, err := r.processPurgeBatch(ctx, msgs, seqSet, rt, resumeFromSeq)
		processed += purged
		if err != nil {
			return processed, err
		}
		if maxSeq > resumeFromSeq {
			resumeFromSeq = maxSeq
		}
		state.LastSequence = resumeFromSeq
		state.Successful = processed
		state.UpdatedAt = time.Now().UTC()
		if err := saveCheckpoint(rt.checkpoint, state); err != nil {
			return processed, err
		}
		nextProgress = maybePrintProgress("purge", processed, rt.progressEvery, nextProgress)
	}
}

func (r *azureDLReceiver) Replay(ctx context.Context, filters []management.Filter, opts DLOptions) (int, error) {
	rt := normalizeDLOptions(opts)
	processed := 0
	state, err := loadCheckpoint(rt.checkpoint, rt.resume)
	if err != nil {
		return 0, err
	}
	resumeFromSeq := state.LastSequence
	nextProgress := nextProgressMark(processed, rt.progressEvery)

	for {
		batchSize := rt.batchSize
		if rt.max > 0 {
			remaining := rt.max - processed
			if remaining <= 0 {
				return processed, nil
			}
			if remaining < batchSize {
				batchSize = remaining
			}
		}

		var msgs []*azservicebus.ReceivedMessage
		err = withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
			var recvErr error
			msgs, recvErr = r.receiver.ReceiveMessages(ctx, batchSize, nil)
			return recvErr
		})
		if err != nil {
			return processed, err
		}
		if len(msgs) == 0 {
			return processed, nil
		}

		converted := convertMessages(msgs)
		matched := ApplyMessageFilters(converted, filters)
		seqSet := make(map[int64]bool, len(matched))
		for _, m := range matched {
			seqSet[m.SequenceNumber] = true
		}

		replayed, maxSeq, err := r.processReplayBatch(ctx, msgs, seqSet, rt, resumeFromSeq)
		processed += replayed
		if err != nil {
			return processed, err
		}
		if maxSeq > resumeFromSeq {
			resumeFromSeq = maxSeq
		}
		state.LastSequence = resumeFromSeq
		state.Successful = processed
		state.UpdatedAt = time.Now().UTC()
		if err := saveCheckpoint(rt.checkpoint, state); err != nil {
			return processed, err
		}
		nextProgress = maybePrintProgress("replay", processed, rt.progressEvery, nextProgress)
	}
}

func (r *azureDLReceiver) processPurgeBatch(ctx context.Context, msgs []*azservicebus.ReceivedMessage, seqSet map[int64]bool, rt dlqRuntimeOptions, resumeFromSeq int64) (int, int64, error) {
	workers := rt.workers
	if len(msgs) < workers {
		workers = len(msgs)
	}
	if workers == 0 {
		return 0, resumeFromSeq, nil
	}

	jobs := make(chan *azservicebus.ReceivedMessage, len(msgs))
	errCh := make(chan error, workers)
	var purged int64
	var maxProcessedSeq int64

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				if ctx.Err() != nil {
					errCh <- ctx.Err()
					continue
				}
				seq := int64(0)
				if m.SequenceNumber != nil {
					seq = *m.SequenceNumber
				}
				if resumeFromSeq > 0 && seq <= resumeFromSeq {
					_ = withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						return r.receiver.AbandonMessage(ctx, m, nil)
					})
					continue
				}
				if seqSet[seq] {
					if err := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						return r.receiver.CompleteMessage(ctx, m, nil)
					}); err != nil {
						errCh <- fmt.Errorf("completing seq %d: %w", seq, err)
						continue
					}
					atomic.AddInt64(&purged, 1)
					for {
						curr := atomic.LoadInt64(&maxProcessedSeq)
						if seq <= curr || atomic.CompareAndSwapInt64(&maxProcessedSeq, curr, seq) {
							break
						}
					}
					continue
				}
				if err := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
					return r.receiver.AbandonMessage(ctx, m, nil)
				}); err != nil {
					errCh <- fmt.Errorf("abandoning seq %d: %w", seq, err)
				}
			}
		}()
	}

	for _, m := range msgs {
		jobs <- m
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	errList := make([]string, 0)
	for err := range errCh {
		if err != nil {
			errList = append(errList, err.Error())
		}
	}
	if len(errList) > 0 {
		sort.Strings(errList)
		return int(purged), atomic.LoadInt64(&maxProcessedSeq), errors.New(errList[0])
	}

	return int(purged), atomic.LoadInt64(&maxProcessedSeq), nil
}

func (r *azureDLReceiver) processReplayBatch(ctx context.Context, msgs []*azservicebus.ReceivedMessage, seqSet map[int64]bool, rt dlqRuntimeOptions, resumeFromSeq int64) (int, int64, error) {
	workers := rt.workers
	if len(msgs) < workers {
		workers = len(msgs)
	}
	if workers == 0 {
		return 0, resumeFromSeq, nil
	}

	jobs := make(chan *azservicebus.ReceivedMessage, len(msgs))
	errCh := make(chan error, workers)
	var replayed int64
	var maxProcessedSeq int64

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				if ctx.Err() != nil {
					errCh <- ctx.Err()
					continue
				}
				seq := int64(0)
				if m.SequenceNumber != nil {
					seq = *m.SequenceNumber
				}
				if resumeFromSeq > 0 && seq <= resumeFromSeq {
					_ = withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						return r.receiver.AbandonMessage(ctx, m, nil)
					})
					continue
				}

				if !seqSet[seq] {
					if err := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						return r.receiver.AbandonMessage(ctx, m, nil)
					}); err != nil {
						errCh <- fmt.Errorf("abandoning seq %d: %w", seq, err)
					}
					continue
				}

				newMsg := &azservicebus.Message{
					Body:                  m.Body,
					ApplicationProperties: m.ApplicationProperties,
				}
				if rt.ttlDays > 0 {
					ttl := time.Duration(rt.ttlDays) * 24 * time.Hour
					newMsg.TimeToLive = &ttl
				}

				if rt.delayMins > 0 {
					at := time.Now().Add(time.Duration(rt.delayMins) * time.Minute)
					if sendErr := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						_, err := r.sender.ScheduleMessages(ctx, []*azservicebus.Message{newMsg}, at, nil)
						return err
					}); sendErr != nil {
						if aErr := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
							return r.receiver.AbandonMessage(ctx, m, nil)
						}); aErr != nil {
							errCh <- fmt.Errorf("abandoning seq %d after send failure: %w", seq, aErr)
						}
						errCh <- fmt.Errorf("replaying seq %d: %w", seq, sendErr)
						continue
					}
				} else {
					if sendErr := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
						return r.sender.SendMessage(ctx, newMsg, nil)
					}); sendErr != nil {
						if aErr := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
							return r.receiver.AbandonMessage(ctx, m, nil)
						}); aErr != nil {
							errCh <- fmt.Errorf("abandoning seq %d after send failure: %w", seq, aErr)
						}
						errCh <- fmt.Errorf("replaying seq %d: %w", seq, sendErr)
						continue
					}
				}

				if err := withRetry(ctx, rt.retryCount, rt.retryDelay, rt.retryMax, func() error {
					return r.receiver.CompleteMessage(ctx, m, nil)
				}); err != nil {
					errCh <- fmt.Errorf("completing seq %d: %w", seq, err)
					continue
				}
				atomic.AddInt64(&replayed, 1)
				for {
					curr := atomic.LoadInt64(&maxProcessedSeq)
					if seq <= curr || atomic.CompareAndSwapInt64(&maxProcessedSeq, curr, seq) {
						break
					}
				}
			}
		}()
	}

	for _, m := range msgs {
		jobs <- m
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	errList := make([]string, 0)
	for err := range errCh {
		if err != nil {
			errList = append(errList, err.Error())
		}
	}
	if len(errList) > 0 {
		sort.Strings(errList)
		return int(replayed), atomic.LoadInt64(&maxProcessedSeq), errors.New(errList[0])
	}

	return int(replayed), atomic.LoadInt64(&maxProcessedSeq), nil
}

func normalizeDLOptions(opts DLOptions) dlqRuntimeOptions {
	rt := dlqRuntimeOptions{
		max:           opts.Max,
		batchSize:     opts.BatchSize,
		workers:       opts.Workers,
		retryCount:    opts.RetryMaxRetries,
		progressEvery: opts.ProgressEvery,
		checkpoint:    opts.CheckpointFile,
		resume:        opts.ResumeCheckpoint,
		delayMins:     opts.DelayMins,
		ttlDays:       opts.TTLDays,
	}
	if rt.batchSize <= 0 {
		rt.batchSize = defaultDLQBatchSize
	}
	if rt.workers <= 0 {
		rt.workers = defaultDLQWorkers
	}
	if rt.retryCount < 0 {
		rt.retryCount = 0
	}
	if rt.retryCount == 0 {
		rt.retryCount = defaultRetryCount
	}
	rt.retryDelay = defaultRetryDelay
	if opts.RetryDelayMS > 0 {
		rt.retryDelay = time.Duration(opts.RetryDelayMS) * time.Millisecond
	}
	rt.retryMax = defaultRetryMax
	if opts.RetryMaxDelayMS > 0 {
		rt.retryMax = time.Duration(opts.RetryMaxDelayMS) * time.Millisecond
	}
	if rt.retryDelay <= 0 {
		rt.retryDelay = defaultRetryDelay
	}
	if rt.retryMax < rt.retryDelay {
		rt.retryMax = rt.retryDelay
	}
	return rt
}

func withRetry(ctx context.Context, maxRetries int, delay, maxDelay time.Duration, op func() error) error {
	if maxRetries < 0 {
		maxRetries = 0
	}
	currentDelay := delay
	for attempt := 0; ; attempt++ {
		err := op()
		if err == nil {
			return nil
		}
		if attempt >= maxRetries {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(currentDelay):
		}
		currentDelay *= 2
		if currentDelay > maxDelay {
			currentDelay = maxDelay
		}
	}
}

func loadCheckpoint(path string, resume bool) (checkpointState, error) {
	if path == "" || !resume {
		return checkpointState{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return checkpointState{}, nil
	}
	if err != nil {
		return checkpointState{}, fmt.Errorf("read checkpoint %q: %w", path, err)
	}
	var state checkpointState
	if err := json.Unmarshal(data, &state); err != nil {
		return checkpointState{}, fmt.Errorf("parse checkpoint %q: %w", path, err)
	}
	return state, nil
}

func saveCheckpoint(path string, state checkpointState) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir checkpoint dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write checkpoint %q: %w", path, err)
	}
	return nil
}

func nextProgressMark(current, every int) int {
	if every <= 0 {
		return 0
	}
	if current == 0 {
		return every
	}
	return ((current / every) + 1) * every
}

func maybePrintProgress(kind string, processed, every, nextMark int) int {
	if every <= 0 {
		return 0
	}
	for processed >= nextMark {
		fmt.Printf("%s progress: %d messages processed\n", kind, nextMark)
		nextMark += every
	}
	return nextMark
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
