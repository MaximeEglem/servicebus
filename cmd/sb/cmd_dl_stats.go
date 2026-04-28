package main

import (
	"context"
	"fmt"
	"os"

	"servicebus/internal/config"
	"servicebus/internal/format"
	"servicebus/pkg/messaging"

	"github.com/spf13/cobra"
)

func buildDLQStatsCmd() *cobra.Command {
	var by string
	var top int
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Group dead-letter messages by error type, description, or body hash",
		Long: `Aggregate dead-letter messages and report counts per group.
Group keys: reason (default), description, body-hash.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required")
			}
			key, ok := messaging.ParseGroupKey(by)
			if !ok {
				return fmt.Errorf("invalid --by %q (use reason, description, or body-hash)", by)
			}
			cred, err := resolveCred()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			filters, err := parseFilters(gf.filters)
			if err != nil {
				return err
			}
			recv, err := messaging.NewDLReceiverForQueue(cred, gf.name)
			if err != nil {
				return err
			}
			max := gf.max
			if max <= 0 {
				max = cfg.Settings.MaxItems
			}
			if gf.all {
				max = 0
			}
			msgs, err := recv.Peek(context.Background(), max)
			if err != nil {
				return err
			}
			msgs = messaging.ApplyMessageFilters(msgs, filters)
			stats := messaging.GroupMessages(msgs, key, top)
			f, err := resolveFormat()
			if err != nil {
				return err
			}
			p := format.NewPrinter(f, os.Stdout, format.ThresholdConfig{})
			return p.Print(stats)
		},
	}
	cmd.Flags().StringVar(&by, "by", "reason", "Group key: reason | description | body-hash")
	cmd.Flags().IntVar(&top, "top", 0, "Limit to top N groups (0 = all)")
	return cmd
}

func buildDLSStatsCmd() *cobra.Command {
	var by string
	var top int
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Group subscription dead-letter messages by error type, description, or body hash",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("--topic / -t and --name / -n are required")
			}
			key, ok := messaging.ParseGroupKey(by)
			if !ok {
				return fmt.Errorf("invalid --by %q (use reason, description, or body-hash)", by)
			}
			cred, err := resolveCred()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			filters, err := parseFilters(gf.filters)
			if err != nil {
				return err
			}
			recv, err := messaging.NewDLReceiverForSubscription(cred, gf.topic, gf.name)
			if err != nil {
				return err
			}
			max := gf.max
			if max <= 0 {
				max = cfg.Settings.MaxItems
			}
			if gf.all {
				max = 0
			}
			msgs, err := recv.Peek(context.Background(), max)
			if err != nil {
				return err
			}
			msgs = messaging.ApplyMessageFilters(msgs, filters)
			stats := messaging.GroupMessages(msgs, key, top)
			f, err := resolveFormat()
			if err != nil {
				return err
			}
			p := format.NewPrinter(f, os.Stdout, format.ThresholdConfig{})
			return p.Print(stats)
		},
	}
	cmd.Flags().StringVar(&by, "by", "reason", "Group key: reason | description | body-hash")
	cmd.Flags().IntVar(&top, "top", 0, "Limit to top N groups (0 = all)")
	return cmd
}
