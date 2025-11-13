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

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	headers := []string{"NAMESPACE", "POD", "ACTION"}
	checker, err := NewPodChecker(podsFile, namespacesFile, headers)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return checker.RunCheck(ctx, checkHostUsers, verbose)
}

func checkHostUsers(ns *corev1.Namespace, pod *corev1.Pod, verboseEnabled bool) (string, error) {
	verbose := func(actionFormat string, args ...any) string {
		if verboseEnabled {
			action := fmt.Sprintf(actionFormat, args...)
			return fmt.Sprintf("%s\t%s\t%s", ns.Name, pod.Name, action)
		}
		return ""
	}

	// Skip pods having hostUsers: false.
	if pod.Spec.HostUsers != nil && !*pod.Spec.HostUsers {
		// Pod has hostUsers: false, skip it
		return verbose("Done (hostUsers: false)"), nil
	}

	// Skip pods with host*: true.
	if pod.Spec.HostNetwork {
		return verbose("Unavailable (hostNetwork: true)"), nil
	}
	if pod.Spec.HostIPC {
		return verbose("Unavailable (hostIPC: true)"), nil
	}
	if pod.Spec.HostPID {
		return verbose("Unavailable (hostPID: true)"), nil
	}

	if pod.Spec.SecurityContext != nil {
		// Skip pods running as root.
		if pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser == 0 {
			return verbose("Unavailable (runAsUser: 0)"), nil
		}
		// Skip pods with privileged containers.
		for _, c := range pod.Spec.Containers {
			if c.SecurityContext == nil {
				continue
			}
			if c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser == 0 {
				return verbose("Unavailable (container %s: runAsUser: 0)", c.Name), nil
			}
			if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
				return verbose("Unavailable (container %s: privileged: true)", c.Name), nil
			}
		}
	}

	var action string
	if ns.Labels["openshift.io/run-level"] == "" {
		action = "Use restricted-v3"
	} else {
		action = "Mimic restricted-v3"
	}

	// Pod doesn't have hostUsers: false, include it in output
	// Use tab separator for tabwriter formatting
	return fmt.Sprintf("%s\t%s\t%s", ns.Name, pod.Name, action), nil
}
