package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebus/pkg/auth"
)

// Sender is the unit-testable interface for sending a single message.
type Sender interface {
	Send(ctx context.Context, opts SendOptions) error
}

// BatchSender is implemented by senders that support efficient batch sends.
type BatchSender interface {
	SendBatch(ctx context.Context, opts SendOptions, count int) error
}

type azureSender struct {
	sender *azservicebus.Sender
}

// NewSender creates a Sender for the given queue or topic.
func NewSender(cred *auth.Credentials, queueOrTopic string) (Sender, error) {
	var client *azservicebus.Client
	var err error
	if cred.ConnectionString != "" {
		client, err = azservicebus.NewClientFromConnectionString(cred.ConnectionString, nil)
	} else {
		client, err = azservicebus.NewClient(cred.Namespace, cred.Token, nil)
	}
	if err != nil {
		return nil, err
	}
	s, err := client.NewSender(queueOrTopic, nil)
	if err != nil {
		return nil, err
	}
	return &azureSender{sender: s}, nil
}

func (a *azureSender) Send(ctx context.Context, opts SendOptions) error {
	msg := buildMessage(opts)
	if opts.DelayMins > 0 {
		at := time.Now().Add(time.Duration(opts.DelayMins) * time.Minute)
		_, err := a.sender.ScheduleMessages(ctx, []*azservicebus.Message{msg}, at, nil)
		return err
	}
	return a.sender.SendMessage(ctx, msg, nil)
}

// SendBatch coalesces up to count copies of opts.Body into MessageBatch
// instances, automatically flushing when a batch is full.
func (a *azureSender) SendBatch(ctx context.Context, opts SendOptions, count int) error {
	if count < 1 {
		count = 1
	}
	// Scheduled messages cannot be batched the same way — fall back to per-msg.
	if opts.DelayMins > 0 {
		for i := 0; i < count; i++ {
			if err := a.Send(ctx, opts); err != nil {
				return err
			}
		}
		return nil
	}
	batch, err := a.sender.NewMessageBatch(ctx, nil)
	if err != nil {
		return err
	}
	flush := func() error {
		if batch.NumMessages() == 0 {
			return nil
		}
		if err := a.sender.SendMessageBatch(ctx, batch, nil); err != nil {
			return err
		}
		batch, err = a.sender.NewMessageBatch(ctx, nil)
		return err
	}
	for i := 0; i < count; i++ {
		msg := buildMessage(opts)
		if err := batch.AddMessage(msg, nil); err != nil {
			// Batch full → flush and retry once.
			if errors.Is(err, azservicebus.ErrMessageTooLarge) {
				return fmt.Errorf("message too large for batch: %w", err)
			}
			if err := flush(); err != nil {
				return err
			}
			if err := batch.AddMessage(msg, nil); err != nil {
				return fmt.Errorf("add message after flush: %w", err)
			}
		}
	}
	return flush()
}

func buildMessage(opts SendOptions) *azservicebus.Message {
	msg := &azservicebus.Message{Body: opts.Body}
	if opts.Properties != nil {
		props := make(map[string]any, len(opts.Properties))
		for k, v := range opts.Properties {
			props[k] = v
		}
		msg.ApplicationProperties = props
	}
	return msg
}

// SendWithSender sends opts.Body opts.Count times (minimum 1).
// Uses BatchSender if available for major throughput gains on count > 1.
func SendWithSender(ctx context.Context, s Sender, opts SendOptions) error {
	count := opts.Count
	if count < 1 {
		count = 1
	}
	if count > 1 {
		if bs, ok := s.(BatchSender); ok {
			return bs.SendBatch(ctx, opts, count)
		}
	}
	for i := 0; i < count; i++ {
		if err := s.Send(ctx, opts); err != nil {
			return err
		}
	}
	return nil
}

// SendFromPath sends from a JSON file or all *.json files in a directory.
// Files in a directory are dispatched to a small worker pool for throughput.
func SendFromPath(ctx context.Context, s Sender, path string, base SendOptions) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return sendFile(ctx, s, path, base)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files = append(files, filepath.Join(path, e.Name()))
	}
	if len(files) == 0 {
		return nil
	}
	const sendWorkers = 8
	workers := sendWorkers
	if len(files) < workers {
		workers = len(files)
	}
	jobs := make(chan string, len(files))
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				if err := sendFile(ctx, s, f, base); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func sendFile(ctx context.Context, s Sender, path string, base SendOptions) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	var props map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		if p, ok := raw["__properties"]; ok {
			if err := json.Unmarshal(p, &props); err != nil {
				return fmt.Errorf("parsing __properties: %w", err)
			}
			delete(raw, "__properties")
			var marshalErr error
			if data, marshalErr = json.Marshal(raw); marshalErr != nil {
				return fmt.Errorf("re-marshalling message body: %w", marshalErr)
			}
		}
	}
	opts := base
	opts.Body = data
	if props != nil {
		opts.Properties = props
	}
	return SendWithSender(ctx, s, opts)
}
