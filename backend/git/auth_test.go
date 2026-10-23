package git

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/GlintPay/gccs/config"
	goGit "github.com/go-git/go-git/v5"
	goGitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/yaml"
)

func TestTokenAuth(t *testing.T) {
	var cfg config.ApplicationConfiguration
	require.NoError(t, yaml.Unmarshal([]byte("git:\n  uri: https://github.com/Org/repo.git\n  token: test-token\n"), &cfg))
	var b Backend
	require.NoError(t, b.Init(context.Background(), cfg))
	auth, ok := b.Auth.(*gitHTTP.BasicAuth)
	require.True(t, ok)
	require.Equal(t, "x-access-token", auth.Username)
	require.Equal(t, "test-token", auth.Password)
	ref := plumbing.NewBranchReferenceName("main")
	require.Same(t, auth, b.getCloneOptions(ref, 0).Auth)
	require.Same(t, auth, b.getPullOptions(ref).Auth)
	require.Equal(t, cfg.Git.Uri, b.getCloneOptions(ref, 0).URL)

	// A branch fetch must send the same credentials as clone and pull.
	var username, password string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ = r.BasicAuth()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	repo, err := goGit.PlainInit(t.TempDir(), false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&goGitConfig.RemoteConfig{Name: "origin", URLs: []string{server.URL}})
	require.NoError(t, err)
	require.Error(t, b.fetchOrigin(context.Background(), repo, ""))
	require.Equal(t, "x-access-token", username)
	require.Equal(t, "test-token", password)
}

func TestTokenFileAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("test-token\n"), 0600))
	var cfg config.ApplicationConfiguration
	require.NoError(t, yaml.Unmarshal([]byte("git:\n  uri: https://github.com/Org/repo.git\n  tokenFile: "+path+"\n"), &cfg))
	var b Backend
	require.NoError(t, b.Init(context.Background(), cfg))
	auth, ok := b.Auth.(*gitHTTP.BasicAuth)
	require.True(t, ok)
	require.Equal(t, "test-token", auth.Password)
	require.Equal(t, "x-access-token", auth.Username)
	ref := plumbing.NewBranchReferenceName("main")
	require.Same(t, auth, b.getCloneOptions(ref, 0).Auth)
	require.Same(t, auth, b.getPullOptions(ref).Auth)

	for _, tt := range []struct {
		name string
		cfg  config.GitConfig
	}{
		{name: "missing", cfg: config.GitConfig{TokenFile: path + ".missing"}},
		{name: "directory", cfg: config.GitConfig{TokenFile: t.TempDir()}},
		{name: "conflicting token", cfg: config.GitConfig{TokenFile: path, Token: "test-token"}},
		{name: "conflicting key", cfg: config.GitConfig{TokenFile: path, PrivateKey: "private-key"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Uri = cfg.Git.Uri
			err := b.Init(context.Background(), config.ApplicationConfiguration{Git: tt.cfg})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "test-token")
		})
	}
	for _, content := range []string{"", " \n\t"} {
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		require.ErrorContains(t, b.Init(context.Background(), cfg), "non-empty token")
	}
}

func TestTokenValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		uri  string
		key  string
	}{
		{name: "SSH", uri: "git@github.com:Org/repo.git"},
		{name: "HTTP", uri: "http://github.com/Org/repo.git"},
		{name: "local", uri: "/tmp/repo"},
		{name: "missing URI"},
		{name: "missing host", uri: "https:///repo"},
		{name: "embedded credentials", uri: "https://user:secret@github.com/Org/repo.git"},
		{name: "conflicting key", uri: "https://github.com/Org/repo.git", key: "private-key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var b Backend
			err := b.Init(context.Background(), config.ApplicationConfiguration{Git: config.GitConfig{
				Uri: tt.uri, Token: "test-token", PrivateKey: tt.key,
			}})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "test-token")
			require.NotContains(t, err.Error(), "secret")
			err = b.Init(context.Background(), config.ApplicationConfiguration{Git: config.GitConfig{
				Uri: tt.uri, TokenFile: "token", PrivateKey: tt.key,
			}})
			require.Error(t, err)
		})
	}
}

func TestSSHAndUnauthenticatedAuth(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(key, "test")
	require.NoError(t, err)
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(knownHosts, nil, 0600))
	var b Backend
	require.NoError(t, b.Init(context.Background(), config.ApplicationConfiguration{Git: config.GitConfig{
		Uri: "git@github.com:Org/repo.git", PrivateKey: string(pem.EncodeToMemory(block)), KnownHostsFile: knownHosts,
	}}))
	require.IsType(t, &gitSSH.PublicKeys{}, b.Auth)
	ref := plumbing.NewBranchReferenceName("main")
	require.Same(t, b.Auth, b.getCloneOptions(ref, 0).Auth)
	require.Same(t, b.Auth, b.getPullOptions(ref).Auth)

	require.NoError(t, b.Init(context.Background(), config.ApplicationConfiguration{}))
	require.Nil(t, b.Auth)
	require.Nil(t, b.getCloneOptions(ref, 0).Auth)
	require.Nil(t, b.getPullOptions(ref).Auth)
}
