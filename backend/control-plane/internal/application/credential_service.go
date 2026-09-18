package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type CredentialRepository interface {
	Save(context.Context, domain.CredentialRecord) error
	Get(context.Context, string) (domain.CredentialRecord, error)
}

type SecretBox interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

type CredentialService struct {
	repo CredentialRepository
	box  SecretBox
}

func NewCredentialService(repo CredentialRepository, box SecretBox) *CredentialService {
	return &CredentialService{repo: repo, box: box}
}

func (s *CredentialService) StoreSecret(ctx context.Context, kind string, plain []byte) (string, error) {
	if kind == "" || len(plain) == 0 {
		return "", errors.New("credential kind and value are required")
	}
	ciphertext, err := s.box.Encrypt(plain)
	if err != nil {
		return "", err
	}
	id, err := newCredentialID()
	if err != nil {
		return "", err
	}
	if err := s.repo.Save(ctx, domain.CredentialRecord{ID: id, Kind: kind, Ciphertext: ciphertext}); err != nil {
		return "", err
	}
	return id, nil
}

func (s *CredentialService) GetSecret(ctx context.Context, id, kind string) ([]byte, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Kind != kind {
		return nil, errors.New("credential kind mismatch")
	}
	return s.box.Decrypt(record.Ciphertext)
}

func (s *CredentialService) StoreSSH(ctx context.Context, credential domain.SSHCredential) (string, error) {
	if credential.PrivateKey == "" {
		return "", errors.New("ssh private key is required")
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		return "", err
	}
	return s.StoreSecret(ctx, "ssh", raw)
}

func (s *CredentialService) GetSSH(ctx context.Context, id string) (domain.SSHCredential, error) {
	raw, err := s.GetSecret(ctx, id, "ssh")
	if err != nil {
		return domain.SSHCredential{}, err
	}
	var credential domain.SSHCredential
	if err := json.Unmarshal(raw, &credential); err != nil {
		return domain.SSHCredential{}, err
	}
	return credential, nil
}

func (s *CredentialService) StoreGitHubToken(ctx context.Context, token string) (string, error) {
	return s.StoreSecret(ctx, "github-token", []byte(token))
}

func (s *CredentialService) GetGitHubToken(ctx context.Context, id string) (string, error) {
	raw, err := s.GetSecret(ctx, id, "github-token")
	return string(raw), err
}

func (s *CredentialService) StoreGitHubWebhookSecret(ctx context.Context, secret string) (string, error) {
	return s.StoreSecret(ctx, "github-webhook-secret", []byte(secret))
}

func (s *CredentialService) GetGitHubWebhookSecret(ctx context.Context, id string) (string, error) {
	raw, err := s.GetSecret(ctx, id, "github-webhook-secret")
	return string(raw), err
}

func newCredentialID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cred-" + hex.EncodeToString(buf), nil
}

type MemoryCredentialRepository struct {
	mu   sync.RWMutex
	data map[string]domain.CredentialRecord
}

func NewMemoryCredentialRepository() *MemoryCredentialRepository {
	return &MemoryCredentialRepository{data: map[string]domain.CredentialRecord{}}
}

func (r *MemoryCredentialRepository) Save(_ context.Context, record domain.CredentialRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyRecord := record
	copyRecord.Ciphertext = append([]byte(nil), record.Ciphertext...)
	r.data[record.ID] = copyRecord
	return nil
}

func (r *MemoryCredentialRepository) Get(_ context.Context, id string) (domain.CredentialRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.data[id]
	if !ok {
		return domain.CredentialRecord{}, errors.New("credential not found")
	}
	record.Ciphertext = append([]byte(nil), record.Ciphertext...)
	return record, nil
}
