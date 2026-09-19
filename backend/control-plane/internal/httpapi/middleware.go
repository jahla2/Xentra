package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const maxRequestBodyBytes int64 = 2 << 20

type runtimeHTTPConfig struct {
	allowedOrigins map[string]struct{}
	authLimit      int
	apiLimit       int
	webhookLimit   int
	runnerLimit    int
	metricsToken   string
	trustProxyHeaders bool
}

func runtimeConfigFromEnv() runtimeHTTPConfig {
	origins := envList("XENTRA_ALLOWED_ORIGINS", []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	})
	return runtimeHTTPConfig{
		allowedOrigins: stringSet(origins),
		authLimit:      envInt("XENTRA_RATE_LIMIT_AUTH_PER_MINUTE", 20),
		apiLimit:       envInt("XENTRA_RATE_LIMIT_API_PER_MINUTE", 600),
		webhookLimit:   envInt("XENTRA_RATE_LIMIT_WEBHOOK_PER_MINUTE", 300),
		runnerLimit:    envInt("XENTRA_RATE_LIMIT_RUNNER_PER_MINUTE", 2400),
		metricsToken:   strings.TrimSpace(os.Getenv("XENTRA_METRICS_TOKEN")),
		trustProxyHeaders: envBool("XENTRA_TRUST_PROXY_HEADERS", false),
	}
}

func withProductionMiddleware(next http.Handler, metrics *httpMetrics, cfg runtimeHTTPConfig) http.Handler {
	apiLimiter := newFixedWindowLimiter(cfg.apiLimit, time.Minute)
	authLimiter := newFixedWindowLimiter(cfg.authLimit, time.Minute)
	webhookLimiter := newFixedWindowLimiter(cfg.webhookLimit, time.Minute)
	runnerLimiter := newFixedWindowLimiter(cfg.runnerLimit, time.Minute)

	handler := withRequestLogging(next, metrics, cfg.trustProxyHeaders)
	handler = withSecurityHeaders(handler)
	handler = withRequestLimits(handler)
	handler = withRateLimits(handler, apiLimiter, authLimiter, webhookLimiter, runnerLimiter, cfg.trustProxyHeaders)
	handler = withConfiguredCORS(handler, cfg.allowedOrigins)
	return handler
}

func withConfiguredCORS(next http.Handler, allowed map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin not allowed"})
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func withRequestLimits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func withRateLimits(
	next http.Handler,
	api, auth, webhook, runner *fixedWindowLimiter,
	trustProxyHeaders bool,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/internal/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		limiter := api
		scope := "api"
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/auth/"):
			limiter, scope = auth, "auth"
		case strings.HasPrefix(r.URL.Path, "/api/webhooks/"):
			limiter, scope = webhook, "webhook"
		case strings.HasPrefix(r.URL.Path, "/api/runners/"):
			limiter, scope = runner, "runner"
		}
		if !limiter.Allow(clientIP(r, trustProxyHeaders) + "|" + scope) {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type fixedWindowEntry struct {
	windowStart time.Time
	count       int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]fixedWindowEntry
	lastGC  time.Time
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	if limit < 1 {
		limit = 1
	}
	return &fixedWindowLimiter{
		limit: limit, window: window,
		entries: map[string]fixedWindowEntry{},
		lastGC: time.Now(),
	}
}

func (l *fixedWindowLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastGC) >= 5*l.window {
		for entryKey, entry := range l.entries {
			if now.Sub(entry.windowStart) >= 2*l.window {
				delete(l.entries, entryKey)
			}
		}
		l.lastGC = now
	}

	entry := l.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		entry = fixedWindowEntry{windowStart: now}
	}
	if entry.count >= l.limit {
		l.entries[key] = entry
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

type httpMetrics struct {
	requests       atomic.Uint64
	errors         atomic.Uint64
	inflight       atomic.Int64
	durationNanos  atomic.Uint64
}

func newHTTPMetrics() *httpMetrics { return &httpMetrics{} }

func (m *httpMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request, token string) {
	if token == "" {
		http.NotFound(w, r)
		return
	}
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "metrics authentication required"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	requests := m.requests.Load()
	durationSeconds := float64(m.durationNanos.Load()) / float64(time.Second)
	_, _ = fmt.Fprintf(w,
		"# TYPE xentra_http_requests_total counter\nxentra_http_requests_total %d\n"+
			"# TYPE xentra_http_errors_total counter\nxentra_http_errors_total %d\n"+
			"# TYPE xentra_http_inflight gauge\nxentra_http_inflight %d\n"+
			"# TYPE xentra_http_request_duration_seconds_total counter\nxentra_http_request_duration_seconds_total %.6f\n",
		requests, m.errors.Load(), m.inflight.Load(), durationSeconds,
	)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func withRequestLogging(next http.Handler, metrics *httpMetrics, trustProxyHeaders bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		started := time.Now()
		metrics.requests.Add(1)
		metrics.inflight.Add(1)
		defer metrics.inflight.Add(-1)

		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		duration := time.Since(started)
		metrics.durationNanos.Add(uint64(duration))
		if recorder.status >= 500 {
			metrics.errors.Add(1)
		}

		event := map[string]any{
			"event":       "http_request",
			"request_id":  requestID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"duration_ms": duration.Milliseconds(),
			"remote_ip":   clientIP(r, trustProxyHeaders),
		}
		spanContext := trace.SpanContextFromContext(r.Context())
		if spanContext.IsValid() {
			event["trace_id"] = spanContext.TraceID().String()
			event["span_id"] = spanContext.SpanID().String()
		}
		if traceparent := strings.TrimSpace(r.Header.Get("traceparent")); traceparent != "" {
			event["traceparent"] = traceparent
		}
		if raw, err := json.Marshal(event); err == nil {
			log.Print(string(raw))
		}
	})
}

func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func newRequestID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buffer)
}

func envList(name string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
