package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

func newUsernsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "userns",
		Short: "List pods that are eligible for using user namespaces",
		Long:  "This command lists all pods that are eligible for using user namespaces.",
		RunE:  runUserns,
	}
}

func runUserns(cmd *cobra.Command, args []string) error {
	podsFile, err := cmd.Flags().GetString("pods")
	if err != nil {
		return err
	}

	namespacesFile, err := cmd.Flags().GetString("namespaces")
	if err != nil {
		return err
	}

	headers := []string{"NAMESPACE", "POD"}
	checker, err := NewPodChecker(podsFile, namespacesFile, headers)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return checker.RunCheck(ctx, checkHostUsers)
}

func checkHostUsers(ns *corev1.Namespace, pod *corev1.Pod) (string, error) {
	// Skip pods having hostUsers: false.
	if pod.Spec.HostUsers != nil && !*pod.Spec.HostUsers {
		// Pod has hostUsers: false, skip it
		return "", nil
	}

	// Pod doesn't have hostUsers: false, include it in output
	// Use tab separator for tabwriter formatting
	return fmt.Sprintf("%s\t%s", ns.Name, pod.Name), nil
}
