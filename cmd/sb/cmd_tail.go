package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"servicebus/pkg/messaging"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func buildTailCmd() *cobra.Command {
	var interval time.Duration
	var fromSeq int64
	var since time.Duration
	var dlq bool
	var batch int
	var bodyOnly bool
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Follow new messages on a queue, subscription, or DLQ (read-only, peek)",
		Long: `Continuously peek for new messages on a queue or subscription.
Uses the Azure SDK PeekMessages API: messages are NEVER locked, consumed, or
delivered to your real consumers — safe to run against production.

Defaults to peeking from the start of the entity. Use --from-seq or --since to
position the cursor closer to the tip on busy entities.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gf.name == "" {
				return fmt.Errorf("--name / -n is required")
			}
			cred, err := resolveCred()
			if err != nil {
				return err
			}
			target := messaging.PeekTarget{
				Queue:        "",
				Topic:        gf.topic,
				Subscription: "",
				DeadLetter:   dlq,
			}
			if gf.topic != "" {
				target.Subscription = gf.name
			} else {
				target.Queue = gf.name
			}
			peeker, err := messaging.NewPeeker(cred, target)
			if err != nil {
				return err
			}
			defer peeker.Close(context.Background())

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			cursor := fromSeq
			startWall := time.Now().Add(-since)
			useTimeFilter := since > 0
			tty := isStderrTTY()

			label := tailLabel(target)
			if !gf.quiet {
				fmt.Fprintf(os.Stderr, "tailing %s (interval=%s, ctrl-c to stop)\n", label, interval)
			}

			for {
				msgs, err := peeker.PeekFrom(ctx, cursor, batch)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					if !gf.quiet {
						fmt.Fprintf(os.Stderr, "peek error: %v (retrying in %s)\n", err, interval)
					}
				}
				for _, m := range msgs {
					if useTimeFilter && m.EnqueuedAt.Before(startWall) {
						continue
					}
					printTailMessage(m, tty, bodyOnly)
				}
				if len(msgs) > 0 {
					last := msgs[len(msgs)-1].SequenceNumber
					if last+1 > cursor {
						cursor = last + 1
					}
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().StringVarP(&gf.name, "name", "n", "", "Queue name, or subscription name when -t is set")
	cmd.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name (when tailing a subscription)")
	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Second, "Polling interval")
	cmd.Flags().Int64Var(&fromSeq, "from-seq", 0, "Start from this sequence number (0 = beginning)")
	cmd.Flags().DurationVar(&since, "since", 0, "Only show messages enqueued in the last duration (e.g. 5m, 1h)")
	cmd.Flags().BoolVar(&dlq, "dlq", false, "Tail the dead-letter sub-queue instead of the main entity")
	cmd.Flags().IntVar(&batch, "batch", 250, "Max messages fetched per poll")
	cmd.Flags().BoolVar(&bodyOnly, "body-only", false, "Print only the message body, one per line")
	return cmd
}

func tailLabel(t messaging.PeekTarget) string {
	suffix := ""
	if t.DeadLetter {
		suffix = " [DLQ]"
	}
	if t.Queue != "" {
		return "queue:" + t.Queue + suffix
	}
	return "topic:" + t.Topic + "/sub:" + t.Subscription + suffix
}

func printTailMessage(m messaging.Message, tty, bodyOnly bool) {
	if bodyOnly {
		fmt.Println(m.Body)
		return
	}
	cyan := color.New(color.FgHiCyan).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	ts := m.EnqueuedAt.Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("[%s] seq=%d id=%s", ts, m.SequenceNumber, m.MessageID)
	if tty {
		header = dim("[") + cyan(ts) + dim("] seq=") + cyan(fmt.Sprintf("%d", m.SequenceNumber)) +
			dim(" id=") + m.MessageID
	}
	if m.DeadLetterReason != "" {
		reason := m.DeadLetterReason
		if tty {
			reason = red(reason)
		}
		header += " " + reason
		if m.DeadLetterErrorDescription != "" {
			desc := m.DeadLetterErrorDescription
			if tty {
				desc = yellow(desc)
			}
			header += ": " + desc
		}
	}
	fmt.Println(header)
	fmt.Println(formatBodyForTail(m.Body))
}

func formatBodyForTail(body string) string {
	// Try to pretty-print JSON, otherwise echo raw.
	var v any
	if err := json.Unmarshal([]byte(body), &v); err == nil {
		if pretty, err := json.MarshalIndent(v, "  ", "  "); err == nil {
			return "  " + string(pretty)
		}
	}
	return "  " + body
}

func isStderrTTY() bool {
	st, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}
