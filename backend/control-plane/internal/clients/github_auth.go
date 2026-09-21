package clients

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
	"github.com/jahla2/Xentra/backend/control-plane/internal/observability"
)

type GitHubStoredTokenProvider interface {
	GetGitHubToken(context.Context, string) (string, error)
}

type GitHubAuthProvider struct {
	storedCredentials GitHubStoredTokenProvider
	appID             string
	privateKey        *rsa.PrivateKey
	baseURL           string
	http              *http.Client
	mu                sync.Mutex
	cache             map[int64]cachedInstallationToken
}

type cachedInstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

func NewGitHubAuthProvider(stored GitHubStoredTokenProvider, appID, privateKeyPEM, baseURL string) (*GitHubAuthProvider, error) {
	provider := &GitHubAuthProvider{
		storedCredentials: stored,
		appID:             strings.TrimSpace(appID),
		baseURL:           strings.TrimRight(baseURL, "/"),
		http:              &http.Client{Timeout: 10 * time.Second, Transport: observability.NewTracingTransport(nil)},
		cache:             map[int64]cachedInstallationToken{},
	}
	if provider.baseURL == "" {
		provider.baseURL = "https://api.github.com"
	}
	if provider.appID == "" && strings.TrimSpace(privateKeyPEM) == "" {
		return provider, nil
	}
	if provider.appID == "" || strings.TrimSpace(privateKeyPEM) == "" {
		return nil, errors.New("GitHub App ID and private key must be configured together")
	}
	key, err := parseGitHubAppPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	provider.privateKey = key
	return provider, nil
}

func (p *GitHubAuthProvider) AppConfigured() bool {
	return p != nil && p.appID != "" && p.privateKey != nil
}

func (p *GitHubAuthProvider) Token(ctx context.Context, integration domain.RepositoryIntegration) (string, error) {
	if integration.AuthMode == "github_app" {
		if integration.InstallationID <= 0 {
			return "", errors.New("GitHub App integration has no installation ID")
		}
		return p.installationToken(ctx, integration.InstallationID)
	}
	if integration.CredentialID == "" || p.storedCredentials == nil {
		return "", errors.New("GitHub token credential is unavailable")
	}
	return p.storedCredentials.GetGitHubToken(ctx, integration.CredentialID)
}

func (p *GitHubAuthProvider) ResolveInstallation(ctx context.Context, owner, repoName string) (int64, error) {
	if !p.AppConfigured() {
		return 0, errors.New("GitHub App is not configured on the Xentra control plane")
	}
	jwt, err := p.appJWT(time.Now().UTC())
	if err != nil {
		return 0, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/installation", p.baseURL, url.PathEscape(owner), url.PathEscape(repoName))
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := p.githubJSON(ctx, http.MethodGet, endpoint, jwt, nil, &payload); err != nil {
		return 0, err
	}
	if payload.ID <= 0 {
		return 0, errors.New("GitHub App installation was not found for repository")
	}
	return payload.ID, nil
}

func (p *GitHubAuthProvider) installationToken(ctx context.Context, installationID int64) (string, error) {
	p.mu.Lock()
	if cached, ok := p.cache[installationID]; ok && time.Until(cached.ExpiresAt) > 5*time.Minute {
		p.mu.Unlock()
		return cached.Token, nil
	}
	p.mu.Unlock()

	jwt, err := p.appJWT(time.Now().UTC())
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("%s/app/installations/%d/access_tokens", p.baseURL, installationID)
	var payload struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := p.githubJSON(ctx, http.MethodPost, endpoint, jwt, map[string]any{}, &payload); err != nil {
		return "", err
	}
	if payload.Token == "" {
		return "", errors.New("GitHub App token response was empty")
	}
	p.mu.Lock()
	p.cache[installationID] = cachedInstallationToken{Token: payload.Token, ExpiresAt: payload.ExpiresAt}
	p.mu.Unlock()
	return payload.Token, nil
}

func (p *GitHubAuthProvider) appJWT(now time.Time) (string, error) {
	if !p.AppConfigured() {
		return "", errors.New("GitHub App is not configured")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": p.appID,
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claims)
	unsigned := header + "." + payload
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (p *GitHubAuthProvider) githubJSON(ctx context.Context, method, endpoint, bearer string, body any, target any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("github app status %d", res.StatusCode)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func parseGitHubAppPrivateKey(value string) (*rsa.PrivateKey, error) {
	value = strings.ReplaceAll(value, "\\n", "\n")
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("GitHub App private key is not valid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub App private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("GitHub App private key is not RSA")
	}
	return key, nil
}

