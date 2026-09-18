package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func TestRegisterLoginAuthenticateAndLogout(t *testing.T) {
	repo := NewMemoryAuthRepository()
	service := NewAuthService(repo, time.Hour)

	registered, err := service.Register(context.Background(), "Owner@Example.com", "very-secure-password", "Acme")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Principal.Role != domain.RoleOwner || registered.Principal.OrganizationName != "Acme" {
		t.Fatalf("unexpected session: %#v", registered)
	}

	principal, err := service.Authenticate(context.Background(), registered.Token)
	if err != nil || principal.Email != "owner@example.com" {
		t.Fatalf("authenticate failed: principal=%#v err=%v", principal, err)
	}

	member, err := service.CreateMember(context.Background(), principal, "member@example.com", "another-secure-password")
	if err != nil || member.Email != "member@example.com" {
		t.Fatalf("create member failed: %#v %v", member, err)
	}
	memberSession, err := service.Login(context.Background(), "member@example.com", "another-secure-password")
	if err != nil || memberSession.Principal.Role != domain.RoleMember {
		t.Fatalf("member login failed: %#v %v", memberSession, err)
	}

	loggedIn, err := service.Login(context.Background(), "owner@example.com", "very-secure-password")
	if err != nil || loggedIn.Token == "" {
		t.Fatalf("login failed: %#v %v", loggedIn, err)
	}

	if err := service.Logout(context.Background(), registered.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), registered.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session after logout, got %v", err)
	}
}

func TestMemberCannotCreateAnotherMember(t *testing.T) {
	repo := NewMemoryAuthRepository()
	service := NewAuthService(repo, time.Hour)
	ownerSession, _ := service.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")
	_, _ = service.CreateMember(context.Background(), ownerSession.Principal, "member@example.com", "another-secure-password")
	memberSession, _ := service.Login(context.Background(), "member@example.com", "another-secure-password")

	if _, err := service.CreateMember(context.Background(), memberSession.Principal, "other@example.com", "third-secure-password"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	service := NewAuthService(NewMemoryAuthRepository(), time.Hour)
	_, _ = service.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")

	if _, err := service.Login(context.Background(), "owner@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
