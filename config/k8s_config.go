package config

type K8sConfig struct {
	Disabled         bool
	Kubeconfig       string // Path to kubeconfig file (empty = in-cluster auth)
	DefaultNamespace string // Default namespace when not specified in placeholder
	CacheTTLSeconds  int    // Secret/ConfigMap cache TTL (0 = no caching)
}
