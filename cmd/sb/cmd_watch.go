package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func buildWatchCmd() *cobra.Command {
	var interval time.Duration
	var noDiff bool
	var noClear bool
	cmd := &cobra.Command{
		Use:                "watch [-- sb-args...]",
		Short:              "Re-run an sb command at intervals with line-level diff highlighting",
		DisableFlagParsing: false,
		Long: `Repeatedly execute an sb sub-command and redraw its output every --interval.

Lines added since the previous run are shown in green, removed lines in red,
unchanged lines are dimmed. Pass the wrapped command after a literal "--" to
avoid flag parsing collisions, e.g.:

  sb watch -- overview -c prod
  sb watch --interval 10s -- dlq -c prod
  sb watch -- dlq stats -c prod -n orders --top 5
  sb watch -- q -c prod -s ActiveMessages:dsc -m 10

Read-only operations are recommended; sb watch will happily re-run anything,
including destructive commands. Don't do that.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("provide an sb sub-command after --, e.g. sb watch -- overview -c prod")
			}
			self, err := os.Executable()
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			tty := isStderrTTY()
			useDiff := !noDiff && tty
			useClear := !noClear && tty

			var prev []string
			iter := 0
			for {
				iter++
				out, runErr := runWatchCommand(ctx, self, args)
				if useClear {
					fmt.Print("\033[H\033[2J") // clear screen + cursor home
				}
				header := fmt.Sprintf("Every %s: sb %s   (#%d  %s)",
					interval, strings.Join(args, " "), iter, time.Now().Format("15:04:05"))
				if tty {
					header = color.New(color.FgHiCyan, color.Bold).Sprint(header)
				}
				fmt.Println(header)
				fmt.Println()
				lines := splitLines(out)
				if useDiff && prev != nil {
					printDiff(prev, lines)
				} else {
					for _, l := range lines {
						fmt.Println(l)
					}
				}
				if runErr != nil {
					if ctx.Err() != nil {
						return nil
					}
					if tty {
						fmt.Fprintln(os.Stderr, color.New(color.FgRed).Sprintf("[command error] %v", runErr))
					} else {
						fmt.Fprintf(os.Stderr, "[command error] %v\n", runErr)
					}
				}
				prev = lines

				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().DurationVarP(&interval, "interval", "i", 5*time.Second, "Re-run interval")
	cmd.Flags().BoolVar(&noDiff, "no-diff", false, "Disable diff coloring; print raw output each iteration")
	cmd.Flags().BoolVar(&noClear, "no-clear", false, "Don't clear the screen between iterations")
	return cmd
}

func runWatchCommand(ctx context.Context, self string, args []string) (string, error) {
	c := exec.CommandContext(ctx, self, args...)
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	c.Env = append(os.Environ(), "NO_COLOR=1") // child output is redrawn by parent
	err := c.Run()
	return buf.String(), err
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimRight(s, "\n")
	return strings.Split(s, "\n")
}

func printDiff(prev, curr []string) {
	added := color.New(color.FgGreen).SprintFunc()
	removed := color.New(color.FgRed).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()

	prevSet := make(map[string]int, len(prev))
	for _, l := range prev {
		prevSet[l]++
	}
	currSet := make(map[string]int, len(curr))
	for _, l := range curr {
		currSet[l]++
	}

	for _, l := range curr {
		if prevSet[l] > 0 {
			fmt.Println(dim("  " + l))
			prevSet[l]--
		} else {
			fmt.Println(added("+ " + l))
		}
	}
	// Show removed lines (anything that was in prev but no longer present).
	for _, l := range prev {
		if currSet[l] > 0 {
			currSet[l]--
			continue
		}
		fmt.Println(removed("- " + l))
	}
}
