package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandSchema represents a command in the JSON schema output.
type CommandSchema struct {
	Name        string          `json:"name"`
	Short       string          `json:"short,omitempty"`
	Long        string          `json:"long,omitempty"`
	Usage       string          `json:"usage,omitempty"`
	Flags       []FlagSchema    `json:"flags,omitempty"`
	Subcommands []CommandSchema `json:"subcommands,omitempty"`
}

// FlagSchema represents a flag in the JSON schema output.
type FlagSchema struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage,omitempty"`
}

func buildSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Output command tree as JSON schema",
		Long:  "Generate a JSON representation of the full command tree for MCP/Claude Code skill integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			schema := buildCommandSchema(root)
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(schema)
		},
	}
	return cmd
}

func buildCommandSchema(cmd *cobra.Command) CommandSchema {
	schema := CommandSchema{
		Name:  cmd.Name(),
		Short: cmd.Short,
		Long:  cmd.Long,
		Usage: cmd.UseLine(),
	}

	// Add local flags (non-persistent)
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flag := FlagSchema{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Usage:     f.Usage,
		}
		schema.Flags = append(schema.Flags, flag)
	})

	// Add persistent flags only for root command
	if !cmd.HasParent() {
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			flag := FlagSchema{
				Name:      f.Name,
				Shorthand: f.Shorthand,
				Type:      f.Value.Type(),
				Default:   f.DefValue,
				Usage:     f.Usage + " (global)",
			}
			schema.Flags = append(schema.Flags, flag)
		})
	}

	// Recursively add subcommands
	for _, sub := range cmd.Commands() {
		if sub.Name() == "help" || sub.Name() == "completion" {
			continue // Skip built-in commands
		}
		schema.Subcommands = append(schema.Subcommands, buildCommandSchema(sub))
	}

	return schema
}

// suppress unused import warning
var _ = fmt.Sprintf
