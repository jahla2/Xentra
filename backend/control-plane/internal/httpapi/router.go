package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
)

type Handler struct { environments *application.EnvironmentService; investigations *application.InvestigationService }
func NewRouter(environments *application.EnvironmentService, investigations *application.InvestigationService) http.Handler {
	h := &Handler{environments: environments, investigations: investigations}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status":"ok","service":"control-plane"}) })
	mux.HandleFunc("GET /api/environments", h.listEnvironments)
	mux.HandleFunc("POST /api/environments", h.createEnvironment)
	mux.HandleFunc("POST /api/investigations", h.investigate)
	return withCORS(mux)
}
func writeJSON(w http.ResponseWriter, status int, v any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(v) }
func (h *Handler) listEnvironments(w http.ResponseWriter, r *http.Request) { items, err := h.environments.List(r.Context()); if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }; writeJSON(w,200,items) }
func (h *Handler) createEnvironment(w http.ResponseWriter, r *http.Request) { var input application.CreateEnvironmentInput; if json.NewDecoder(r.Body).Decode(&input) != nil { writeJSON(w,400,map[string]string{"error":"invalid request"}); return }; env, err := h.environments.Create(r.Context(), input); if err != nil { writeJSON(w,502,map[string]string{"error":err.Error()}); return }; writeJSON(w,201,env) }
func (h *Handler) investigate(w http.ResponseWriter, r *http.Request) { var input struct { EnvironmentID string `json:"environmentId"`; Question string `json:"question"` }; if json.NewDecoder(r.Body).Decode(&input) != nil || input.Question == "" { writeJSON(w,400,map[string]string{"error":"environmentId and question are required"}); return }; result, err := h.investigations.Investigate(r.Context(), input.EnvironmentID, input.Question); if err != nil { writeJSON(w,502,map[string]string{"error":err.Error()}); return }; writeJSON(w,200,result) }
func withCORS(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Access-Control-Allow-Origin", "*"); w.Header().Set("Access-Control-Allow-Headers", "Content-Type"); w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS"); if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }; next.ServeHTTP(w,r) }) }
