package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func TestProjectAPIIsOrganizationScopedAndOwnerManaged(t *testing.T) {
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

	projects := application.NewProjectService(application.NewMemoryProjectRepository())
	router := NewRouter(nil, nil, Services{Auth: auth, Projects: projects})

	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewBufferString(`{"name":"Core","description":"Core platform"}`))
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner create project status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created domain.Project
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "Core" {
		t.Fatalf("unexpected project: %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+member.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list project status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewBufferString(`{"name":"Forbidden"}`))
	req.Header.Set("Authorization", "Bearer "+member.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member create project status=%d body=%s", rec.Code, rec.Body.String())
	}
}
