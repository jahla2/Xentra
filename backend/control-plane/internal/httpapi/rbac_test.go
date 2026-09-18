package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type rbacDiscovery struct{}

func (rbacDiscovery) Discover(context.Context, domain.Environment) (domain.Discovery, error) {
	return domain.Discovery{OS: "linux", Hostname: "host"}, nil
}

func TestMemberCanReadButCannotCreateEnvironment(t *testing.T) {
	auth := application.NewAuthService(application.NewMemoryAuthRepository(), time.Hour)
	owner, err := auth.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CreateMember(context.Background(), owner.Principal, "member@example.com", "another-secure-password"); err != nil {
		t.Fatal(err)
	}
	member, err := auth.Login(context.Background(), "member@example.com", "another-secure-password")
	if err != nil {
		t.Fatal(err)
	}

	environments := application.NewEnvironmentService(application.NewMemoryEnvironmentRepository(), rbacDiscovery{}, nil)
	router := NewRouter(environments, nil, Services{Auth: auth})

	req := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	req.Header.Set("Authorization", "Bearer "+member.Token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("member read status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := bytes.NewBufferString(`{"name":"Prod","connectionType":"runner","runnerUrl":"http://runner:8090"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/environments", body)
	req.Header.Set("Authorization", "Bearer "+member.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member create status=%d body=%s", rec.Code, rec.Body.String())
	}
}
