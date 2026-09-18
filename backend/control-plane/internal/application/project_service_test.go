package application

import (
	"context"
	"testing"
)

func TestProjectsAreOrganizationScoped(t *testing.T) {
	repo := NewMemoryProjectRepository()
	service := NewProjectService(repo)

	a, err := service.Create(context.Background(), "org-a", "API", "Production API")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), "org-b", "Other", ""); err != nil {
		t.Fatal(err)
	}

	items, err := service.List(context.Background(), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != a.ID {
		t.Fatalf("unexpected org-a projects: %#v", items)
	}
	if _, err := repo.Get(context.Background(), "org-b", a.ID); err == nil {
		t.Fatal("expected cross-organization project lookup to fail")
	}
}
