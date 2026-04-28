package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"servicebus/internal/config"
	"servicebus/pkg/auth"
	"servicebus/pkg/management"
	"servicebus/pkg/messaging"
)

// Long-lived caches keyed by context name. Avoids re-running config.Load,
// auth.Resolve and AAD client construction on every MCP tool call.
// A TTL ensures that on-disk config changes are picked up within mcpCacheTTL.
const mcpCacheTTL = 5 * time.Minute

type cachedManager struct {
	mgr management.Manager
	at  time.Time
}

type cachedCred struct {
	cred *auth.Credentials
	at   time.Time
}

type cachedConfig struct {
	cfg *config.Config
	at  time.Time
}

var (
	mcpManagerCache sync.Map     // ctxName → *cachedManager
	mcpCredCache    sync.Map     // ctxName → *cachedCred
	mcpConfig       atomic.Value // *cachedConfig
)

// MCP JSON-RPC types
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpInitResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    mcpCaps      `json:"capabilities"`
	ServerInfo      mcpServer    `json:"serverInfo"`
}

type mcpCaps struct {
	Tools map[string]interface{} `json:"tools,omitempty"`
}

type mcpServer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type mcpToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func buildMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP server (stdio JSON-RPC)",
		Long:  "Start an MCP server over stdio for integration with Claude Code and other MCP clients",
		RunE:  runMCPServer,
	}
	return cmd
}

func runMCPServer(_ *cobra.Command, _ []string) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := mcpResponse{
				JSONRPC: "2.0",
				Error:   &mcpError{Code: -32700, Message: "Parse error"},
			}
			encoder.Encode(resp)
			continue
		}

		resp := handleMCPRequest(req)
		encoder.Encode(resp)
	}

	return scanner.Err()
}

func handleMCPRequest(req mcpRequest) mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpInitResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: mcpCaps{
					Tools: map[string]interface{}{},
				},
				ServerInfo: mcpServer{
					Name:    "sb",
					Version: "1.0.0",
				},
			},
		}

	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": getMCPTools(),
			},
		}

	case "tools/call":
		var call mcpToolCall
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &mcpError{Code: -32602, Message: "Invalid params"},
			}
		}
		result, err := executeMCPTool(call)
		if err != nil {
			return mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpToolResult{
					Content: []mcpContent{{Type: "text", Text: err.Error()}},
					IsError: true,
				},
			}
		}
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpError{Code: -32601, Message: "Method not found"},
		}
	}
}

func getMCPTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "list_queues",
			Description: "List Azure Service Bus queues with optional filtering",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Named context from config",
					},
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter expression (e.g., 'order' or 'ActiveMessages:1-10')",
					},
					"max": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results",
					},
				},
			},
		},
		{
			Name:        "list_topics",
			Description: "List Azure Service Bus topics with optional filtering",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Named context from config",
					},
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter expression",
					},
					"max": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results",
					},
				},
			},
		},
		{
			Name:        "get_queue",
			Description: "Get details of a specific queue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Named context from config",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Queue name",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "peek_dlq",
			Description: "Peek at dead letter messages for a queue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Named context from config",
					},
					"queue": map[string]interface{}{
						"type":        "string",
						"description": "Queue name",
					},
					"max": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of messages to peek",
					},
				},
				"required": []string{"queue"},
			},
		},
		{
			Name:        "list_contexts",
			Description: "List configured Service Bus contexts",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func executeMCPTool(call mcpToolCall) (mcpToolResult, error) {
	ctx := context.Background()
	args := call.Arguments

	switch call.Name {
	case "list_queues":
		return executeListQueues(ctx, args)
	case "list_topics":
		return executeListTopics(ctx, args)
	case "get_queue":
		return executeGetQueue(ctx, args)
	case "peek_dlq":
		return executePeekDLQ(ctx, args)
	case "list_contexts":
		return executeListContexts()
	default:
		return mcpToolResult{}, fmt.Errorf("unknown tool: %s", call.Name)
	}
}

func executeListQueues(ctx context.Context, args map[string]interface{}) (mcpToolResult, error) {
	mgr, err := getMCPManager(args)
	if err != nil {
		return mcpToolResult{}, err
	}

	queues, err := mgr.ListQueues(ctx, management.ListOptions{})
	if err != nil {
		return mcpToolResult{}, err
	}

	// Apply filter if provided
	if filterStr, ok := args["filter"].(string); ok && filterStr != "" {
		filter, err := management.ParseFilter(filterStr)
		if err != nil {
			return mcpToolResult{}, err
		}
		queues = management.ApplyQueueFilters(queues, []management.Filter{filter})
	}

	// Apply max if provided
	if maxVal, ok := args["max"].(float64); ok && maxVal > 0 {
		max := int(maxVal)
		if len(queues) > max {
			queues = queues[:max]
		}
	}

	data, err := json.MarshalIndent(queues, "", "  ")
	if err != nil {
		return mcpToolResult{}, err
	}

	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(data)}},
	}, nil
}

func executeListTopics(ctx context.Context, args map[string]interface{}) (mcpToolResult, error) {
	mgr, err := getMCPManager(args)
	if err != nil {
		return mcpToolResult{}, err
	}

	topics, err := mgr.ListTopics(ctx, management.ListOptions{})
	if err != nil {
		return mcpToolResult{}, err
	}

	// Apply filter if provided
	if filterStr, ok := args["filter"].(string); ok && filterStr != "" {
		filter, err := management.ParseFilter(filterStr)
		if err != nil {
			return mcpToolResult{}, err
		}
		topics = management.ApplyTopicFilters(topics, []management.Filter{filter})
	}

	// Apply max if provided
	if maxVal, ok := args["max"].(float64); ok && maxVal > 0 {
		max := int(maxVal)
		if len(topics) > max {
			topics = topics[:max]
		}
	}

	data, err := json.MarshalIndent(topics, "", "  ")
	if err != nil {
		return mcpToolResult{}, err
	}

	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(data)}},
	}, nil
}

func executeGetQueue(ctx context.Context, args map[string]interface{}) (mcpToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcpToolResult{}, fmt.Errorf("name is required")
	}

	mgr, err := getMCPManager(args)
	if err != nil {
		return mcpToolResult{}, err
	}

	queue, err := mgr.GetQueue(ctx, name)
	if err != nil {
		return mcpToolResult{}, err
	}

	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return mcpToolResult{}, err
	}

	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(data)}},
	}, nil
}

func executePeekDLQ(ctx context.Context, args map[string]interface{}) (mcpToolResult, error) {
	queueName, ok := args["queue"].(string)
	if !ok || queueName == "" {
		return mcpToolResult{}, fmt.Errorf("queue is required")
	}

	cred, err := getMCPCredentials(args)
	if err != nil {
		return mcpToolResult{}, err
	}

	recv, err := messaging.NewDLReceiverForQueue(cred, queueName)
	if err != nil {
		return mcpToolResult{}, err
	}

	max := 10
	if maxVal, ok := args["max"].(float64); ok && maxVal > 0 {
		max = int(maxVal)
	}

	msgs, err := recv.Peek(ctx, max)
	if err != nil {
		return mcpToolResult{}, err
	}

	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return mcpToolResult{}, err
	}

	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(data)}},
	}, nil
}

func executeListContexts() (mcpToolResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return mcpToolResult{}, err
	}

	result := map[string]interface{}{
		"current": cfg.CurrentContext,
		"contexts": func() []string {
			names := make([]string, 0, len(cfg.Contexts))
			for name := range cfg.Contexts {
				names = append(names, name)
			}
			return names
		}(),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcpToolResult{}, err
	}

	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(data)}},
	}, nil
}

func getMCPManager(args map[string]interface{}) (management.Manager, error) {
	ctxName, err := resolveMCPContextName(args)
	if err != nil {
		return nil, err
	}
	if v, ok := mcpManagerCache.Load(ctxName); ok {
		c := v.(*cachedManager)
		if time.Since(c.at) < mcpCacheTTL {
			return c.mgr, nil
		}
	}
	cred, err := getMCPCredentials(args)
	if err != nil {
		return nil, err
	}
	mgr, err := management.NewManager(cred)
	if err != nil {
		return nil, err
	}
	mcpManagerCache.Store(ctxName, &cachedManager{mgr: mgr, at: time.Now()})
	return mgr, nil
}

func getMCPCredentials(args map[string]interface{}) (*auth.Credentials, error) {
	ctxName, err := resolveMCPContextName(args)
	if err != nil {
		return nil, err
	}
	if v, ok := mcpCredCache.Load(ctxName); ok {
		c := v.(*cachedCred)
		if time.Since(c.at) < mcpCacheTTL {
			return c.cred, nil
		}
	}
	cfg, err := loadMCPConfig()
	if err != nil {
		return nil, err
	}
	cred, err := auth.Resolve(auth.Options{
		ContextName: ctxName,
		Config:      cfg,
	})
	if err != nil {
		return nil, err
	}
	mcpCredCache.Store(ctxName, &cachedCred{cred: cred, at: time.Now()})
	return cred, nil
}

// resolveMCPContextName picks the context name from args, current config,
// or last-used state file (in that order).
func resolveMCPContextName(args map[string]interface{}) (string, error) {
	if c, ok := args["context"].(string); ok && c != "" {
		return c, nil
	}
	cfg, err := loadMCPConfig()
	if err != nil {
		return "", err
	}
	if cfg.CurrentContext != "" {
		return cfg.CurrentContext, nil
	}
	if state, err := config.LoadState(); err == nil && state.Context != "" {
		return state.Context, nil
	}
	return "", fmt.Errorf("no context specified and no current context configured")
}

// loadMCPConfig caches the config with a TTL so on-disk edits are picked up.
func loadMCPConfig() (*config.Config, error) {
	if v := mcpConfig.Load(); v != nil {
		c := v.(*cachedConfig)
		if time.Since(c.at) < mcpCacheTTL {
			return c.cfg, nil
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	mcpConfig.Store(&cachedConfig{cfg: cfg, at: time.Now()})
	return cfg, nil
}
