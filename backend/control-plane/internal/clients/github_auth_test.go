package clients

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeStoredGitHubTokenProvider struct{ token string }

func (f fakeStoredGitHubTokenProvider) GetGitHubToken(context.Context, string) (string, error) {
	return f.token, nil
}

func testGitHubAppPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

func TestGitHubAuthProviderResolvesInstallationAndCachesInstallationToken(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer authorization for %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/repos/acme/api/installation":
			fmt.Fprint(w, `{"id":42}`)
		case "/app/installations/42/access_tokens":
			tokenCalls.Add(1)
			fmt.Fprintf(w, `{"token":"installation-token","expires_at":"%s"}`, time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := NewGitHubAuthProvider(
		fakeStoredGitHubTokenProvider{token: "fallback-token"},
		"12345",
		testGitHubAppPrivateKey(t),
		server.URL,
	)
	if err != nil {
		t.Fatal(err)
	}
	installationID, err := provider.ResolveInstallation(context.Background(), "acme", "api")
	if err != nil {
		t.Fatal(err)
	}
	if installationID != 42 {
		t.Fatalf("installation id=%d", installationID)
	}

	integration := domain.RepositoryIntegration{AuthMode: "github_app", InstallationID: 42}
	first, err := provider.Token(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.Token(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if first != "installation-token" || second != first {
		t.Fatalf("unexpected installation tokens: %q %q", first, second)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("expected cached installation token, API called %d times", tokenCalls.Load())
	}
}

func TestGitHubAuthProviderFallsBackToStoredTokenMode(t *testing.T) {
	provider, err := NewGitHubAuthProvider(fakeStoredGitHubTokenProvider{token: "stored-token"}, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	token, err := provider.Token(context.Background(), domain.RepositoryIntegration{AuthMode: "token", CredentialID: "cred-1"})
	if err != nil {
		t.Fatal(err)
	}
	if token != "stored-token" {
		t.Fatalf("unexpected token %q", token)
	}
}

func TestGitHubAuthProviderRequiresCompleteAppConfiguration(t *testing.T) {
	if _, err := NewGitHubAuthProvider(fakeStoredGitHubTokenProvider{}, "123", "", ""); err == nil {
		t.Fatal("expected incomplete GitHub App configuration to fail")
	}
}
