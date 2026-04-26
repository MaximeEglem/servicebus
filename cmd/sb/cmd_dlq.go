package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"servicebus/internal/config"
	"servicebus/internal/format"
	"servicebus/pkg/management"
	"servicebus/pkg/messaging"
)

func buildDLQCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dlq",
		Short: "Manage dead letter queues",
		Long:  "List queues with dead letters, or use -n to view/purge/replay/export a specific queue's DLQ",
		RunE:  runDLQList,
	}
	cmd.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results")
	cmd.Flags().BoolVarP(&gf.all, "all", "a", false, "All results")
	cmd.Flags().StringVarP(&gf.sort, "sort", "s", "", "<Field>:asc|dsc")
	cmd.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Queue name (for DLQ operations)")

	cmd.AddCommand(
		buildDLQViewCmd(),
		buildDLQPurgeCmd(),
		buildDLQReplayCmd(),
		buildDLQExportCmd(),
	)
	return cmd
}

func runDLQList(cmd *cobra.Command, _ []string) error {
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	filters, err := parseFilters(gf.filters)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// If a name is provided, show info for that specific queue's DLQ
	if gf.name != "" {
		q, err := mgr.GetQueue(ctx, gf.name)
		if err != nil {
			return err
		}
		return printResult(cmd, []management.QueueInfo{*q}, cfg)
	}

	// List queues that have dead letter messages
	queues, err := mgr.ListQueues(ctx, management.ListOptions{})
	if err != nil {
		return err
	}

	// Filter to only queues with dead letters
	var dlQueues []management.QueueInfo
	for _, q := range queues {
		if q.DeadLetterMessages > 0 {
			dlQueues = append(dlQueues, q)
		}
	}

	dlQueues = management.ApplyQueueFilters(dlQueues, filters)
	if gf.sort != "" {
		field, dir := parseSortFlag(gf.sort)
		management.SortQueues(dlQueues, field, dir)
	}
	dlQueues = applyMax(dlQueues, cfg)
	return printResult(cmd, dlQueues, cfg)
}

func buildDLQViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "v",
		Short: "View dead letter messages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required to view messages")
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
				max = 1000
			}
			msgs, err := recv.Peek(context.Background(), max)
			if err != nil {
				return err
			}
			msgs = messaging.ApplyMessageFilters(msgs, filters)
			f, err := resolveFormat()
			if err != nil {
				return err
			}
			p := format.NewPrinter(f, os.Stdout, format.ThresholdConfig{})
			return p.Print(msgs)
		},
	}
	return cmd
}

func buildDLQPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "p",
		Short: "Purge (delete) dead letter messages",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required to purge messages")
			}
			if !yes {
				ok, err := confirmAction(fmt.Sprintf("Purge dead-letter messages from %s DLQ?", gf.name))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Println("Purge canceled.")
					return nil
				}
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
				max = 1000
			}
			count, err := recv.Purge(context.Background(), filters, max)
			if err != nil {
				return err
			}
			fmt.Printf("Purged %d messages from %s DLQ\n", count, gf.name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Queue name (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func buildDLQReplayCmd() *cobra.Command {
	var delayMins, ttlDays int
	cmd := &cobra.Command{
		Use:   "r",
		Short: "Replay (requeue) dead letter messages to main queue",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required to replay messages")
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
				max = 1000
			}
			opts := messaging.DLOptions{
				Max:       max,
				DelayMins: delayMins,
				TTLDays:   ttlDays,
			}
			count, err := recv.Replay(context.Background(), filters, opts)
			if err != nil {
				return err
			}
			fmt.Printf("Replayed %d messages from %s DLQ\n", count, gf.name)
			return nil
		},
	}
	cmd.Flags().IntVar(&delayMins, "delay", 0, "Scheduled delivery delay in minutes")
	cmd.Flags().IntVar(&ttlDays, "ttl", 0, "Time to live in days")
	return cmd
}

func buildDLQExportCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "x",
		Short: "Export dead letter messages to JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required to export messages")
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
				max = 1000
			}
			if outPath == "" {
				outPath = filepath.Join(expandDir(cfg.Settings.WorkingDirectory),
					fmt.Sprintf("dlq-%s-%s.json", gf.name, time.Now().Format("20060102-150405")))
			}
			if err := recv.Export(context.Background(), filters, max, outPath); err != nil {
				return err
			}
			fmt.Printf("Exported DLQ messages to %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	return cmd
}
