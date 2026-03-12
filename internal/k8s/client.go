package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// ClientConfig holds the configuration for creating a Kubernetes client.
type ClientConfig struct {
	KubeconfigPath    string
	ContextOverride   string
	NamespaceOverride string
	AllNamespaces     bool
}

// Client wraps a Kubernetes clientset and cluster metadata.
type Client struct {
	clientset        kubernetes.Interface
	restConfig       *rest.Config
	kubeconfigPath   string
	clusterInfo      ClusterInfo
	metricsAvailable bool
	metricsClient    *MetricsClient
}

// NewClient creates a new Kubernetes client from the provided config.
func NewClient(cfg ClientConfig) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.KubeconfigPath != "" {
		loadingRules.ExplicitPath = cfg.KubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if cfg.ContextOverride != "" {
		overrides.CurrentContext = cfg.ContextOverride
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// No contexts at all — kubeconfig is missing or empty
	if len(rawConfig.Contexts) == 0 {
		return nil, fmt.Errorf(
			"no kubeconfig found\n\n" +
				"  k8sweep requires a valid kubeconfig to connect to a Kubernetes cluster.\n\n" +
				"  Try one of:\n" +
				"    • Set KUBECONFIG environment variable:  export KUBECONFIG=/path/to/kubeconfig\n" +
				"    • Use the --kubeconfig/-k flag:            k8sweep -k /path/to/kubeconfig\n" +
				"    • Place config at default path:         ~/.kube/config\n")
	}

	contextName := rawConfig.CurrentContext
	if cfg.ContextOverride != "" {
		contextName = cfg.ContextOverride
	}

	if contextName == "" {
		// Config exists but no current-context is set
		available := make([]string, 0, len(rawConfig.Contexts))
		for name := range rawConfig.Contexts {
			available = append(available, name)
		}
		sort.Strings(available)
		return nil, fmt.Errorf(
			"no current-context set in kubeconfig\n\n"+
				"  Available contexts: %s\n\n"+
				"  Try one of:\n"+
				"    • Set a context:  kubectl config use-context <context-name>\n"+
				"    • Use the flag:   k8sweep --context <context-name>\n",
			strings.Join(available, ", "))
	}

	ctxInfo, ok := rawConfig.Contexts[contextName]
	if !ok {
		available := make([]string, 0, len(rawConfig.Contexts))
		for name := range rawConfig.Contexts {
			available = append(available, name)
		}
		sort.Strings(available)
		return nil, fmt.Errorf(
			"context %q not found in kubeconfig\n\n"+
				"  Available contexts: %s\n\n"+
				"  Try: k8sweep --context <context-name>\n",
			contextName, strings.Join(available, ", "))
	}

	namespace := ctxInfo.Namespace
	if namespace == "" {
		namespace = "default"
	}
	if cfg.NamespaceOverride != "" {
		namespace = cfg.NamespaceOverride
	}
	if cfg.AllNamespaces {
		namespace = AllNamespaces
	}

	server := ""
	if cluster, ok := rawConfig.Clusters[ctxInfo.Cluster]; ok {
		server = cluster.Server
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config: %w", err)
	}

	// Increase QPS and burst to support parallel per-namespace fetches
	// in all-namespaces mode. Defaults (QPS=5, Burst=10) cause client-side
	// throttling when multiple goroutines make concurrent API calls.
	restConfig.QPS = 50
	restConfig.Burst = 100

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	c := &Client{
		clientset:      clientset,
		restConfig:     restConfig,
		kubeconfigPath: cfg.KubeconfigPath,
		clusterInfo: ClusterInfo{
			ContextName: contextName,
			Namespace:   namespace,
			Server:      server,
		},
	}

	return c, nil
}

// NewClientFromClientset creates a Client from an existing clientset (for testing).
func NewClientFromClientset(cs kubernetes.Interface, info ClusterInfo) *Client {
	return &Client{
		clientset:   cs,
		clusterInfo: info,
	}
}

// GetClusterInfo returns a copy of the cluster connection details.
func (c *Client) GetClusterInfo() ClusterInfo {
	return c.clusterInfo
}

// Clientset returns the underlying Kubernetes clientset.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// RESTConfig returns the underlying Kubernetes REST configuration.
func (c *Client) RESTConfig() *rest.Config {
	return c.restConfig
}

// KubeconfigPath returns the explicit kubeconfig path used to create the client.
// Empty means default client-go loading rules were used.
func (c *Client) KubeconfigPath() string {
	return c.kubeconfigPath
}

// MetricsAvailable returns true if the Metrics API is available in the cluster.
func (c *Client) MetricsAvailable() bool {
	return c.metricsAvailable
}

// EnableMetrics activates metrics support after an async probe confirms availability.
func (c *Client) EnableMetrics() error {
	mc, err := metricsclient.NewForConfig(c.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}
	c.metricsAvailable = true
	c.metricsClient = NewMetricsClient(mc)
	return nil
}

// GetMetricsClient returns the metrics client, or nil if metrics are unavailable.
func (c *Client) GetMetricsClient() *MetricsClient {
	return c.metricsClient
}

// ListNamespaces returns a sorted list of namespace names.
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	nsList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	names := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}
