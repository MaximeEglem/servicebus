package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"servicebus/pkg/management"
)

func buildOverviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "Show namespace metrics: queue, topic and subscription counts",
		RunE:  runOverview,
	}
}

type namespaceMetrics struct {
	Queues              int
	QueuesWithDLQ       int
	TotalDLQMessages    int64
	Topics              int
	Subscriptions       int
	SubsWithDLQ         int
	TotalSubDLQMessages int64
}

func runOverview(_ *cobra.Command, _ []string) error {
	mgr, cfg, err := newManager()
	if err != nil {
		return err
	}
	_ = cfg

	ctx := context.Background()

	// Fetch queues and topics in parallel; subscriptions need topic list first.
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
		q, err := mgr.ListQueues(ctx, management.ListOptions{})
		qCh <- queueResult{q, err}
	}()
	go func() {
		t, err := mgr.ListTopics(ctx, management.ListOptions{})
		tCh <- topicResult{t, err}
	}()

	qr := <-qCh
	if qr.err != nil {
		return fmt.Errorf("listing queues: %w", qr.err)
	}
	tr := <-tCh
	if tr.err != nil {
		return fmt.Errorf("listing topics: %w", tr.err)
	}

	// Fetch subscriptions for all topics in parallel.
	type subResult struct {
		subs []management.SubscriptionInfo
		err  error
	}
	subCh := make(chan subResult, len(tr.topics))
	var wg sync.WaitGroup
	for _, t := range tr.topics {
		wg.Add(1)
		go func(topicName string) {
			defer wg.Done()
			subs, err := mgr.ListSubscriptions(ctx, topicName, management.ListOptions{})
			subCh <- subResult{subs, err}
		}(t.Name)
	}
	go func() {
		wg.Wait()
		close(subCh)
	}()

	var allSubs []management.SubscriptionInfo
	for sr := range subCh {
		if sr.err != nil {
			continue // best-effort; count what we can
		}
		allSubs = append(allSubs, sr.subs...)
	}

	// Compute metrics.
	m := namespaceMetrics{
		Queues:        len(qr.queues),
		Topics:        len(tr.topics),
		Subscriptions: len(allSubs),
	}
	for _, q := range qr.queues {
		if q.DeadLetterMessages > 0 {
			m.QueuesWithDLQ++
			m.TotalDLQMessages += q.DeadLetterMessages
		}
	}
	for _, s := range allSubs {
		if s.DeadLetterMessages > 0 {
			m.SubsWithDLQ++
			m.TotalSubDLQMessages += s.DeadLetterMessages
		}
	}

	printOverview(m)
	return nil
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
	if m.SubsWithDLQ > 0 {
		fmt.Printf("  %-28s  %s  (%s total DLQ messages)\n",
			label("  └─ with dead letters"),
			warn(fmt.Sprintf("%d", m.SubsWithDLQ)),
			bad(fmt.Sprintf("%d", m.TotalSubDLQMessages)),
		)
	} else {
		fmt.Printf("  %-28s  %s\n", label("  └─ with dead letters"), ok("0"))
	}
	fmt.Println()
}
