package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "sb",
		Short: "Azure Service Bus CLI",
		Long:  "sb — manage Azure Service Bus queues, topics, subscriptions, and messages.",
	}
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
