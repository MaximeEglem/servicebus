package main

import (
	"context"
	"fmt"

	"servicebus/pkg/management"
	"servicebus/pkg/messaging"

	"github.com/spf13/cobra"
)

func buildSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Manage subscriptions (requires -t <topic>)",
		RunE:  runSubList,
	}
	cmd.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results per page")
	cmd.Flags().IntVar(&gf.page, "page", 0, "Page index, 1-based (default 1)")
	cmd.Flags().BoolVarP(&gf.all, "all", "a", false, "All results")
	cmd.Flags().StringVarP(&gf.sort, "sort", "s", "", "<Field>:asc|dsc")
	cmd.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Subscription name (shows info)")
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (required)")
	cmd.MarkFlagRequired("topic")
	cmd.AddCommand(buildSubSendCmd())
	return cmd
}

func runSubList(_ *cobra.Command, _ []string) error {
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	progress := newListProgress("subs", 4)
	filters, err := parseFilters(gf.filters)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if gf.name != "" {
		s, err := mgr.GetSubscription(ctx, gf.topic, gf.name)
		if err != nil {
			return err
		}
		return printResult(nil, []management.SubscriptionInfo{*s}, cfg)
	}
	subs, err := mgr.ListSubscriptions(ctx, gf.topic, management.ListOptions{})
	if err != nil {
		return err
	}
	progress.step(fmt.Sprintf("loaded %d", len(subs)))
	subs = management.ApplySubscriptionFilters(subs, filters)
	progress.step(fmt.Sprintf("filtered %d", len(subs)))
	sortSubscriptionsByFlags(subs)
	progress.step("sorted")
	subs = applyMaxSubs(subs, cfg)
	progress.step(fmt.Sprintf("ready %d", len(subs)))
	return printResult(nil, subs, cfg)
}

func buildSubSendCmd() *cobra.Command {
	var path string
	var count, delayMins int
	cmd := &cobra.Command{
		Use:   "s",
		Short: "Send messages to a topic subscription",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.topic == "" {
				return fmt.Errorf("-t <topic> is required")
			}
			cred, err := resolveCred()
			if err != nil {
				return err
			}
			sender, err := messaging.NewSender(cred, gf.topic)
			if err != nil {
				return err
			}
			opts := messaging.SendOptions{Count: count, DelayMins: delayMins}
			return messaging.SendFromPath(context.Background(), sender, path, opts)
		},
	}
	cmd.Flags().StringVarP(&path, "path", "p", "", "JSON file or directory path")
	cmd.Flags().IntVar(&count, "count", 1, "Number of times to send")
	cmd.Flags().IntVar(&delayMins, "delay", 0, "Scheduled delivery delay in minutes")
	cmd.MarkFlagRequired("path")
	return cmd
}
