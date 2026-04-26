package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/maxime/sb/internal/config"
)

func buildUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use [context] [type]",
		Short: "Set the active context and/or resource type for short commands",
		Long: `Set the active context and resource type so you can omit -c and the command name.

Examples:
  sb use dev          # set active context to dev
  sb use dev dlq      # set active context to dev, resource type to dlq
  sb use              # show current session state`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := config.LoadState()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				fmt.Printf("context: %s\ntype:    %s\n", state.Context, state.Type)
				return nil
			}
			state.Context = args[0]
			if len(args) == 2 {
				validTypes := map[string]bool{"q": true, "t": true, "sub": true, "dlq": true, "dls": true, "rule": true}
				if !validTypes[args[1]] {
					return fmt.Errorf("unknown type %q: use q, t, sub, dlq, dls, or rule", args[1])
				}
				state.Type = args[1]
			}
			if err := config.SaveState(state); err != nil {
				return err
			}
			fmt.Printf("Active: context=%s type=%s\n", state.Context, state.Type)
			return nil
		},
	}
}
