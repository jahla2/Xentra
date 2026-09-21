package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrEmailExists        = errors.New("email already exists")
	ErrForbidden          = errors.New("forbidden")
)

type AuthRepository interface {
	CreateAccount(context.Context, domain.User, domain.Organization, domain.Membership) error
	CreateMember(context.Context, domain.User, domain.Membership) error
	FindUserByEmail(context.Context, string) (domain.User, error)
	FindPrincipalByUser(context.Context, string) (domain.Principal, error)
	CreateSession(context.Context, domain.Session) error
	FindPrincipalBySessionHash(context.Context, string, time.Time) (domain.Principal, error)
	DeleteSession(context.Context, string) error
}

type AuthService struct {
	repo AuthRepository
	ttl  time.Duration
}

func NewAuthService(repo AuthRepository, ttl time.Duration) *AuthService {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &AuthService{repo: repo, ttl: ttl}
}

func (s *AuthService) Register(ctx context.Context, email, password, organizationName string) (domain.AuthSession, error) {
	email = normalizeEmail(email)
	organizationName = strings.TrimSpace(organizationName)
	if err := validateAccountInput(email, password); err != nil {
		return domain.AuthSession{}, err
	}
	if organizationName == "" {
		return domain.AuthSession{}, errors.New("organization name is required")
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.AuthSession{}, err
	}
	userID, err := newResourceID("usr")
	if err != nil {
		return domain.AuthSession{}, err
	}
	orgID, err := newResourceID("org")
	if err != nil {
		return domain.AuthSession{}, err
	}
	now := time.Now().UTC()
	user := domain.User{ID: userID, Email: email, PasswordHash: passwordHash, CreatedAt: now}
	org := domain.Organization{ID: orgID, Name: organizationName, CreatedAt: now}
	membership := domain.Membership{UserID: userID, OrganizationID: orgID, Role: domain.RoleOwner}
	if err := s.repo.CreateAccount(ctx, user, org, membership); err != nil {
		return domain.AuthSession{}, err
	}
	principal := domain.Principal{
		UserID: user.ID, Email: user.Email, OrganizationID: org.ID,
		OrganizationName: org.Name, Role: domain.RoleOwner,
	}
	return s.createSession(ctx, principal)
}

func (s *AuthService) CreateMember(ctx context.Context, owner domain.Principal, email, password string) (domain.User, error) {
	if owner.Role != domain.RoleOwner || owner.OrganizationID == "" {
		return domain.User{}, ErrForbidden
	}
	email = normalizeEmail(email)
	if err := validateAccountInput(email, password); err != nil {
		return domain.User{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	userID, err := newResourceID("usr")
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: userID, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	membership := domain.Membership{UserID: userID, OrganizationID: owner.OrganizationID, Role: domain.RoleMember}
	if err := s.repo.CreateMember(ctx, user, membership); err != nil {
		return domain.User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (domain.AuthSession, error) {
	user, err := s.repo.FindUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return domain.AuthSession{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return domain.AuthSession{}, ErrInvalidCredentials
	}
	principal, err := s.repo.FindPrincipalByUser(ctx, user.ID)
	if err != nil {
		return domain.AuthSession{}, ErrInvalidCredentials
	}
	return s.createSession(ctx, principal)
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if strings.TrimSpace(token) == "" {
		return domain.Principal{}, ErrInvalidSession
	}
	principal, err := s.repo.FindPrincipalBySessionHash(ctx, hashSessionToken(token), time.Now().UTC())
	if err != nil {
		return domain.Principal{}, ErrInvalidSession
	}
	return principal, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashSessionToken(token))
}

func (s *AuthService) createSession(ctx context.Context, principal domain.Principal) (domain.AuthSession, error) {
	rawToken, err := newSessionToken()
	if err != nil {
		return domain.AuthSession{}, err
	}
	sessionID, err := newResourceID("ses")
	if err != nil {
		return domain.AuthSession{}, err
	}
	now := time.Now().UTC()
	session := domain.Session{
		ID: sessionID, TokenHash: hashSessionToken(rawToken), UserID: principal.UserID,
		OrganizationID: principal.OrganizationID, ExpiresAt: now.Add(s.ttl), CreatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return domain.AuthSession{}, err
	}
	return domain.AuthSession{Token: rawToken, ExpiresAt: session.ExpiresAt, Principal: principal}, nil
}

func validateAccountInput(email, password string) error {
	if email == "" || !strings.Contains(email, "@") {
		return errors.New("valid email is required")
	}
	if len(password) < 10 {
		return errors.New("password must be at least 10 characters")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(value), err
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type MemoryAuthRepository struct {
	mu          sync.RWMutex
	users       map[string]domain.User
	userByEmail map[string]string
	orgs        map[string]domain.Organization
	memberships map[string]domain.Membership
	sessions    map[string]domain.Session
}

func NewMemoryAuthRepository() *MemoryAuthRepository {
	return &MemoryAuthRepository{
		users: map[string]domain.User{}, userByEmail: map[string]string{},
		orgs: map[string]domain.Organization{}, memberships: map[string]domain.Membership{},
		sessions: map[string]domain.Session{},
	}
}

func (r *MemoryAuthRepository) CreateAccount(_ context.Context, user domain.User, org domain.Organization, membership domain.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.userByEmail[user.Email]; exists {
		return ErrEmailExists
	}
	r.users[user.ID] = user
	r.userByEmail[user.Email] = user.ID
	r.orgs[org.ID] = org
	r.memberships[user.ID] = membership
	return nil
}

func (r *MemoryAuthRepository) CreateMember(_ context.Context, user domain.User, membership domain.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.userByEmail[user.Email]; exists {
		return ErrEmailExists
	}
	if _, exists := r.orgs[membership.OrganizationID]; !exists {
		return errors.New("organization not found")
	}
	r.users[user.ID] = user
	r.userByEmail[user.Email] = user.ID
	r.memberships[user.ID] = membership
	return nil
}

func (r *MemoryAuthRepository) FindUserByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.userByEmail[email]
	if !ok {
		return domain.User{}, ErrInvalidCredentials
	}
	return r.users[id], nil
}

func (r *MemoryAuthRepository) FindPrincipalByUser(_ context.Context, userID string) (domain.Principal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[userID]
	if !ok {
		return domain.Principal{}, ErrInvalidCredentials
	}
	membership, ok := r.memberships[userID]
	if !ok {
		return domain.Principal{}, ErrInvalidCredentials
	}
	org := r.orgs[membership.OrganizationID]
	return domain.Principal{
		UserID: user.ID, Email: user.Email, OrganizationID: org.ID,
		OrganizationName: org.Name, Role: membership.Role,
	}, nil
}

func (r *MemoryAuthRepository) CreateSession(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *MemoryAuthRepository) FindPrincipalBySessionHash(ctx context.Context, tokenHash string, now time.Time) (domain.Principal, error) {
	r.mu.RLock()
	session, ok := r.sessions[tokenHash]
	r.mu.RUnlock()
	if !ok || !session.ExpiresAt.After(now) {
		return domain.Principal{}, ErrInvalidSession
	}
	return r.FindPrincipalByUser(ctx, session.UserID)
}

func (r *MemoryAuthRepository) DeleteSession(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, tokenHash)
	return nil
}
