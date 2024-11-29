package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ConfigMapRef struct {
	namespace string
	name      string
}

// Result structure to hold scanning results for each namespace
type NamespaceScanResult struct {
	namespace      string
	usedConfigMaps map[ConfigMapRef]bool
	err            error
}

func scanNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string, resultChan chan<- NamespaceScanResult, wg *sync.WaitGroup) {
	defer wg.Done()

	result := NamespaceScanResult{
		namespace:      namespace,
		usedConfigMaps: make(map[ConfigMapRef]bool),
	}

	// Helper function to handle errors
	handleError := func(err error) {
		if err != nil {
			fmt.Printf("Warning: error scanning resources in namespace %s: %v\n", namespace, err)
		}
	}

	// Use a WaitGroup for parallel resource scanning within namespace
	var resourceWg sync.WaitGroup
	var resourceMutex sync.Mutex

	// Scan Pods
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, pod := range pods.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(pod.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Scan Deployments
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, deployment := range deployments.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(deployment.Spec.Template.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Scan StatefulSets
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, sts := range statefulsets.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(sts.Spec.Template.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Scan DaemonSets
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		daemonsets, err := clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, ds := range daemonsets.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(ds.Spec.Template.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Scan Jobs
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, job := range jobs.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(job.Spec.Template.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Scan CronJobs
	resourceWg.Add(1)
	go func() {
		defer resourceWg.Done()
		cronjobs, err := clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		handleError(err)
		if err == nil {
			for _, cronjob := range cronjobs.Items {
				resourceMutex.Lock()
				findConfigMapsInPodSpec(cronjob.Spec.JobTemplate.Spec.Template.Spec, namespace, result.usedConfigMaps)
				resourceMutex.Unlock()
			}
		}
	}()

	// Wait for all resource scans to complete
	resourceWg.Wait()
	resultChan <- result
}

func main() {
	// Number of concurrent workers for scanning namespaces
	workers := 5

	// Add flags
	deleteUnused := flag.Bool("delete", false, "Delete unused ConfigMaps")
	namespace := flag.String("namespace", "", "Namespace to scan for ConfigMaps")
	flag.Parse()

	// Load the kubeconfig using the default loading rules
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get the current context and configuration
	config, err := kubeConfig.ClientConfig()
	config.QPS = 100   // Incrase from default 5
	config.Burst = 100 // Incrase from default 10
	if err != nil {
		fmt.Println(os.Stderr, "Error getting Kubernetes config: %v\n", err)
		os.Exit(1)
	}

	// Get current context name for display
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting raw config: %v\n", err)
		os.Exit(1)
	}
	currentContext := rawConfig.CurrentContext

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using context: %s\n", currentContext)
	if *namespace != "" {
		fmt.Printf("Scanning namespace: %s\n", *namespace)
	} else {
		fmt.Println("Scanning all accessible namespaces")
	}

	ctx := context.Background()

	// Get namespaces to scan
	var namespacesToScan []string
	if *namespace != "" {
		// Verify the namespace exists
		_, err := clientset.CoreV1().Namespaces().Get(ctx, *namespace, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: namespace %s does not exist\n", *namespace)
			os.Exit(1)
		}
		namespacesToScan = []string{*namespace}
	} else {
		namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing namespaces: %v\n", err)
			os.Exit(1)
		}
		for _, ns := range namespaces.Items {
			namespacesToScan = append(namespacesToScan, ns.Name)
		}
	}

	// Create channel for results and WaitGroup for goroutines
	resultChan := make(chan NamespaceScanResult, len(namespacesToScan))
	var wg sync.WaitGroup

	// Process namespaces with worker pool
	semaphore := make(chan struct{}, workers)
	for _, ns := range namespacesToScan {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore
		go func(namespace string) {
			defer func() { <-semaphore }() // Release semaphore
			scanNamespace(ctx, clientset, namespace, resultChan, &wg)
		}(ns)
	}

	// Start a goroutine to close result channel when all work is done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	usedConfigMaps := make(map[ConfigMapRef]bool)
	for result := range resultChan {
		for cm := range result.usedConfigMaps {
			usedConfigMaps[cm] = true
		}
	}

	// Get all ConfigMaps
	allConfigMaps := make(map[ConfigMapRef]bool)
	for _, ns := range namespacesToScan {
		if configMaps, err := clientset.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{}); err == nil {
			for _, cm := range configMaps.Items {
				allConfigMaps[ConfigMapRef{namespace: ns, name: cm.Name}] = true
			}
		}
	}

	// Print results
	fmt.Printf("\nConfigMaps currently in use:\n")
	fmt.Println("================================")
	printSortedConfigMaps(usedConfigMaps)

	fmt.Printf("\nUnused ConfigMaps:\n")
	fmt.Println("=================")
	unusedConfigMaps := make(map[ConfigMapRef]bool)
	for cm := range allConfigMaps {
		if !usedConfigMaps[cm] {
			unusedConfigMaps[cm] = true
		}
	}
	printSortedConfigMaps(unusedConfigMaps)

	// Handle deletion if requested
	if *deleteUnused && len(unusedConfigMaps) > 0 {
		fmt.Printf("\nWARNING: You are about to delete %d unused ConfigMaps.\n", len(unusedConfigMaps))
		fmt.Printf("This action cannot be undone. Are you sure? (yes/no): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "yes" {
			fmt.Println("\nDeleting unused ConfigMaps...")
			deleteUnusedConfigMaps(ctx, clientset, unusedConfigMaps)
		} else {
			fmt.Println("Deletion cancelled")
		}
	}
}

func findConfigMapsInPodSpec(spec corev1.PodSpec, namespace string, usedConfigMaps map[ConfigMapRef]bool) {
	// Check volumes
	for _, volume := range spec.Volumes {
		if volume.ConfigMap != nil {
			usedConfigMaps[ConfigMapRef{namespace: namespace, name: volume.ConfigMap.Name}] = true
		}
	}

	// Check containers and init containers
	containers := append(spec.Containers, spec.InitContainers...)
	for _, container := range containers {
		// Check envFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				usedConfigMaps[ConfigMapRef{namespace: namespace, name: envFrom.ConfigMapRef.Name}] = true
			}
		}

		// Check env
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				usedConfigMaps[ConfigMapRef{namespace: namespace, name: env.ValueFrom.ConfigMapKeyRef.Name}] = true
			}
		}
	}
}

func printSortedConfigMaps(configMaps map[ConfigMapRef]bool) {
	var refs []ConfigMapRef
	for ref := range configMaps {
		refs = append(refs, ref)
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].namespace != refs[j].namespace {
			return refs[i].namespace < refs[j].namespace
		}
		return refs[i].name < refs[j].name
	})

	for _, ref := range refs {
		fmt.Printf("Namespace: %s, Configmap: %s\n", ref.namespace, ref.name)
	}
}

func deleteUnusedConfigMaps(ctx context.Context, clientset *kubernetes.Clientset, unusedConfigMaps map[ConfigMapRef]bool) {
	var failed []ConfigMapRef

	for cm := range unusedConfigMaps {
		err := clientset.CoreV1().ConfigMaps(cm.namespace).Delete(ctx, cm.name, metav1.DeleteOptions{})
		if err != nil {
			failed = append(failed, cm)
			fmt.Printf("Failed to delete ConfigMap %s in namespace %s: %v\n", cm.name, cm.namespace, err)
		} else {
			fmt.Printf("Deleted ConfigMap %s in namespace %s\n", cm.name, cm.namespace)
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nFailed to delete %d ConfigMaps:\n", len(failed))
		for _, cm := range failed {
			fmt.Printf("- %s/%s\n", cm.namespace, cm.name)
		}
	} else {
		fmt.Printf("\nSuccessfully delete all %d unused ConfigMaps\n", len(unusedConfigMaps))
	}
}
