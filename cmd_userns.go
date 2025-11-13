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

	headers := []string{"NAMESPACE", "POD", "SCC ENABLED"}
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

	// Skip pods with host*: true.
	if pod.Spec.HostNetwork || pod.Spec.HostPID || pod.Spec.HostIPC {
		return "", nil
	}

	if pod.Spec.SecurityContext != nil {
		// Skip pods running as root.
		if pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser == 0 {
			return "", nil
		}
		// Skip pods with privileged containers.
		for _, c := range pod.Spec.Containers {
			if c.SecurityContext == nil {
				continue
			}
			if c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser == 0 {
				return "", nil
			}
			if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
				return "", nil
			}
		}
	}

	sccEnabled := ns.Labels["openshift.io/run-level"] == ""

	// Pod doesn't have hostUsers: false, include it in output
	// Use tab separator for tabwriter formatting
	return fmt.Sprintf("%s\t%s\t%v", ns.Name, pod.Name, sccEnabled), nil
}
