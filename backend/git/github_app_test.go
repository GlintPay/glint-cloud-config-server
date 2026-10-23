package git

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GlintPay/gccs/config"
	goGit "github.com/go-git/go-git/v5"
	goGitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func appConfig(t *testing.T) (config.GitConfig, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "app.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	var cfg config.ApplicationConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf("git:\n  uri: https://github.com/Org/repo.git\n  githubApp:\n    clientId: Iv1.test\n    installationId: 123\n    privateKeyFile: %s\n", path)), &cfg))
	return cfg.Git, key
}

func TestGitHubAppTokenLifecycle(t *testing.T) {
	cfg, key := appConfig(t)
	var b Backend
	require.NoError(t, b.Init(context.Background(), config.ApplicationConfiguration{Git: cfg}))
	require.Equal(t, "https://api.github.com/app/installations/123/access_tokens", b.githubApp.endpoint)
	require.Equal(t, 15*time.Second, b.githubApp.client.Timeout)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "2026-03-10", r.Header.Get("X-GitHub-Api-Version"))
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if !assert.Len(t, parts, 3) {
			http.Error(w, "invalid JWT", http.StatusBadRequest)
			return
		}
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		assert.NoError(t, err)
		hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		assert.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], signature))
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		assert.NoError(t, err)
		var claims struct {
			Iss      string
			Iat, Exp int64
		}
		assert.NoError(t, json.Unmarshal(payload, &claims))
		assert.Equal(t, "Iv1.test", claims.Iss)
		assert.InDelta(t, time.Now().Add(-time.Minute).Unix(), claims.Iat, 5)
		assert.InDelta(t, time.Now().Add(9*time.Minute).Unix(), claims.Exp, 5)
		var body struct {
			Repositories []string          `json:"repositories"`
			Permissions  map[string]string `json:"permissions"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"repo"}, body.Repositories)
		assert.Equal(t, map[string]string{"contents": "read"}, body.Permissions)
		w.WriteHeader(http.StatusCreated)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("token-%d", calls.Load()), "expires_at": time.Now().Add(time.Hour)}))
	}))
	defer server.Close()
	b.githubApp.endpoint = server.URL
	auth, err := b.authentication(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-1", auth.(*gitHTTP.BasicAuth).Password)
	auth, err = b.authentication(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-1", auth.(*gitHTTP.BasicAuth).Password)
	require.EqualValues(t, 1, calls.Load())
	b.githubApp.expiresAt = time.Now().Add(30 * time.Second)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			auth, err := b.authentication(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			if auth.(*gitHTTP.BasicAuth).Password != "token-2" {
				t.Error("stale credentials")
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 2, calls.Load())
}

func TestGitHubAppValidation(t *testing.T) {
	cfg, key := appConfig(t)
	for _, tt := range []struct {
		name   string
		change func(*config.GitConfig)
	}{
		{"token", func(c *config.GitConfig) { c.Token = "secret" }},
		{"token file", func(c *config.GitConfig) { c.TokenFile = "token" }},
		{"HTTP", func(c *config.GitConfig) { c.Uri = "http://github.com/Org/repo.git" }},
		{"other host", func(c *config.GitConfig) { c.Uri = "https://example.com/Org/repo.git" }},
		{"embedded credentials", func(c *config.GitConfig) { c.Uri = "https://user:secret@github.com/Org/repo.git" }},
		{"missing client", func(c *config.GitConfig) { c.GitHubApp.ClientID = "" }},
		{"missing installation", func(c *config.GitConfig) { c.GitHubApp.InstallationID = 0 }},
		{"missing key", func(c *config.GitConfig) { c.GitHubApp.PrivateKeyFile = "" }},
		{"unreadable key", func(c *config.GitConfig) { c.GitHubApp.PrivateKeyFile += ".missing" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			copyCfg := cfg
			app := *cfg.GitHubApp
			copyCfg.GitHubApp = &app
			tt.change(&copyCfg)
			var b Backend
			err := b.Init(context.Background(), config.ApplicationConfiguration{Git: copyCfg})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	parsed, err := parseAppKey(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	require.NoError(t, err)
	require.Equal(t, key.N, parsed.N)
	for _, data := range [][]byte{nil, []byte("secret"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("secret")})} {
		_, err := parseAppKey(data)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
	}
}

func TestGitHubAppTokenFailures(t *testing.T) {
	cfg, _ := appConfig(t)
	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden", 403, `{"message":"secret"}`},
		{"malformed", 201, `secret`},
		{"empty token", 201, `{"expires_at":"2099-01-01T00:00:00Z"}`},
		{"missing expiry", 201, `{"token":"secret"}`},
		{"expired", 201, `{"token":"secret","expires_at":"2000-01-01T00:00:00Z"}`},
		{"redirect", 302, `secret`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, err := newGitHubAppAuth(cfg)
			require.NoError(t, err)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			a.endpoint = server.URL
			a.token = "expired-secret"
			a.expiresAt = time.Now().Add(-time.Hour)
			_, err = a.authentication(context.Background())
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
	a, err := newGitHubAppAuth(cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = a.authentication(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestGitHubAppGitOperations(t *testing.T) {
	cfg, _ := appConfig(t)
	var tokenRequests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenRequests.Add(1)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("installation-%d", tokenRequests.Load()), "expires_at": time.Now().Add(time.Hour)})
	}))
	defer api.Close()
	var password string
	gitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, token, _ := r.BasicAuth()
		assert.Equal(t, "x-access-token", username)
		password = token
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer gitServer.Close()
	for _, operation := range []string{"clone", "pull", "fetch"} {
		t.Run(operation, func(t *testing.T) {
			var b Backend
			require.NoError(t, b.Init(context.Background(), config.ApplicationConfiguration{Git: cfg}))
			b.githubApp.endpoint = api.URL
			b.Config.Uri = gitServer.URL
			b.Config.Basedir = filepath.Join(t.TempDir(), "repo")
			b.Config.DefaultBranchName = "main"
			if operation == "clone" {
				require.Error(t, b.connect(context.Background(), "", false))
			} else {
				repo, err := goGit.PlainInitWithOptions(b.Config.Basedir, &goGit.PlainInitOptions{InitOptions: goGit.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")}})
				require.NoError(t, err)
				// A stale origin must not receive installation credentials.
				_, err = repo.CreateRemote(&goGitConfig.RemoteConfig{Name: "origin", URLs: []string{"https://stale-origin.invalid/repo.git"}})
				require.NoError(t, err)
				w, err := repo.Worktree()
				require.NoError(t, err)
				_, err = w.Commit("initial", &goGit.CommitOptions{AllowEmptyCommits: true, Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}})
				require.NoError(t, err)
				if operation == "pull" {
					require.Error(t, b.connect(context.Background(), "", false))
				} else {
					require.Error(t, b.fetchOrigin(context.Background(), repo, ""))
				}
			}
			require.Equal(t, fmt.Sprintf("installation-%d", tokenRequests.Load()), password)
		})
	}
	require.EqualValues(t, 3, tokenRequests.Load())
}

func TestGitHubAppRenewalWaitCancellation(t *testing.T) {
	cfg, _ := appConfig(t)
	a, err := newGitHubAppAuth(cfg)
	require.NoError(t, err)
	started := make(chan struct{})
	unblock := make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		select {
		case <-unblock:
		case <-r.Context().Done():
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "renewed-token", "expires_at": time.Now().Add(time.Hour)})
	}))
	defer server.Close()
	defer release()
	a.endpoint = server.URL
	first := make(chan error, 1)
	go func() {
		_, err := a.authentication(context.Background())
		first <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first renewal did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan struct{})
	second := make(chan error, 1)
	go func() {
		close(waiting)
		_, err := a.authentication(ctx)
		second <- err
	}()
	<-waiting
	cancel()
	select {
	case err := <-second:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("canceled caller remained blocked by another caller's renewal")
	}
	select {
	case err := <-first:
		t.Fatalf("first renewal finished before being released: %v", err)
	default:
	}
	require.EqualValues(t, 1, calls.Load())
	release()
	require.NoError(t, <-first)
	auth, err := a.authentication(context.Background())
	require.NoError(t, err)
	require.Equal(t, "renewed-token", auth.(*gitHTTP.BasicAuth).Password)
	require.EqualValues(t, 1, calls.Load())
}

func TestGitHubRepository(t *testing.T) {
	for _, uri := range []string{
		"https://github.com/Org/repo.git",
		"https://github.com:443/Org/repo.git",
		"https://github.com/Org/repo",
		"https://github.com/Org/repo.git/",
	} {
		t.Run(uri, func(t *testing.T) {
			repository, err := githubRepository(uri)
			require.NoError(t, err)
			require.Equal(t, "repo", repository)
		})
	}
	for _, uri := range []string{
		"https://github.com:8443/Org/repo.git",
		"https://github.com/Org",
		"https://github.com//repo.git",
		"https://github.com/Org/.git",
		"https://github.com/Org/repo.git/extra",
		"https://github.com/Org/repo.git?query=secret",
		"https://github.com/Org/repo.git#fragment",
	} {
		t.Run(uri, func(t *testing.T) {
			_, err := githubRepository(uri)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}
