package main

import (
	"fmt"
	"sort"
	"strings"

	"servicebus/internal/config"

	"github.com/spf13/cobra"
)

func buildFavCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fav",
		Aliases: []string{"favorite", "favorites"},
		Short:   "Manage starred queues, topics, and subscriptions for the active context",
		Long: `Favorites bubble to the top of every list, sort, and overview.
Stored per context in the config file.

Examples:
  sb fav add q my-queue
  sb fav add t my-topic
  sb fav add sub my-sub -t my-topic
  sb fav list
  sb fav rm q my-queue
  sb fav clear`,
		RunE: runFavList,
	}
	cmd.AddCommand(buildFavAddCmd(), buildFavRmCmd(), buildFavListCmd(), buildFavClearCmd())
	return cmd
}

func buildFavAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <q|t|sub> <name>",
		Short: "Add a queue, topic, or subscription to favorites",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return mutateFavorites(args[0], args[1], func(f *config.Favorites, kind, name string) error {
				switch kind {
				case "q":
					if !f.HasQueue(name) {
						f.Queues = append(f.Queues, name)
					}
				case "t":
					if !f.HasTopic(name) {
						f.Topics = append(f.Topics, name)
					}
				case "sub":
					key := name // already topic/sub form
					if !contains(f.Subscriptions, key) {
						f.Subscriptions = append(f.Subscriptions, key)
					}
				}
				return nil
			})
		},
	}
}

func buildFavRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <q|t|sub> <name>",
		Aliases: []string{"remove", "delete", "del"},
		Short:   "Remove a queue, topic, or subscription from favorites",
		Args:    cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return mutateFavorites(args[0], args[1], func(f *config.Favorites, kind, name string) error {
				switch kind {
				case "q":
					f.Queues = removeFav(f.Queues, name)
				case "t":
					f.Topics = removeFav(f.Topics, name)
				case "sub":
					f.Subscriptions = removeFav(f.Subscriptions, name)
				}
				return nil
			})
		},
	}
}

func buildFavListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List favorites for the active context",
		RunE:    runFavList,
	}
}

func buildFavClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all favorites for the active context",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, ctxName, err := loadCfgAndCtx()
			if err != nil {
				return err
			}
			c := cfg.Contexts[ctxName]
			c.Favorites = config.Favorites{}
			cfg.Contexts[ctxName] = c
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Cleared favorites for context %q.\n", ctxName)
			return nil
		},
	}
}

func runFavList(_ *cobra.Command, _ []string) error {
	cfg, ctxName, err := loadCfgAndCtx()
	if err != nil {
		return err
	}
	f := cfg.Contexts[ctxName].Favorites
	fmt.Printf("Favorites for context %q:\n", ctxName)
	printFavSection("Queues", f.Queues)
	printFavSection("Topics", f.Topics)
	printFavSection("Subscriptions", f.Subscriptions)
	if len(f.Queues)+len(f.Topics)+len(f.Subscriptions) == 0 {
		fmt.Println("  (none — add some with `sb fav add q|t|sub <name>`)")
	}
	return nil
}

func printFavSection(label string, items []string) {
	if len(items) == 0 {
		return
	}
	cp := append([]string(nil), items...)
	sort.Strings(cp)
	fmt.Printf("  %s (%d):\n", label, len(cp))
	for _, x := range cp {
		fmt.Printf("    ★ %s\n", x)
	}
}

func mutateFavorites(kind, name string, mutate func(*config.Favorites, string, string) error) error {
	kind = strings.ToLower(kind)
	switch kind {
	case "q", "queue":
		kind = "q"
	case "t", "topic":
		kind = "t"
	case "sub", "subscription", "s":
		kind = "sub"
	default:
		return fmt.Errorf("unknown kind %q (expected q, t, or sub)", kind)
	}
	if kind == "sub" {
		// Accept either explicit "topic/sub" or use -t flag for topic.
		if !strings.Contains(name, "/") {
			if gf.topic == "" {
				return fmt.Errorf("subscription favorites need a topic: pass -t <topic> or use topic/sub form")
			}
			name = gf.topic + "/" + name
		}
	}
	cfg, ctxName, err := loadCfgAndCtx()
	if err != nil {
		return err
	}
	c := cfg.Contexts[ctxName]
	if err := mutate(&c.Favorites, kind, name); err != nil {
		return err
	}
	cfg.Contexts[ctxName] = c
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("Updated favorites for context %q.\n", ctxName)
	return nil
}

func removeFav(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// loadCfgAndCtx returns the config and the active context name (creating an
// empty context entry if needed so favorites can be stored).
func loadCfgAndCtx() (*config.Config, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}
	name := gf.context
	if name == "" {
		name = cfg.CurrentContext
	}
	if name == "" {
		if state, err := config.LoadState(); err == nil {
			name = state.Context
		}
	}
	if name == "" {
		return nil, "", fmt.Errorf("no active context — run `sb use <context>` or pass -c <context>")
	}
	if _, ok := cfg.Contexts[name]; !ok {
		return nil, "", fmt.Errorf("context %q not found in config", name)
	}
	return cfg, name, nil
}
