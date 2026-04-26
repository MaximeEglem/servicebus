package messaging

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/maxime/sb/pkg/auth"
)

// Sender is the unit-testable interface for sending a single message.
type Sender interface {
	Send(ctx context.Context, opts SendOptions) error
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
	msg := &azservicebus.Message{Body: opts.Body}
	if opts.Properties != nil {
		props := make(map[string]any, len(opts.Properties))
		for k, v := range opts.Properties {
			props[k] = v
		}
		msg.ApplicationProperties = props
	}
	if opts.DelayMins > 0 {
		at := time.Now().Add(time.Duration(opts.DelayMins) * time.Minute)
		_, err := a.sender.ScheduleMessages(ctx, []*azservicebus.Message{msg}, at, nil)
		return err
	}
	return a.sender.SendMessage(ctx, msg, nil)
}

// SendWithSender sends opts.Body opts.Count times (minimum 1).
func SendWithSender(ctx context.Context, s Sender, opts SendOptions) error {
	count := opts.Count
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		if err := s.Send(ctx, opts); err != nil {
			return err
		}
	}
	return nil
}

// SendFromPath sends from a JSON file or all *.json files in a directory.
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
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := sendFile(ctx, s, filepath.Join(path, e.Name()), base); err != nil {
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
			json.Unmarshal(p, &props)
			delete(raw, "__properties")
			data, _ = json.Marshal(raw)
		}
	}
	opts := base
	opts.Body = data
	if props != nil {
		opts.Properties = props
	}
	return SendWithSender(ctx, s, opts)
}
