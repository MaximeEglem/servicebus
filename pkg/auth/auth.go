package auth

import (
	"errors"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/maxime/sb/internal/config"
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
// 4. --namespace flag → DefaultAzureCredential
func Resolve(opts Options) (*Credentials, error) {
	if opts.ConnectionString != "" {
		return &Credentials{ConnectionString: opts.ConnectionString}, nil
	}
	if cs := os.Getenv("AZURE_SERVICEBUS_CONNECTION_STRING"); cs != "" {
		return &Credentials{ConnectionString: cs}, nil
	}
	if opts.ContextName != "" {
		if opts.Config == nil {
			return nil, errors.New("context specified but no config loaded")
		}
		ctx, ok := opts.Config.Contexts[opts.ContextName]
		if !ok {
			return nil, errors.New("context not found: " + opts.ContextName)
		}
		return fromContext(ctx)
	}
	if opts.Namespace != "" {
		return azureAD(opts.Namespace)
	}
	return nil, errors.New("no credentials: use --connection-string, set AZURE_SERVICEBUS_CONNECTION_STRING, or run 'sb config set-context'")
}

func fromContext(ctx config.Context) (*Credentials, error) {
	if ctx.ConnectionString != "" {
		return &Credentials{ConnectionString: ctx.ConnectionString}, nil
	}
	return azureAD(ctx.Namespace)
}

func azureAD(namespace string) (*Credentials, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return &Credentials{Namespace: namespace, Token: cred}, nil
}
