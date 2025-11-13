package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

// CheckFunc is a function that checks a pod and returns output if it matches criteria
type CheckFunc func(ns *corev1.Namespace, pod *corev1.Pod) (string, error)

// PodChecker provides the core functionality for checking pods
type PodChecker struct {
	clientset     *kubernetes.Clientset
	podsFile      string
	namespacesFile string
}

// NewPodChecker creates a new PodChecker using the current kubeconfig
func NewPodChecker(podsFile, namespacesFile string) (*PodChecker, error) {
	pc := &PodChecker{
		podsFile:       podsFile,
		namespacesFile: namespacesFile,
	}

	// Only initialize Kubernetes client if we're fetching from API
	if podsFile == "" || namespacesFile == "" {
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
		pc.clientset = clientset
	}

	return pc, nil
}

// loadPodsFromFile loads a PodList from a YAML file
func loadPodsFromFile(filename string) (*corev1.PodList, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	// Handle both PodList and List types
	switch v := obj.(type) {
	case *corev1.PodList:
		return v, nil
	case *corev1.List:
		podList := &corev1.PodList{}
		for _, item := range v.Items {
			pod := &corev1.Pod{}
			if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), item.Raw, pod); err != nil {
				return nil, fmt.Errorf("failed to decode pod from List: %w", err)
			}
			podList.Items = append(podList.Items, *pod)
		}
		return podList, nil
	default:
		return nil, fmt.Errorf("expected PodList or List, got %T", obj)
	}
}

// loadNamespacesFromFile loads a NamespaceList from a YAML file
func loadNamespacesFromFile(filename string) (*corev1.NamespaceList, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	// Handle both NamespaceList and List types
	switch v := obj.(type) {
	case *corev1.NamespaceList:
		return v, nil
	case *corev1.List:
		nsList := &corev1.NamespaceList{}
		for _, item := range v.Items {
			ns := &corev1.Namespace{}
			if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), item.Raw, ns); err != nil {
				return nil, fmt.Errorf("failed to decode namespace from List: %w", err)
			}
			nsList.Items = append(nsList.Items, *ns)
		}
		return nsList, nil
	default:
		return nil, fmt.Errorf("expected NamespaceList or List, got %T", obj)
	}
}

// RunCheck fetches all pods and namespaces, then applies the check function
func (pc *PodChecker) RunCheck(ctx context.Context, checkFn CheckFunc) error {
	var namespaces *corev1.NamespaceList
	var pods *corev1.PodList
	var err error

	// Load or fetch namespaces
	if pc.namespacesFile != "" {
		namespaces, err = loadNamespacesFromFile(pc.namespacesFile)
		if err != nil {
			return err
		}
	} else {
		namespaces, err = pc.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}
	}

	// Load or fetch pods
	if pc.podsFile != "" {
		pods, err = loadPodsFromFile(pc.podsFile)
		if err != nil {
			return err
		}
	} else {
		pods, err = pc.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
	}

	// Create a map of namespace name to namespace object for quick lookup
	nsMap := make(map[string]*corev1.Namespace)
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		nsMap[ns.Name] = ns
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
