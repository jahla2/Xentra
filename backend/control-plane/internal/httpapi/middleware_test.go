package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfiguredCORSAllowsConfiguredOriginAndRejectsOthers(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := withConfiguredCORS(next, stringSet([]string{"https://xentra.example.com"}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Origin", "https://xentra.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("allowed origin status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://xentra.example.com" {
		t.Fatalf("unexpected allow origin %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("forbidden origin status=%d", rec.Code)
	}
}

func TestClientIPOnlyTrustsForwardedHeaderWhenConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.8:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.5")

	if got := clientIP(req, false); got != "10.0.0.8" {
		t.Fatalf("untrusted proxy header changed client IP: %q", got)
	}
	if got := clientIP(req, true); got != "203.0.113.10" {
		t.Fatalf("trusted proxy header not used: %q", got)
	}
}

func TestRateLimiterRejectsAfterLimit(t *testing.T) {
	api := newFixedWindowLimiter(1, time.Minute)
	auth := newFixedWindowLimiter(1, time.Minute)
	webhook := newFixedWindowLimiter(1, time.Minute)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	runner := newFixedWindowLimiter(10, time.Minute)
	handler := withRateLimits(next, api, auth, webhook, runner, false)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.RemoteAddr = "10.0.0.9:1000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first request status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.RemoteAddr = "10.0.0.9:1001"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status=%d", rec.Code)
	}
}

func TestSecurityHeadersAreApplied(t *testing.T) {
	handler := withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY",
		"Referrer-Policy": "no-referrer",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Fatalf("%s=%q want %q", name, got, want)
		}
	}
}

func TestMetricsRequireBearerToken(t *testing.T) {
	metrics := newHTTPMetrics()
	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.ServeHTTP(rec, req, "secret")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	metrics.ServeHTTP(rec, req, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated metrics status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "xentra_http_requests_total") {
		t.Fatalf("metrics output missing request counter: %s", rec.Body.String())
	}
}
