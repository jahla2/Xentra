package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type EnvironmentRepository interface {
	Save(context.Context, domain.Environment) error
	Get(context.Context, string) (domain.Environment, error)
	List(context.Context) ([]domain.Environment, error)
}

type DiscoveryClient interface {
	Discover(context.Context, string) (domain.Discovery, error)
}

type CreateEnvironmentInput struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	RunnerURL string `json:"runnerUrl"`
}

type EnvironmentService struct {
	repo      EnvironmentRepository
	discovery DiscoveryClient
	counter   atomic.Uint64
}

func NewEnvironmentService(repo EnvironmentRepository, discovery DiscoveryClient) *EnvironmentService {
	return &EnvironmentService{repo: repo, discovery: discovery}
}

func (s *EnvironmentService) Create(ctx context.Context, input CreateEnvironmentInput) (domain.Environment, error) {
	if input.Name == "" || input.RunnerURL == "" {
		return domain.Environment{}, errors.New("name and runnerUrl are required")
	}
	discovered, err := s.discovery.Discover(ctx, input.RunnerURL)
	if err != nil {
		return domain.Environment{}, fmt.Errorf("discover environment: %w", err)
	}
	id := fmt.Sprintf("env-%d", s.counter.Add(1))
	env := domain.Environment{ID: id, Name: input.Name, Type: input.Type, RunnerURL: input.RunnerURL, OS: discovered.OS, Hostname: discovered.Hostname, Capabilities: discovered.Capabilities}
	if err := s.repo.Save(ctx, env); err != nil {
		return domain.Environment{}, err
	}
	return env, nil
}

func (s *EnvironmentService) List(ctx context.Context) ([]domain.Environment, error) { return s.repo.List(ctx) }

type MemoryEnvironmentRepository struct {
	mu   sync.RWMutex
	data map[string]domain.Environment
}

func NewMemoryEnvironmentRepository() *MemoryEnvironmentRepository {
	return &MemoryEnvironmentRepository{data: map[string]domain.Environment{}}
}
func (r *MemoryEnvironmentRepository) Save(_ context.Context, env domain.Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[env.ID] = env
	return nil
}
func (r *MemoryEnvironmentRepository) Get(_ context.Context, id string) (domain.Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	env, ok := r.data[id]
	if !ok {
		return domain.Environment{}, errors.New("environment not found")
	}
	return env, nil
}
func (r *MemoryEnvironmentRepository) List(_ context.Context) ([]domain.Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Environment, 0, len(r.data))
	for _, env := range r.data {
		result = append(result, env)
	}
	return result, nil
}
