package k8s

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	clientset   kubernetes.Interface
	clusterInfo ClusterInfo
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

	contextName := rawConfig.CurrentContext
	if cfg.ContextOverride != "" {
		contextName = cfg.ContextOverride
	}

	ctxInfo, ok := rawConfig.Contexts[contextName]
	if !ok {
		return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
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

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset: clientset,
		clusterInfo: ClusterInfo{
			ContextName: contextName,
			Namespace:   namespace,
			Server:      server,
		},
	}, nil
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
