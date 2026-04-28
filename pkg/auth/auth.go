package auth

import (
	"errors"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"servicebus/internal/config"
)

type Options struct {
	ConnectionString string
	Namespace        string
	ContextName      string
	Config           *config.Config
}

type Credentials struct {
	ConnectionString string
	Namespace        string
	Token            azcore.TokenCredential
}

// Resolve picks credentials in priority order:
// 1. explicit --connection-string flag
// 2. AZURE_SERVICEBUS_CONNECTION_STRING env var
// 3. named context from Config
// 4. current context from Config
// 5. --namespace flag → DefaultAzureCredential
func Resolve(opts Options) (*Credentials, error) {
	connectionString, namespace := normalizeCredentialFields(opts.ConnectionString, opts.Namespace)
	if connectionString != "" {
		return &Credentials{ConnectionString: connectionString}, nil
	}
	if cs := os.Getenv("AZURE_SERVICEBUS_CONNECTION_STRING"); cs != "" {
		return &Credentials{ConnectionString: cs}, nil
	}
	if opts.ContextName != "" {
		return resolveContext(opts.Config, opts.ContextName)
	}
	if opts.Config != nil && opts.Config.CurrentContext != "" {
		return resolveContext(opts.Config, opts.Config.CurrentContext)
	}
	if namespace != "" {
		return azureAD(namespace)
	}
	return nil, errors.New("no credentials: use --connection-string, set AZURE_SERVICEBUS_CONNECTION_STRING, or run 'sb config set-context'")
}

func fromContext(ctx config.Context) (*Credentials, error) {
	connectionString, namespace := normalizeCredentialFields(ctx.ConnectionString, ctx.Namespace)
	if connectionString != "" {
		return &Credentials{ConnectionString: connectionString}, nil
	}
	if namespace == "" {
		return nil, errors.New("context has neither connection-string nor namespace")
	}
	return azureAD(namespace)
}

func resolveContext(cfg *config.Config, name string) (*Credentials, error) {
	if cfg == nil {
		return nil, errors.New("context specified but no config loaded")
	}
	ctx, ok := cfg.Contexts[name]
	if !ok {
		return nil, errors.New("context not found: " + name)
	}
	return fromContext(ctx)
}

func normalizeCredentialFields(connectionString, namespace string) (string, string) {
	if strings.TrimSpace(connectionString) != "" {
		return connectionString, namespace
	}
	if looksLikeConnectionString(namespace) {
		return namespace, ""
	}
	return connectionString, namespace
}

func looksLikeConnectionString(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "Endpoint=sb://") || strings.HasPrefix(trimmed, "Endpoint=amqps://")
}

func azureAD(namespace string) (*Credentials, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return &Credentials{Namespace: namespace, Token: cred}, nil
}
