package httpapi

import (
	"net/http"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
)

func NewRunnerControlRouter(runners *application.RunnerControlService) http.Handler {
	h := &Handler{runners: runners}
	mux := http.NewServeMux()
	cfg := runtimeConfigFromEnv()
	metrics := newHTTPMetrics()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "runner-control"})
	})
	mux.HandleFunc("GET /internal/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.ServeHTTP(w, r, cfg.metricsToken)
	})
	mux.HandleFunc("POST /api/runners/{id}/poll", h.pollRunner)
	mux.HandleFunc("POST /api/runners/{id}/tasks/{taskId}/result", h.completeRunnerTask)

	return withProductionMiddleware(mux, metrics, cfg)
}
