package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type principalContextKey struct{}

func withAuthentication(next http.Handler, auth *application.AuthService) http.Handler {
	if auth == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r)
		principal, err := auth.Authenticate(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicRoute(r *http.Request) bool {
	if r.Method == http.MethodGet && (r.URL.Path == "/health" || r.URL.Path == "/internal/metrics") {
		return true
	}
	if r.Method == http.MethodPost && (r.URL.Path == "/api/auth/register" || r.URL.Path == "/api/auth/login") {
		return true
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/webhooks/github/") {
		return true
	}
	return false
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func principalFromRequest(r *http.Request) (domain.Principal, bool) {
	principal, ok := r.Context().Value(principalContextKey{}).(domain.Principal)
	return principal, ok
}

func requireOwner(w http.ResponseWriter, r *http.Request) (domain.Principal, bool) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return domain.Principal{}, false
	}
	if principal.Role != domain.RoleOwner {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner role required"})
		return domain.Principal{}, false
	}
	return principal, true
}
