package main

import (
	"fmt"
	"os"

	"servicebus/internal/config"
	"servicebus/internal/format"

	"github.com/spf13/cobra"
)

type globalFlags struct {
	context          string
	connectionString string
	namespace        string
	output           string
	max              int
	all              bool
	sort             string
	filters          []string
	name             string
	topic            string
	quiet            bool
}

var gf globalFlags

func main() {
	root := buildRoot()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "sb",
		Short: "Azure Service Bus CLI",
		Long:  "sb — manage Azure Service Bus queues, topics, subscriptions, and messages.\n\nRun 'sb use <context> [type]' to set a default context and resource type.",
		RunE:  rootDispatch,
	}

	root.PersistentFlags().StringVarP(&gf.context, "context", "c", "", "Named context (overrides active context)")
	root.PersistentFlags().StringVar(&gf.connectionString, "connection-string", "", "Connection string (overrides all other auth)")
	root.PersistentFlags().StringVar(&gf.namespace, "namespace", "", "Azure AD namespace (e.g. ns.servicebus.windows.net)")
	root.PersistentFlags().StringVarP(&gf.output, "output", "o", "table", "Output format: table, json, yaml")
	root.PersistentFlags().BoolVarP(&gf.quiet, "quiet", "q", false, "Suppress progress bars and informational stderr output (auto-on when stderr is not a TTY)")

	root.Flags().IntVarP(&gf.max, "max", "m", 0, "Max results (0 = use MaxItems setting)")
	root.Flags().BoolVarP(&gf.all, "all", "a", false, "Return all results")
	root.Flags().StringVarP(&gf.sort, "sort", "s", "", "Sort: <Field>:asc|dsc")
	root.Flags().StringArrayVarP(&gf.filters, "filter", "f", nil, "Filter (repeatable)")
	root.Flags().StringVarP(&gf.name, "name", "n", "", "Resource name")
	root.Flags().StringVarP(&gf.topic, "topic", "t", "", "Topic name")
	root.Flags().Bool("settings", false, "Show settings (alias for 'sb config')")

	root.AddCommand(
		buildVersionCmd(),
		buildOverviewCmd(),
		buildUseCmd(),
		buildConfigCmd(),
		buildQueueCmd(),
		buildTopicCmd(),
		buildSubCmd(),
		buildRuleCmd(),
		buildDLQCmd(),
		buildDLSCmd(),
		buildSchemaCmd(),
		buildDocsCmd(),
		buildMCPCmd(),
	)

	return root
}

func rootDispatch(cmd *cobra.Command, args []string) error {
	if s, _ := cmd.Flags().GetBool("settings"); s {
		return runConfigShow(cmd, args)
	}

	state, err := config.LoadState()
	if err != nil || (state.Context == "" && state.Type == "") {
		return cmd.Help()
	}

	if gf.context == "" && state.Context != "" {
		gf.context = state.Context
	}

	switch state.Type {
	case "q":
		return runQueueList(cmd, args)
	case "t":
		return runTopicList(cmd, args)
	case "sub":
		return runSubList(cmd, args)
	case "dlq":
		return runDLQList(cmd, args)
	case "dls":
		return runDLSList(cmd, args)
	default:
		return cmd.Help()
	}
}

func resolveFormat() (format.OutputFormat, error) {
	return format.ParseFormat(gf.output)
}

// suppress unused import
var _ = fmt.Sprintf
