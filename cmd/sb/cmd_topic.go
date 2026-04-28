package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"servicebus/pkg/management"

	"github.com/spf13/cobra"
)

func buildTopicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "t",
		Short: "Manage topics",
		RunE:  runTopicList,
	}
	cmd.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results")
	cmd.Flags().BoolVarP(&gf.all, "all", "a", false, "All results")
	cmd.Flags().StringVarP(&gf.sort, "sort", "s", "", "<Field>:asc|dsc")
	cmd.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Topic name (shows info)")
	cmd.AddCommand(buildTopicExportCmd())
	return cmd
}

func runTopicList(_ *cobra.Command, _ []string) error {
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	progress := newListProgress("topics", 4)
	filters, err := parseFilters(gf.filters)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if gf.name != "" {
		t, err := mgr.GetTopic(ctx, gf.name)
		if err != nil {
			return err
		}
		return printResult(nil, []management.TopicInfo{*t}, cfg)
	}
	topics, err := mgr.ListTopics(ctx, management.ListOptions{})
	if err != nil {
		return err
	}
	progress.step(fmt.Sprintf("loaded %d", len(topics)))
	topics = management.ApplyTopicFilters(topics, filters)
	progress.step(fmt.Sprintf("filtered %d", len(topics)))
	sortTopicsByFlags(topics)
	progress.step("sorted")
	topics = applyMaxTopics(topics, cfg)
	progress.step(fmt.Sprintf("ready %d", len(topics)))
	return printResult(nil, topics, cfg)
}

func buildTopicExportCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "x",
		Short: "Export topic list to JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			mgr, cfg, err := newManager()
			if err != nil {
				return err
			}
			progress := newListProgress("topics", 4)
			filters, err := parseFilters(gf.filters)
			if err != nil {
				return err
			}
			topics, err := mgr.ListTopics(context.Background(), management.ListOptions{})
			if err != nil {
				return err
			}
			progress.step(fmt.Sprintf("loaded %d", len(topics)))
			topics = management.ApplyTopicFilters(topics, filters)
			progress.step(fmt.Sprintf("filtered %d", len(topics)))
			sortTopicsByFlags(topics)
			progress.step("sorted")
			topics = applyMaxTopics(topics, cfg)
			progress.step(fmt.Sprintf("ready %d", len(topics)))
			if outPath == "" {
				outPath = filepath.Join(expandDir(cfg.Settings.WorkingDirectory),
					fmt.Sprintf("topics-%s.json", time.Now().Format("20060102-150405")))
			}
			data, err := json.MarshalIndent(topics, "", "  ")
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return err
			}
			fmt.Printf("Exported %d topics to %s\n", len(topics), outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	return cmd
}
