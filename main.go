package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "podcheck",
		Short: "A utility to check and filter Kubernetes pods",
		Long: `podcheck is a CLI utility that helps you check and filter pods in Kubernetes clusters.
It provides various subcommands to identify pods based on specific criteria.`,
	}

	// Add subcommands
	rootCmd.AddCommand(newUsernsCmd())

	return rootCmd
}
