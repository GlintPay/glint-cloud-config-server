package config

type K8sConfig struct {
	Enabled          bool   // Must be explicitly enabled to use K8s resolution
	Kubeconfig       string // Path to kubeconfig file (empty = in-cluster auth)
	DefaultNamespace string // Default namespace when not specified in placeholder
	CacheTTLSeconds  int    // Secret/ConfigMap cache TTL (0 = no caching)
}
