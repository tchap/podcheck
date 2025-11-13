package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// CheckFunc is a function that checks a pod and returns output if it matches criteria
type CheckFunc func(ns *corev1.Namespace, pod *corev1.Pod) (string, error)

// PodChecker provides the core functionality for checking pods
type PodChecker struct {
	clientset *kubernetes.Clientset
}

// NewPodChecker creates a new PodChecker using the current kubeconfig
func NewPodChecker() (*PodChecker, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &PodChecker{clientset: clientset}, nil
}

// RunCheck fetches all pods and namespaces, then applies the check function
func (pc *PodChecker) RunCheck(ctx context.Context, checkFn CheckFunc) error {
	// Fetch all namespaces
	namespaces, err := pc.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Create a map of namespace name to namespace object for quick lookup
	nsMap := make(map[string]*corev1.Namespace)
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		nsMap[ns.Name] = ns
	}

	// Fetch all pods
	pods, err := pc.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Iterate through pods and apply check function
	for i := range pods.Items {
		pod := &pods.Items[i]
		ns := nsMap[pod.Namespace]
		if ns == nil {
			fmt.Fprintf(os.Stderr, "Warning: namespace %s not found for pod %s\n", pod.Namespace, pod.Name)
			continue
		}

		output, err := checkFn(ns, pod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking pod %s/%s: %v\n", pod.Namespace, pod.Name, err)
			continue
		}
		if output != "" {
			fmt.Println(output)
		}
	}

	return nil
}
