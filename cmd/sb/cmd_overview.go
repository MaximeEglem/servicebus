package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"servicebus/internal/config"
	"servicebus/internal/format"
	"servicebus/pkg/management"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func buildOverviewCmd() *cobra.Command {
	var deep bool
	var useCache bool
	var cacheTTL time.Duration
	var top int
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Show namespace metrics: queue, topic and subscription counts",
		RunE: func(c *cobra.Command, args []string) error {
			return runOverview(c, args, deep, useCache, cacheTTL, top)
		},
	}
	cmd.Flags().BoolVar(&deep, "deep", false, "Fetch subscription DLQ details (slower)")
	cmd.Flags().BoolVar(&useCache, "cache", false, "Use cached overview if fresh; persist result to disk")
	cmd.Flags().DurationVar(&cacheTTL, "cache-ttl", 5*time.Minute, "Cache freshness window for --cache")
	cmd.Flags().IntVar(&top, "top", 0, "Also show top-N tables of queues, topics and subscriptions (0 = off, no extra HTTP calls)")
	return cmd
}

type namespaceMetrics struct {
	Queues           int
	QueuesWithDLQ    int
	TotalDLQMessages int64
	Topics           int
	Subscriptions    int
	// Deep mode only:
	SubsWithDLQ         int
	TotalSubDLQMessages int64
	DeepMode            bool
}

func runOverview(_ *cobra.Command, _ []string, deep, useCache bool, cacheTTL time.Duration, top int) error {
	ctxName := gf.context
	if ctxName == "" {
		if cfg, err := config.Load(); err == nil {
			ctxName = cfg.CurrentContext
		}
	}
	cacheFile := overviewCachePath(ctxName, deep)
	if useCache && top == 0 {
		if m, ok := loadOverviewCache(cacheFile, cacheTTL); ok {
			printOverview(m)
			if !gf.quiet {
				fmt.Fprintf(os.Stderr, "(served from cache: %s)\n", cacheFile)
			}
			return nil
		}
	}

	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	_ = cfg

	ctx := context.Background()

	spin := newSpinner("Loading namespace metrics…")
	spin.start()

	type queueResult struct {
		queues []management.QueueInfo
		err    error
	}
	type topicResult struct {
		topics []management.TopicInfo
		err    error
	}

	qCh := make(chan queueResult, 1)
	tCh := make(chan topicResult, 1)

	go func() {
		q, err := mgr.ListQueues(ctx, management.ListOptions{RuntimeOnly: true})
		qCh <- queueResult{q, err}
	}()
	go func() {
		t, err := mgr.ListTopics(ctx, management.ListOptions{RuntimeOnly: true})
		tCh <- topicResult{t, err}
	}()

	qr := <-qCh
	if qr.err != nil {
		spin.stop()
		return fmt.Errorf("listing queues: %w", qr.err)
	}
	tr := <-tCh
	if tr.err != nil {
		spin.stop()
		return fmt.Errorf("listing topics: %w", tr.err)
	}

	m := namespaceMetrics{
		Queues:   len(qr.queues),
		Topics:   len(tr.topics),
		DeepMode: deep,
	}
	for _, q := range qr.queues {
		if q.DeadLetterMessages > 0 {
			m.QueuesWithDLQ++
			m.TotalDLQMessages += q.DeadLetterMessages
		}
	}

	if !deep {
		// Fast path: SubscriptionCount is already in TopicRuntimeProperties — no extra HTTP calls.
		var total int32
		for _, t := range tr.topics {
			total += t.SubscriptionCount
		}
		m.Subscriptions = int(total)
		spin.stop()
		printOverview(m)
		fav := activeFavoritesNoErr()
		printFavoritesOverview(qr.queues, tr.topics, nil, fav)
		if top > 0 {
			printTopTables(qr.queues, tr.topics, nil, top, fav)
		}
	} else {
		// Deep path: fetch all subscriptions with a semaphore to avoid throttling Azure.
		const concurrency = 20
		sem := make(chan struct{}, concurrency)
		total := len(tr.topics)

		type subResult struct {
			subs []management.SubscriptionInfo
			err  error
		}
		subCh := make(chan subResult, total)
		var wg sync.WaitGroup
		var doneMu sync.Mutex
		doneCount := 0

		for _, t := range tr.topics {
			wg.Add(1)
			go func(topicName string) {
				defer wg.Done()
				sem <- struct{}{}
				subs, err := mgr.ListSubscriptions(ctx, topicName, management.ListOptions{})
				<-sem
				subCh <- subResult{subs, err}
				doneMu.Lock()
				doneCount++
				d := doneCount
				doneMu.Unlock()
				spin.update(fmt.Sprintf("Loading subscriptions… %d/%d topics", d, total))
			}(t.Name)
		}
		go func() {
			wg.Wait()
			close(subCh)
		}()

		var allSubs []management.SubscriptionInfo
		for sr := range subCh {
			if sr.err != nil {
				continue
			}
			m.Subscriptions += len(sr.subs)
			allSubs = append(allSubs, sr.subs...)
			for _, s := range sr.subs {
				if s.DeadLetterMessages > 0 {
					m.SubsWithDLQ++
					m.TotalSubDLQMessages += s.DeadLetterMessages
				}
			}
		}
		spin.stop()
		printOverview(m)
		fav := activeFavoritesNoErr()
		printFavoritesOverview(qr.queues, tr.topics, allSubs, fav)
		if top > 0 {
			printTopTables(qr.queues, tr.topics, allSubs, top, fav)
		}
	}

	if useCache && top == 0 {
		if err := saveOverviewCache(cacheFile, m); err != nil && !gf.quiet {
			fmt.Fprintf(os.Stderr, "warning: cache write failed: %v\n", err)
		}
	}
	return nil
}

func printTopTables(queues []management.QueueInfo, topics []management.TopicInfo, subs []management.SubscriptionInfo, n int, fav config.Favorites) {
	p := format.NewPrinter(format.Table, os.Stdout, format.ThresholdConfig{})

	// Top queues by DLQ count (only those with DLQ > 0), then by ActiveMessages.
	dlq := make([]management.QueueInfo, 0, len(queues))
	for _, q := range queues {
		if q.DeadLetterMessages > 0 {
			dlq = append(dlq, q)
		}
	}
	sort.Slice(dlq, func(i, j int) bool { return dlq[i].DeadLetterMessages > dlq[j].DeadLetterMessages })
	if len(dlq) > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("Top %d queues by dead letters", min(n, len(dlq)))))
		rows := headTopQueues(dlq, n)
		bubbleQueueFavorites(rows, fav)
		markQueueFavoritesForTable(rows, fav)
		_ = p.Print(rows)
	}

	active := append([]management.QueueInfo(nil), queues...)
	sort.Slice(active, func(i, j int) bool { return active[i].ActiveMessages > active[j].ActiveMessages })
	if len(active) > 0 && active[0].ActiveMessages > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("Top %d queues by active messages", min(n, len(active)))))
		rows := headTopQueues(active, n)
		bubbleQueueFavorites(rows, fav)
		markQueueFavoritesForTable(rows, fav)
		_ = p.Print(rows)
	}

	// Top topics by SubscriptionCount, then by Size.
	bySubs := append([]management.TopicInfo(nil), topics...)
	sort.Slice(bySubs, func(i, j int) bool { return bySubs[i].SubscriptionCount > bySubs[j].SubscriptionCount })
	if len(bySubs) > 0 && bySubs[0].SubscriptionCount > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("Top %d topics by subscription count", min(n, len(bySubs)))))
		rows := headTopTopics(bySubs, n)
		bubbleTopicFavorites(rows, fav)
		markTopicFavoritesForTable(rows, fav)
		_ = p.Print(rows)
	}

	bySize := append([]management.TopicInfo(nil), topics...)
	sort.Slice(bySize, func(i, j int) bool { return bySize[i].SizeInMegabytes > bySize[j].SizeInMegabytes })
	if len(bySize) > 0 && bySize[0].SizeInMegabytes > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("Top %d topics by size", min(n, len(bySize)))))
		rows := headTopTopics(bySize, n)
		bubbleTopicFavorites(rows, fav)
		markTopicFavoritesForTable(rows, fav)
		_ = p.Print(rows)
	}

	// Top subscriptions by DLQ (only available with --deep).
	if len(subs) > 0 {
		dlqSubs := make([]management.SubscriptionInfo, 0, len(subs))
		for _, s := range subs {
			if s.DeadLetterMessages > 0 {
				dlqSubs = append(dlqSubs, s)
			}
		}
		sort.Slice(dlqSubs, func(i, j int) bool { return dlqSubs[i].DeadLetterMessages > dlqSubs[j].DeadLetterMessages })
		if len(dlqSubs) > 0 {
			fmt.Println()
			fmt.Println(headerLine(fmt.Sprintf("Top %d subscriptions by dead letters", min(n, len(dlqSubs)))))
			rows := headTopSubs(dlqSubs, n)
			bubbleSubFavorites(rows, fav)
			markSubFavoritesForTable(rows, fav)
			_ = p.Print(rows)
		}
	}
}

func headTopQueues(qs []management.QueueInfo, n int) []management.QueueInfo {
	if n > 0 && len(qs) > n {
		qs = qs[:n]
	}
	out := make([]management.QueueInfo, len(qs))
	copy(out, qs)
	return out
}

func headTopTopics(ts []management.TopicInfo, n int) []management.TopicInfo {
	if n > 0 && len(ts) > n {
		ts = ts[:n]
	}
	out := make([]management.TopicInfo, len(ts))
	copy(out, ts)
	return out
}

func headTopSubs(ss []management.SubscriptionInfo, n int) []management.SubscriptionInfo {
	if n > 0 && len(ss) > n {
		ss = ss[:n]
	}
	out := make([]management.SubscriptionInfo, len(ss))
	copy(out, ss)
	return out
}

// printFavoritesOverview renders a single ★ Favorites section with all
// favorited queues, topics, and (when --deep) subscriptions of the active
// context. Uses already-fetched data — zero extra HTTP calls.
func printFavoritesOverview(queues []management.QueueInfo, topics []management.TopicInfo, subs []management.SubscriptionInfo, fav config.Favorites) {
	if len(fav.Queues)+len(fav.Topics)+len(fav.Subscriptions) == 0 {
		return
	}
	p := format.NewPrinter(format.Table, os.Stdout, format.ThresholdConfig{})

	favQ := make([]management.QueueInfo, 0, len(fav.Queues))
	for _, q := range queues {
		if fav.HasQueue(q.Name) {
			favQ = append(favQ, q)
		}
	}
	if len(favQ) > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("★ Favorite queues (%d)", len(favQ))))
		_ = p.Print(favQ)
	}

	favT := make([]management.TopicInfo, 0, len(fav.Topics))
	for _, t := range topics {
		if fav.HasTopic(t.Name) {
			favT = append(favT, t)
		}
	}
	if len(favT) > 0 {
		fmt.Println()
		fmt.Println(headerLine(fmt.Sprintf("★ Favorite topics (%d)", len(favT))))
		_ = p.Print(favT)
	}

	if len(subs) > 0 {
		favS := make([]management.SubscriptionInfo, 0, len(fav.Subscriptions))
		for _, s := range subs {
			if fav.HasSubscription(s.TopicName, s.Name) {
				favS = append(favS, s)
			}
		}
		if len(favS) > 0 {
			fmt.Println()
			fmt.Println(headerLine(fmt.Sprintf("★ Favorite subscriptions (%d)", len(favS))))
			_ = p.Print(favS)
		}
	}
}

func headerLine(s string) string {
	if stat, err := os.Stdout.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		return color.New(color.FgHiCyan, color.Bold).Sprint("  " + s)
	}
	return "  " + s
}

func overviewCachePath(ctxName string, deep bool) string {
	suffix := "shallow"
	if deep {
		suffix = "deep"
	}
	name := ctxName
	if name == "" {
		name = "_default"
	}
	return filepath.Join(config.CacheDir(), fmt.Sprintf("overview-%s-%s.json", name, suffix))
}

type cachedOverview struct {
	CachedAt time.Time        `json:"cached_at"`
	Metrics  namespaceMetrics `json:"metrics"`
}

func loadOverviewCache(path string, ttl time.Duration) (namespaceMetrics, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return namespaceMetrics{}, false
	}
	var c cachedOverview
	if err := json.Unmarshal(data, &c); err != nil {
		return namespaceMetrics{}, false
	}
	if time.Since(c.CachedAt) > ttl {
		return namespaceMetrics{}, false
	}
	return c.Metrics, true
}

func saveOverviewCache(path string, m namespaceMetrics) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	c := cachedOverview{CachedAt: time.Now(), Metrics: m}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func printOverview(m namespaceMetrics) {
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.New(color.FgHiCyan, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	tty := false
	if stat, err := os.Stdout.Stat(); err == nil {
		tty = (stat.Mode() & os.ModeCharDevice) != 0
	}

	label := func(s string) string {
		if tty {
			return cyan(s)
		}
		return s
	}
	hi := func(s string) string {
		if tty {
			return bold(s)
		}
		return s
	}
	warn := func(s string) string {
		if tty {
			return yellow(s)
		}
		return s
	}
	bad := func(s string) string {
		if tty {
			return red(s)
		}
		return s
	}
	ok := func(s string) string {
		if tty {
			return green(s)
		}
		return s
	}

	fmt.Println()
	fmt.Printf("  %s\n", hi("Namespace Overview"))
	fmt.Println()
	fmt.Printf("  %-28s  %s\n", label("Queues"), hi(fmt.Sprintf("%d", m.Queues)))
	if m.QueuesWithDLQ > 0 {
		fmt.Printf("  %-28s  %s  (%s total DLQ messages)\n",
			label("  └─ with dead letters"),
			warn(fmt.Sprintf("%d", m.QueuesWithDLQ)),
			bad(fmt.Sprintf("%d", m.TotalDLQMessages)),
		)
	} else {
		fmt.Printf("  %-28s  %s\n", label("  └─ with dead letters"), ok("0"))
	}
	fmt.Println()
	fmt.Printf("  %-28s  %s\n", label("Topics"), hi(fmt.Sprintf("%d", m.Topics)))
	fmt.Println()
	fmt.Printf("  %-28s  %s\n", label("Subscriptions"), hi(fmt.Sprintf("%d", m.Subscriptions)))
	if m.DeepMode {
		if m.SubsWithDLQ > 0 {
			fmt.Printf("  %-28s  %s  (%s total DLQ messages)\n",
				label("  └─ with dead letters"),
				warn(fmt.Sprintf("%d", m.SubsWithDLQ)),
				bad(fmt.Sprintf("%d", m.TotalSubDLQMessages)),
			)
		} else {
			fmt.Printf("  %-28s  %s\n", label("  └─ with dead letters"), ok("0"))
		}
	} else {
		fmt.Printf("  %-28s  %s\n", label("  └─ tip"), fmt.Sprintf("run with %s for DLQ details", hi("--deep")))
	}
	fmt.Println()
}

// spinner — simple TTY progress indicator written to stderr.
type spinner struct {
	mu      sync.Mutex
	msg     string
	done    chan struct{}
	enabled bool
}

func newSpinner(msg string) *spinner {
	enabled := false
	if !gf.quiet {
		if stat, err := os.Stderr.Stat(); err == nil {
			enabled = (stat.Mode() & os.ModeCharDevice) != 0
		}
	}
	return &spinner{msg: msg, done: make(chan struct{}), enabled: enabled}
}

func (s *spinner) update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

func (s *spinner) start() {
	if !s.enabled {
		return
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				fmt.Fprint(os.Stderr, "\r\033[K") // clear line
				return
			default:
				s.mu.Lock()
				msg := s.msg
				s.mu.Unlock()
				fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

func (s *spinner) stop() {
	if !s.enabled {
		return
	}
	close(s.done)
	time.Sleep(90 * time.Millisecond) // let goroutine clear the line
}
