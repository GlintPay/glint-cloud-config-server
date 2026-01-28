package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GlintPay/gccs/config"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset *kubernetes.Clientset
	config    config.K8sConfig
	cache     *resourceCache
}

type resourceCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

func NewClient(cfg config.K8sConfig) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		// Out-of-cluster: use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
		log.Info().Str("kubeconfig", cfg.Kubeconfig).Msg("Using kubeconfig for K8s authentication")
	} else {
		// In-cluster: use service account
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		log.Info().Msg("Using in-cluster K8s authentication")
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	client := &Client{
		clientset: clientset,
		config:    cfg,
	}

	if cfg.CacheTTLSeconds > 0 {
		client.cache = &resourceCache{
			entries: make(map[string]cacheEntry),
			ttl:     time.Duration(cfg.CacheTTLSeconds) * time.Second,
		}
		log.Info().Int("ttl_seconds", cfg.CacheTTLSeconds).Msg("K8s resource caching enabled")
	}

	return client, nil
}

func (c *Client) GetSecretValue(ctx context.Context, namespace, name, key string) (string, bool, error) {
	cacheKey := fmt.Sprintf("secret:%s/%s/%s", namespace, name, key)

	if c.cache != nil {
		if val, ok := c.cache.get(cacheKey); ok {
			return val, true, nil
		}
	}

	log.Debug().Msgf("Fetching K8s secret [%s/%s] with key [%s]...", namespace, name, key)
	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	value, ok := c.getSecretDataValue(secret, key)
	if !ok {
		return "", false, nil
	}

	if c.cache != nil {
		c.cache.set(cacheKey, value)
	}

	return value, true, nil
}

func (c *Client) getSecretDataValue(secret *corev1.Secret, key string) (string, bool) {
	// Try Data first (base64 decoded by client-go)
	if data, ok := secret.Data[key]; ok {
		return string(data), true
	}
	// Fall back to StringData
	if data, ok := secret.StringData[key]; ok {
		return data, true
	}
	return "", false
}

func (c *Client) GetConfigMapValue(ctx context.Context, namespace, name, key string) (string, bool, error) {
	cacheKey := fmt.Sprintf("configmap:%s/%s/%s", namespace, name, key)

	if c.cache != nil {
		if val, ok := c.cache.get(cacheKey); ok {
			return val, true, nil
		}
	}

	log.Debug().Msgf("Fetching K8s configmap [%s/%s] with key [%s]...", namespace, name, key)
	configMap, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to get configmap %s/%s: %w", namespace, name, err)
	}

	value, ok := configMap.Data[key]
	if !ok {
		// Try BinaryData as string
		if binData, ok := configMap.BinaryData[key]; ok {
			value = string(binData)
		} else {
			return "", false, nil
		}
	}

	if c.cache != nil {
		c.cache.set(cacheKey, value)
	}

	return value, true, nil
}

func (rc *resourceCache) get(key string) (string, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, ok := rc.entries[key]
	if !ok {
		return "", false
	}

	if time.Now().After(entry.expiresAt) {
		return "", false
	}

	return entry.value, true
}

func (rc *resourceCache) set(key, value string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(rc.ttl),
	}
}
