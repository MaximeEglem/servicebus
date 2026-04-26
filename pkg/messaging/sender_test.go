package messaging_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxime/sb/pkg/messaging"
)

type mockSender struct {
	sent []messaging.SendOptions
}

func (m *mockSender) Send(_ context.Context, opts messaging.SendOptions) error {
	m.sent = append(m.sent, opts)
	return nil
}

func TestSendCountRepeats(t *testing.T) {
	mock := &mockSender{}
	err := messaging.SendWithSender(context.Background(), mock, messaging.SendOptions{
		Body:  []byte(`{"id":1}`),
		Count: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 3 {
		t.Errorf("sent %d messages, want 3", len(mock.sent))
	}
}

func TestSendFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "msg.json")
	os.WriteFile(f, []byte(`{"event":"test"}`), 0644)

	mock := &mockSender{}
	err := messaging.SendFromPath(context.Background(), mock, f, messaging.SendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Errorf("sent %d messages, want 1", len(mock.sent))
	}
}

func TestSendFromDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"a":1}`), 0644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{"b":2}`), 0644)
	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte(`not json`), 0644)

	mock := &mockSender{}
	err := messaging.SendFromPath(context.Background(), mock, dir, messaging.SendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 2 {
		t.Errorf("sent %d messages, want 2 (skips .txt)", len(mock.sent))
	}
}
