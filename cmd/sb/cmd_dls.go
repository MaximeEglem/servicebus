package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"servicebus/internal/config"
	"servicebus/internal/format"
	"servicebus/pkg/management"
	"servicebus/pkg/messaging"

	"github.com/spf13/cobra"
)

func buildDLSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dls",
		Short: "Manage dead letter subscriptions",
		Long:  "List subscriptions with dead letters, or use -t and -n to view/purge/replay/export a specific subscription's DLQ",
		RunE:  runDLSList,
	}
	cmd.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results")
	cmd.Flags().BoolVarP(&gf.all, "all", "a", false, "All results")
	cmd.Flags().StringVarP(&gf.sort, "sort", "s", "", "<Field>:asc|dsc")
	cmd.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Subscription name")
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (required)")

	cmd.AddCommand(
		buildDLSViewCmd(),
		buildDLSPurgeCmd(),
		buildDLSReplayCmd(),
		buildDLSExportCmd(),
	)
	return cmd
}

func runDLSList(cmd *cobra.Command, _ []string) error {
	if gf.topic == "" {
		return fmt.Errorf("--topic / -t is required")
	}
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	filters, err := parseFilters(gf.filters)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// If a name is provided, show info for that specific subscription's DLQ
	if gf.name != "" {
		sub, err := mgr.GetSubscription(ctx, gf.topic, gf.name)
		if err != nil {
			return err
		}
		return printResult(cmd, []management.SubscriptionInfo{*sub}, cfg)
	}

	// List subscriptions that have dead letter messages
	subs, err := mgr.ListSubscriptions(ctx, gf.topic, management.ListOptions{})
	if err != nil {
		return err
	}

	// Filter to only subscriptions with dead letters
	var dlSubs []management.SubscriptionInfo
	for _, s := range subs {
		if s.DeadLetterMessages > 0 {
			dlSubs = append(dlSubs, s)
		}
	}

	dlSubs = management.ApplySubscriptionFilters(dlSubs, filters)
	if gf.sort != "" {
		field, dir := parseSortFlag(gf.sort)
		management.SortSubscriptions(dlSubs, field, dir)
	}
	dlSubs = applyMaxSubs(dlSubs, cfg)
	return printResult(cmd, dlSubs, cfg)
}

func buildDLSViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "v",
		Short: "View dead letter messages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("--topic / -t and --name / -n are required to view messages")
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

func buildDLSPurgeCmd() *cobra.Command {
	var yes bool
	var batchSize, workers int
	var retryMaxRetries, retryDelayMS, retryMaxDelayMS int
	var progressEvery int
	var checkpointFile string
	var resume bool
	cmd := &cobra.Command{
		Use:   "p",
		Short: "Purge (delete) dead letter messages",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("--topic / -t and --name / -n are required to purge messages")
			}
			if !yes {
				ok, err := confirmAction(fmt.Sprintf("Purge dead-letter messages from %s/%s DLQ?", gf.topic, gf.name))
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
			opts := messaging.DLOptions{
				Max:              max,
				BatchSize:        batchSize,
				Workers:          workers,
				RetryMaxRetries:  retryMaxRetries,
				RetryDelayMS:     retryDelayMS,
				RetryMaxDelayMS:  retryMaxDelayMS,
				ProgressEvery:    progressEvery,
				CheckpointFile:   checkpointFile,
				ResumeCheckpoint: resume,
			}
			count, err := recv.Purge(context.Background(), filters, opts)
			if err != nil {
				return err
			}
			fmt.Printf("Purged %d messages from %s/%s DLQ\n", count, gf.topic, gf.name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (required)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Subscription name (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().IntVar(&batchSize, "batch-size", 200, "Receive batch size")
	cmd.Flags().IntVar(&workers, "workers", 8, "Async worker count")
	cmd.Flags().IntVar(&retryMaxRetries, "retry-max-retries", 5, "Maximum retries for transient failures")
	cmd.Flags().IntVar(&retryDelayMS, "retry-delay-ms", 200, "Initial retry delay in milliseconds")
	cmd.Flags().IntVar(&retryMaxDelayMS, "retry-max-delay-ms", 5000, "Maximum retry delay in milliseconds")
	cmd.Flags().IntVar(&progressEvery, "progress-every", 0, "Print progress every N successful operations (0 disables)")
	cmd.Flags().StringVar(&checkpointFile, "checkpoint-file", "", "Checkpoint file path for long-running operations")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from checkpoint file")
	return cmd
}

func buildDLSReplayCmd() *cobra.Command {
	var delayMins, ttlDays int
	var batchSize, workers int
	var retryMaxRetries, retryDelayMS, retryMaxDelayMS int
	var progressEvery int
	var checkpointFile string
	var resume bool
	cmd := &cobra.Command{
		Use:   "r",
		Short: "Replay (requeue) dead letter messages to topic",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("--topic / -t and --name / -n are required to replay messages")
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
			opts := messaging.DLOptions{
				Max:              max,
				DelayMins:        delayMins,
				TTLDays:          ttlDays,
				BatchSize:        batchSize,
				Workers:          workers,
				RetryMaxRetries:  retryMaxRetries,
				RetryDelayMS:     retryDelayMS,
				RetryMaxDelayMS:  retryMaxDelayMS,
				ProgressEvery:    progressEvery,
				CheckpointFile:   checkpointFile,
				ResumeCheckpoint: resume,
			}
			count, err := recv.Replay(context.Background(), filters, opts)
			if err != nil {
				return err
			}
			fmt.Printf("Replayed %d messages from %s/%s DLQ to topic\n", count, gf.topic, gf.name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (required)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Subscription name (required)")
	cmd.Flags().IntVar(&delayMins, "delay", 0, "Scheduled delivery delay in minutes")
	cmd.Flags().IntVar(&ttlDays, "ttl", 0, "Time to live in days")
	cmd.Flags().IntVar(&batchSize, "batch-size", 200, "Receive batch size")
	cmd.Flags().IntVar(&workers, "workers", 8, "Async worker count")
	cmd.Flags().IntVar(&retryMaxRetries, "retry-max-retries", 5, "Maximum retries for transient failures")
	cmd.Flags().IntVar(&retryDelayMS, "retry-delay-ms", 200, "Initial retry delay in milliseconds")
	cmd.Flags().IntVar(&retryMaxDelayMS, "retry-max-delay-ms", 5000, "Maximum retry delay in milliseconds")
	cmd.Flags().IntVar(&progressEvery, "progress-every", 0, "Print progress every N successful operations (0 disables)")
	cmd.Flags().StringVar(&checkpointFile, "checkpoint-file", "", "Checkpoint file path for long-running operations")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from checkpoint file")
	return cmd
}

func buildDLSExportCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "x",
		Short: "Export dead letter messages to JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.topic == "" || gf.name == "" {
				return fmt.Errorf("--topic / -t and --name / -n are required to export messages")
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
				max = 1000
			}
			if outPath == "" {
				outPath = filepath.Join(expandDir(cfg.Settings.WorkingDirectory),
					fmt.Sprintf("dls-%s-%s-%s.json", gf.topic, gf.name, time.Now().Format("20060102-150405")))
			}
			if err := recv.Export(context.Background(), filters, max, outPath); err != nil {
				return err
			}
			fmt.Printf("Exported DLS messages to %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	return cmd
}
