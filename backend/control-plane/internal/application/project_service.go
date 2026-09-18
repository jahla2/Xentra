package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ProjectRepository interface {
	Save(context.Context, domain.Project) error
	Get(context.Context, string, string) (domain.Project, error)
	List(context.Context, string) ([]domain.Project, error)
}

type ProjectService struct {
	repo ProjectRepository
}

func NewProjectService(repo ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(ctx context.Context, organizationID, name, description string) (domain.Project, error) {
	name = strings.TrimSpace(name)
	if organizationID == "" {
		return domain.Project{}, errors.New("organization is required")
	}
	if name == "" {
		return domain.Project{}, errors.New("project name is required")
	}
	id, err := newResourceID("prj")
	if err != nil {
		return domain.Project{}, err
	}
	project := domain.Project{
		ID: id, OrganizationID: organizationID, Name: name,
		Description: strings.TrimSpace(description), CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, project); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func (s *ProjectService) List(ctx context.Context, organizationID string) ([]domain.Project, error) {
	return s.repo.List(ctx, organizationID)
}

type MemoryProjectRepository struct {
	mu   sync.RWMutex
	data map[string]domain.Project
}

func NewMemoryProjectRepository() *MemoryProjectRepository {
	return &MemoryProjectRepository{data: map[string]domain.Project{}}
}

func (r *MemoryProjectRepository) Save(_ context.Context, item domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[item.ID] = item
	return nil
}

func (r *MemoryProjectRepository) Get(_ context.Context, organizationID, id string) (domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.data[id]
	if !ok || item.OrganizationID != organizationID {
		return domain.Project{}, errors.New("project not found")
	}
	return item, nil
}

func (r *MemoryProjectRepository) List(_ context.Context, organizationID string) ([]domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.Project{}
	for _, item := range r.data {
		if item.OrganizationID == organizationID {
			items = append(items, item)
		}
	}
	return items, nil
}
