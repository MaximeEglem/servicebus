package main

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

var semverPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+`)

func buildVersionCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display project version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(displayVersion(version))
			if verbose {
				fmt.Printf("full:   %s\n", version)
				fmt.Printf("commit: %s\n", gitCommit)
				fmt.Printf("built:  %s\n", buildDate)
			}
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show full build metadata")
	return cmd
}

func displayVersion(version string) string {
	if match := semverPattern.FindString(version); match != "" {
		return match
	}
	return version
}
