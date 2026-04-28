package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"servicebus/internal/config"
	"servicebus/internal/format"
	"servicebus/pkg/auth"
	"servicebus/pkg/management"
	"servicebus/pkg/messaging"

	"github.com/spf13/cobra"
)

func buildQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "q",
		Short: "Manage queues",
		RunE:  runQueueList,
	}
	cmd.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results")
	cmd.Flags().BoolVarP(&gf.all, "all", "a", false, "All results")
	cmd.Flags().StringVarP(&gf.sort, "sort", "s", "", "<Field>:asc|dsc")
	cmd.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Queue name (shows info)")
	cmd.AddCommand(buildQueueExportCmd(), buildQueueSendCmd(), buildQueueCompareCmd())
	return cmd
}

func runQueueList(cmd *cobra.Command, _ []string) error {
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	progress := newListProgress("queues", 4)
	filters, err := parseFilters(gf.filters)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if gf.name != "" {
		q, err := mgr.GetQueue(ctx, gf.name)
		if err != nil {
			return err
		}
		return printResult(cmd, []management.QueueInfo{*q}, cfg)
	}
	queues, err := mgr.ListQueues(ctx, management.ListOptions{})
	if err != nil {
		return err
	}
	progress.step(fmt.Sprintf("loaded %d", len(queues)))
	queues = management.ApplyQueueFilters(queues, filters)
	progress.step(fmt.Sprintf("filtered %d", len(queues)))
	sortQueuesByFlags(queues)
	progress.step("sorted")
	queues = applyMax(queues, cfg)
	progress.step(fmt.Sprintf("ready %d", len(queues)))
	return printResult(cmd, queues, cfg)
}

func buildQueueExportCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "x",
		Short: "Export queue list to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, err := newManager()
			if err != nil {
				return err
			}
			progress := newListProgress("queues", 4)
			filters, err := parseFilters(gf.filters)
			if err != nil {
				return err
			}
			queues, err := mgr.ListQueues(context.Background(), management.ListOptions{})
			if err != nil {
				return err
			}
			progress.step(fmt.Sprintf("loaded %d", len(queues)))
			queues = management.ApplyQueueFilters(queues, filters)
			progress.step(fmt.Sprintf("filtered %d", len(queues)))
			sortQueuesByFlags(queues)
			progress.step("sorted")
			queues = applyMax(queues, cfg)
			progress.step(fmt.Sprintf("ready %d", len(queues)))
			if outPath == "" {
				outPath = filepath.Join(expandDir(cfg.Settings.WorkingDirectory),
					fmt.Sprintf("queues-%s.json", time.Now().Format("20060102-150405")))
			}
			data, err := json.MarshalIndent(queues, "", "  ")
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return err
			}
			fmt.Printf("Exported %d queues to %s\n", len(queues), outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path")
	return cmd
}

func buildQueueSendCmd() *cobra.Command {
	var path string
	var count, delayMins int
	cmd := &cobra.Command{
		Use:   "s",
		Short: "Send messages to a queue",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required for send")
			}
			cred, err := resolveCred()
			if err != nil {
				return err
			}
			sender, err := messaging.NewSender(cred, gf.name)
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

func buildQueueCompareCmd() *cobra.Command {
	var issues bool
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare queues across all registered contexts",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			filters, _ := parseFilters(gf.filters)

			type ctxResult struct {
				name   string
				queues []management.QueueInfo
			}
			resCh := make(chan ctxResult, len(cfg.Contexts))
			var wg sync.WaitGroup

			for ctxName, ctxCfg := range cfg.Contexts {
				wg.Add(1)
				go func(ctxName string, ctxCfg config.Context) {
					defer wg.Done()
					cred, err := auth.Resolve(auth.Options{
						ConnectionString: ctxCfg.ConnectionString,
						Namespace:        ctxCfg.Namespace,
						Config:           cfg,
					})
					if err != nil {
						fmt.Fprintf(os.Stderr, "skipping %s: %v\n", ctxName, err)
						return
					}
					mgr, err := management.NewManager(cred)
					if err != nil {
						fmt.Fprintf(os.Stderr, "skipping %s: %v\n", ctxName, err)
						return
					}
					queues, err := mgr.ListQueues(context.Background(), management.ListOptions{RuntimeOnly: true})
					if err != nil {
						fmt.Fprintf(os.Stderr, "skipping %s: %v\n", ctxName, err)
						return
					}
					queues = management.ApplyQueueFilters(queues, filters)
					if gf.name != "" {
						var filtered []management.QueueInfo
						for _, q := range queues {
							if q.Name == gf.name {
								filtered = append(filtered, q)
							}
						}
						queues = filtered
					}
					resCh <- ctxResult{ctxName, queues}
				}(ctxName, ctxCfg)
			}
			wg.Wait()
			close(resCh)

			contextQueues := map[string][]management.QueueInfo{}
			for r := range resCh {
				contextQueues[r.name] = r.queues
			}

			results := management.CompareQueues(contextQueues)
			if issues {
				var filtered []management.QueueCompareResult
				for _, r := range results {
					if r.HasIssue {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
			f, err := resolveFormat()
			if err != nil {
				return err
			}
			p := format.NewPrinter(f, os.Stdout, format.ThresholdConfig{})
			return p.Print(results)
		},
	}
	cmd.Flags().BoolVar(&issues, "issues", false, "Show only queues with differences")
	return cmd
}

// — shared helpers —

func newManager() (management.Manager, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	cred, err := resolveCredWithConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	mgr, err := management.NewManager(cred)
	if err != nil {
		return nil, nil, err
	}
	return mgr, cfg, nil
}

func resolveCred() (*auth.Credentials, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return resolveCredWithConfig(cfg)
}

func resolveCredWithConfig(cfg *config.Config) (*auth.Credentials, error) {
	ctxName := gf.context
	if ctxName == "" {
		ctxName = cfg.CurrentContext
	}
	if ctxName == "" {
		if state, err := config.LoadState(); err == nil {
			ctxName = state.Context
		}
	}
	return auth.Resolve(auth.Options{
		ConnectionString: gf.connectionString,
		Namespace:        gf.namespace,
		ContextName:      ctxName,
		Config:           cfg,
	})
}

func parseFilters(raw []string) ([]management.Filter, error) {
	var filters []management.Filter
	for _, s := range raw {
		f, err := management.ParseFilter(s)
		if err != nil {
			return nil, fmt.Errorf("invalid filter %q: %w", s, err)
		}
		filters = append(filters, f)
	}
	return filters, nil
}

func parseSortFlag(s string) (field, dir string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return s[:i], s[i+1:]
		}
	}
	return s, "asc"
}

func sortQueuesByFlags(queues []management.QueueInfo) {
	if gf.sort != "" {
		field, dir := parseSortFlag(gf.sort)
		management.SortQueues(queues, field, dir)
		return
	}
	management.SortQueues(queues, "Name", "asc")
}

func sortTopicsByFlags(topics []management.TopicInfo) {
	if gf.sort != "" {
		field, dir := parseSortFlag(gf.sort)
		management.SortTopics(topics, field, dir)
		return
	}
	management.SortTopics(topics, "Name", "asc")
}

func sortSubscriptionsByFlags(subs []management.SubscriptionInfo) {
	if gf.sort != "" {
		field, dir := parseSortFlag(gf.sort)
		management.SortSubscriptions(subs, field, dir)
		return
	}
	management.SortSubscriptions(subs, "Name", "asc")
}

type listProgress struct {
	label   string
	total   int
	current int
	enabled bool
}

func newListProgress(label string, total int) *listProgress {
	if total <= 0 {
		total = 1
	}
	enabled := false
	if stat, err := os.Stderr.Stat(); err == nil {
		enabled = (stat.Mode() & os.ModeCharDevice) != 0
	}
	return &listProgress{label: label, total: total, enabled: enabled}
}

func (p *listProgress) step(state string) {
	if !p.enabled {
		return
	}
	if p.current < p.total {
		p.current++
	}
	width := 20
	filled := (p.current * width) / p.total
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	fmt.Fprintf(os.Stderr, "\r%s [%s] %d/%d %s", p.label, bar, p.current, p.total, state)
	if p.current >= p.total {
		fmt.Fprintln(os.Stderr)
	}
}

func applyMax(queues []management.QueueInfo, cfg *config.Config) []management.QueueInfo {
	if gf.all {
		return queues
	}
	limit := gf.max
	if limit <= 0 {
		limit = cfg.Settings.MaxItems
	}
	if limit > 0 && len(queues) > limit {
		return queues[:limit]
	}
	return queues
}

func applyMaxTopics(topics []management.TopicInfo, cfg *config.Config) []management.TopicInfo {
	if gf.all {
		return topics
	}
	limit := gf.max
	if limit <= 0 {
		limit = cfg.Settings.MaxItems
	}
	if limit > 0 && len(topics) > limit {
		return topics[:limit]
	}
	return topics
}

func applyMaxSubs(subs []management.SubscriptionInfo, cfg *config.Config) []management.SubscriptionInfo {
	if gf.all {
		return subs
	}
	limit := gf.max
	if limit <= 0 {
		limit = cfg.Settings.MaxItems
	}
	if limit > 0 && len(subs) > limit {
		return subs[:limit]
	}
	return subs
}

func printResult(_ *cobra.Command, data any, cfg *config.Config) error {
	f, err := resolveFormat()
	if err != nil {
		return err
	}
	p := format.NewPrinter(f, os.Stdout, format.ThresholdConfig{
		SizePercentage: cfg.Settings.Thresholds.SizePercentage,
		LastAccessDays: cfg.Settings.Thresholds.LastAccessDays,
	})
	return p.Print(data)
}

func expandDir(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func exportJSON(data any, outPath string) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, b, 0644); err != nil {
		return err
	}
	fmt.Printf("Exported to %s\n", outPath)
	return nil
}
