package config

import "github.com/caarlos0/env/v6"

type GitConfig struct {
	Disabled bool
	Order    int

	Uri            string           `env:"GIT_URI"`
	KnownHostsFile string           `json:"knownHostsFile" env:"GIT_KNOWN_HOSTS_FILE"`
	PrivateKey     string           `json:"privateKey" env:"GIT_PRIVATE_KEY"`
	Token          string           `json:"token" env:"GIT_TOKEN"`
	TokenFile      string           `json:"tokenFile" env:"GIT_TOKEN_FILE"`
	GitHubApp      *GitHubAppConfig `json:"githubApp"`

	Basedir                string `json:"basedir"`
	DisableBaseDirCleaning bool   `json:"disableBaseDirCleaning"`

	DisableLabels     bool   `json:"disableLabels"`
	DefaultBranchName string `json:"defaultBranchName"`

	CloneOnStart bool `json:"clone-on-start"`
	ForcePull    bool `json:"force-pull"`

	// ConnectTimeoutMillis bounds the TCP dial and TLS handshake to the remote (default 5000).
	ConnectTimeoutMillis int64 `json:"connectTimeout"`

	// StartupRetries is the number of retries after a failed startup clone (default 5, negative disables).
	StartupRetries int `json:"startupRetries"`
	// StartupRetryBackoffMillis is the initial delay between startup retries, doubling each attempt (default 1000).
	StartupRetryBackoffMillis int64 `json:"startupRetryBackoff"`

	ShowProgress bool `json:"show-progress"`

	TimeoutMillis     int64 `json:"timeout"`
	RefreshRateMillis int64 `json:"refreshRate"`
}

type GitHubAppConfig struct {
	ClientID       string `json:"clientId" env:"GIT_GITHUB_APP_CLIENT_ID"`
	InstallationID int64  `json:"installationId" env:"GIT_GITHUB_APP_INSTALLATION_ID"`
	PrivateKeyFile string `json:"privateKeyFile" env:"GIT_GITHUB_APP_PRIVATE_KEY_FILE"`
}

// ReadEnvironment overrides Git settings after loading application.yml.
func (g *GitConfig) ReadEnvironment() error {
	if g.Disabled {
		return nil
	}

	common := *g
	common.GitHubApp = nil
	if err := env.Parse(&common); err != nil {
		return err
	}
	common.GitHubApp = g.GitHubApp
	*g = common
	if g.PrivateKey != "" {
		return nil
	}

	hasAppConfig := g.GitHubApp != nil
	if !hasAppConfig {
		g.GitHubApp = &GitHubAppConfig{}
	}
	if err := env.Parse(g.GitHubApp); err != nil {
		return err
	}
	if !hasAppConfig && *g.GitHubApp == (GitHubAppConfig{}) {
		g.GitHubApp = nil
	}
	return nil
}
