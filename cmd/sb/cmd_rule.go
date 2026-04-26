package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func buildRuleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage subscription rules/filters",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("-t <topic> and -n <subscription> are required")
			}
			mgr, cfg, err := newManager()
			if err != nil {
				return err
			}
			rules, err := mgr.ListRules(context.Background(), gf.topic, gf.name)
			if err != nil {
				return err
			}
			return printResult(nil, rules, cfg)
		},
	}
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (required)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Subscription name (required)")
	cmd.AddCommand(buildRuleCreateCmd(), buildRuleDeleteCmd())
	return cmd
}

func buildRuleCreateCmd() *cobra.Command {
	var sqlFilter string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a subscription rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			mgr, _, err := newManager()
			if err != nil {
				return err
			}
			if err := mgr.CreateRule(context.Background(), gf.topic, gf.name, args[0], sqlFilter); err != nil {
				return err
			}
			fmt.Printf("Rule %q created.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&sqlFilter, "filter", "1=1", "SQL filter expression")
	return cmd
}

func buildRuleDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a subscription rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			mgr, _, err := newManager()
			if err != nil {
				return err
			}
			if err := mgr.DeleteRule(context.Background(), gf.topic, gf.name, args[0]); err != nil {
				return err
			}
			fmt.Printf("Rule %q deleted.\n", args[0])
			return nil
		},
	}
}
