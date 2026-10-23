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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/GlintPay/gccs/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type githubAppAuth struct {
	mu        sync.Mutex
	clientID  string
	key       *rsa.PrivateKey
	endpoint  string
	client    *http.Client
	token     string
	expiresAt time.Time
}

func newGitHubAppAuth(cfg config.GitConfig) (*githubAppAuth, error) {
	app := cfg.GitHubApp
	if cfg.Token != "" || cfg.TokenFile != "" || cfg.PrivateKey != "" {
		return nil, fmt.Errorf("git.githubApp cannot be combined with git.token, git.tokenFile or git.privateKey")
	}
	u, err := url.Parse(cfg.Uri)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil {
		return nil, fmt.Errorf("git.githubApp requires an HTTPS github.com git.uri without embedded credentials")
	}
	if strings.TrimSpace(app.ClientID) == "" || app.InstallationID <= 0 || app.PrivateKeyFile == "" {
		return nil, fmt.Errorf("git.githubApp requires clientId, a positive installationId and privateKeyFile")
	}
	data, err := os.ReadFile(app.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read git.githubApp.privateKeyFile: %w", err)
	}
	key, err := parseAppKey(data)
	if err != nil {
		return nil, err
	}
	return &githubAppAuth{
		clientID: app.ClientID, key: key,
		endpoint: fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", app.InstallationID),
		client:   &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

func parseAppKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("git.githubApp.privateKeyFile must contain a PEM RSA private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("git.githubApp.privateKeyFile must contain an unencrypted PKCS1 or PKCS8 RSA private key")
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("git.githubApp.privateKeyFile must contain an RSA private key")
		}
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("invalid git.githubApp RSA private key")
	}
	return key, nil
}

func (s *Backend) authentication(ctx context.Context) (transport.AuthMethod, error) {
	if s.githubApp == nil {
		return s.Auth, nil
	}
	return s.githubApp.authentication(ctx)
}

func (a *githubAppAuth) authentication(ctx context.Context) (transport.AuthMethod, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Until(a.expiresAt) > time.Minute {
		return &gitHTTP.BasicAuth{Username: "x-access-token", Password: a.token}, nil
	}
	jwt, err := a.jwt(time.Now())
	if err != nil {
		return nil, fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, strings.NewReader(`{"permissions":{"contents":"read"}}`))
	if err != nil {
		return nil, fmt.Errorf("create GitHub App token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GitHub App installation token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		// API response bodies may contain credentials; report only the status code.
		return nil, fmt.Errorf("GitHub App installation token request returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid GitHub App installation token response")
	}
	if strings.TrimSpace(result.Token) == "" || time.Until(result.ExpiresAt) <= time.Minute {
		return nil, fmt.Errorf("GitHub App installation token response has an empty token or invalid expiry")
	}
	a.token, a.expiresAt = result.Token, result.ExpiresAt
	return &gitHTTP.BasicAuth{Username: "x-access-token", Password: a.token}, nil
}

func (a *githubAppAuth) jwt(now time.Time) (string, error) {
	claims, err := json.Marshal(struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}{a.clientID, now.Add(-time.Minute).Unix(), now.Add(9 * time.Minute).Unix()})
	if err != nil {
		return "", err
	}
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, a.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + encode(signature), nil
}
