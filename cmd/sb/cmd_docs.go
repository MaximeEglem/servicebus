package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func buildDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate markdown documentation",
		Long:  "Generate a comprehensive markdown reference for all sb commands",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			fmt.Println(generateDocs(root, 1))
			return nil
		},
	}
	return cmd
}

func generateDocs(cmd *cobra.Command, level int) string {
	var sb strings.Builder

	// Header
	header := strings.Repeat("#", level)
	sb.WriteString(fmt.Sprintf("%s %s\n\n", header, cmd.UseLine()))

	// Description
	if cmd.Long != "" {
		sb.WriteString(cmd.Long + "\n\n")
	} else if cmd.Short != "" {
		sb.WriteString(cmd.Short + "\n\n")
	}

	// Usage
	sb.WriteString("**Usage:**\n\n```\n")
	sb.WriteString(cmd.UseLine() + "\n")
	sb.WriteString("```\n\n")

	// Flags
	hasFlags := false
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		hasFlags = true
	})

	if hasFlags {
		sb.WriteString("**Flags:**\n\n")
		sb.WriteString("| Flag | Short | Type | Default | Description |\n")
		sb.WriteString("|------|-------|------|---------|-------------|\n")
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			short := ""
			if f.Shorthand != "" {
				short = "-" + f.Shorthand
			}
			sb.WriteString(fmt.Sprintf("| --%s | %s | %s | %s | %s |\n",
				f.Name, short, f.Value.Type(), f.DefValue, f.Usage))
		})
		sb.WriteString("\n")
	}

	// Global flags (only for root)
	if !cmd.HasParent() {
		sb.WriteString("**Global Flags:**\n\n")
		sb.WriteString("| Flag | Short | Type | Default | Description |\n")
		sb.WriteString("|------|-------|------|---------|-------------|\n")
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			short := ""
			if f.Shorthand != "" {
				short = "-" + f.Shorthand
			}
			sb.WriteString(fmt.Sprintf("| --%s | %s | %s | %s | %s |\n",
				f.Name, short, f.Value.Type(), f.DefValue, f.Usage))
		})
		sb.WriteString("\n")
	}

	// Subcommands
	subCmds := cmd.Commands()
	var visibleCmds []*cobra.Command
	for _, sub := range subCmds {
		if sub.Name() != "help" && sub.Name() != "completion" {
			visibleCmds = append(visibleCmds, sub)
		}
	}

	if len(visibleCmds) > 0 {
		sb.WriteString("**Subcommands:**\n\n")
		sb.WriteString("| Command | Description |\n")
		sb.WriteString("|---------|-------------|\n")
		for _, sub := range visibleCmds {
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", sub.Name(), sub.Short))
		}
		sb.WriteString("\n")

		// Generate docs for each subcommand
		for _, sub := range visibleCmds {
			sb.WriteString("---\n\n")
			sb.WriteString(generateDocs(sub, min(level+1, 6)))
		}
	}

	return sb.String()
}

// suppress unused import
var _ = os.Stdout
