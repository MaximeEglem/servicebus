package auth_test

import (
	"os"
	"testing"

	"github.com/maxime/sb/internal/config"
	"github.com/maxime/sb/pkg/auth"
)

func TestResolveExplicitConnectionString(t *testing.T) {
	cred, err := auth.Resolve(auth.Options{
		ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=dmFsdWU=",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cred.ConnectionString == "" {
		t.Error("expected ConnectionString to be set")
	}
}

func TestResolveEnvVar(t *testing.T) {
	t.Setenv("AZURE_SERVICEBUS_CONNECTION_STRING",
		"Endpoint=sb://env.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=dmFsdWU=")
	cred, err := auth.Resolve(auth.Options{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cred.ConnectionString == "" {
		t.Error("expected ConnectionString from env var")
	}
}

func TestResolveNamedContext(t *testing.T) {
	os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING")
	t.Cleanup(func() { os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING") })
	cfg := &config.Config{
		Contexts: map[string]config.Context{
			"staging": {ConnectionString: "Endpoint=sb://staging.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=dmFsdWU="},
		},
	}
	cred, err := auth.Resolve(auth.Options{ContextName: "staging", Config: cfg})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cred.ConnectionString == "" {
		t.Error("expected ConnectionString from named context")
	}
}

func TestResolveUnknownContextErrors(t *testing.T) {
	os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING")
	t.Cleanup(func() { os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING") })
	cfg := &config.Config{Contexts: map[string]config.Context{}}
	_, err := auth.Resolve(auth.Options{ContextName: "ghost", Config: cfg})
	if err == nil {
		t.Error("expected error for unknown context")
	}
}

func TestResolveNothingErrors(t *testing.T) {
	os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING")
	t.Cleanup(func() { os.Unsetenv("AZURE_SERVICEBUS_CONNECTION_STRING") })
	_, err := auth.Resolve(auth.Options{})
	if err == nil {
		t.Error("expected error when no credentials available")
	}
}
