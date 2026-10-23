package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitEnvironment(t *testing.T) {
	for _, name := range []string{"GIT_URI", "GIT_KNOWN_HOSTS_FILE", "GIT_PRIVATE_KEY", "GIT_TOKEN", "GIT_TOKEN_FILE", "GIT_GITHUB_APP_CLIENT_ID", "GIT_GITHUB_APP_INSTALLATION_ID", "GIT_GITHUB_APP_PRIVATE_KEY_FILE"} {
		t.Setenv(name, "")
	}
	cfg := GitConfig{Uri: "https://github.com/Org/repo.git", Basedir: "/tmp/config"}
	require.NoError(t, cfg.ReadEnvironment())
	require.Nil(t, cfg.GitHubApp)
	require.Equal(t, "https://github.com/Org/repo.git", cfg.Uri)
	require.Equal(t, "/tmp/config", cfg.Basedir)

	t.Setenv("GIT_GITHUB_APP_CLIENT_ID", "Iv1.test")
	t.Setenv("GIT_GITHUB_APP_INSTALLATION_ID", "123")
	t.Setenv("GIT_GITHUB_APP_PRIVATE_KEY_FILE", "/secrets/app.pem")
	require.NoError(t, cfg.ReadEnvironment())
	require.Equal(t, &GitHubAppConfig{ClientID: "Iv1.test", InstallationID: 123, PrivateKeyFile: "/secrets/app.pem"}, cfg.GitHubApp)

	t.Setenv("GIT_URI", "git@github.com:Org/repo.git")
	t.Setenv("GIT_KNOWN_HOSTS_FILE", "/ssh/known_hosts")
	t.Setenv("GIT_PRIVATE_KEY", "runtime-ssh-key")
	t.Setenv("GIT_TOKEN", "runtime-token")
	t.Setenv("GIT_TOKEN_FILE", "/secrets/token")
	require.NoError(t, cfg.ReadEnvironment())
	require.Equal(t, "git@github.com:Org/repo.git", cfg.Uri)
	require.Equal(t, "/ssh/known_hosts", cfg.KnownHostsFile)
	require.Equal(t, "runtime-ssh-key", cfg.PrivateKey)
	require.Equal(t, "runtime-token", cfg.Token)
	require.Equal(t, "/secrets/token", cfg.TokenFile)
	require.Equal(t, "/tmp/config", cfg.Basedir)

	t.Setenv("GIT_GITHUB_APP_INSTALLATION_ID", "invalid")
	require.NoError(t, cfg.ReadEnvironment())
	t.Setenv("GIT_PRIVATE_KEY", "")
	cfg.PrivateKey = ""
	require.Error(t, cfg.ReadEnvironment())
}

func TestGitEnvironmentIgnoresMalformedAppWithSSH(t *testing.T) {
	for _, source := range []string{"yaml", "environment"} {
		t.Run(source, func(t *testing.T) {
			t.Setenv("GIT_PRIVATE_KEY", "")
			t.Setenv("GIT_GITHUB_APP_INSTALLATION_ID", "invalid")
			cfg := GitConfig{GitHubApp: &GitHubAppConfig{ClientID: "from-yaml", InstallationID: 123}}
			if source == "yaml" {
				cfg.PrivateKey = "yaml-ssh-key"
			} else {
				t.Setenv("GIT_PRIVATE_KEY", "runtime-ssh-key")
			}
			require.NoError(t, cfg.ReadEnvironment())
			require.NotEmpty(t, cfg.PrivateKey)
			require.Equal(t, int64(123), cfg.GitHubApp.InstallationID)
		})
	}
}

func TestGitEnvironmentPreservesYAMLAppSettings(t *testing.T) {
	for _, name := range []string{"GIT_GITHUB_APP_CLIENT_ID", "GIT_GITHUB_APP_INSTALLATION_ID", "GIT_GITHUB_APP_PRIVATE_KEY_FILE"} {
		t.Setenv(name, "")
	}
	emptyApp := GitConfig{GitHubApp: &GitHubAppConfig{}}
	require.NoError(t, emptyApp.ReadEnvironment())
	require.NotNil(t, emptyApp.GitHubApp)
	cfg := GitConfig{GitHubApp: &GitHubAppConfig{ClientID: "from-yaml", InstallationID: 123, PrivateKeyFile: "/yaml/key.pem"}}
	t.Setenv("GIT_GITHUB_APP_PRIVATE_KEY_FILE", "/runtime/key.pem")
	require.NoError(t, cfg.ReadEnvironment())
	require.Equal(t, &GitHubAppConfig{ClientID: "from-yaml", InstallationID: 123, PrivateKeyFile: "/runtime/key.pem"}, cfg.GitHubApp)
}
