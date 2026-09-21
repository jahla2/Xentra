package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jahla2/Xentra/backend/runner/internal/application"
	"github.com/jahla2/Xentra/backend/runner/internal/domain"
)

type Handler struct { discovery *application.DiscoveryService; tools *application.ToolService }
func NewRouter(discovery *application.DiscoveryService, tools *application.ToolService) http.Handler {
	h := &Handler{discovery: discovery, tools: tools}; mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health); mux.HandleFunc("GET /v1/discovery", h.discover); mux.HandleFunc("POST /v1/tools/execute", h.execute); return mux
}
func writeJSON(w http.ResponseWriter, status int, v any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(v) }
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status":"ok","service":"runner"}) }
func (h *Handler) discover(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, h.discovery.Discover(r.Context())) }
func (h *Handler) execute(w http.ResponseWriter, r *http.Request) { var req domain.ToolRequest; if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w,400,map[string]string{"error":"invalid request"}); return }; result := h.tools.Execute(r.Context(), req.Tool, req.Arguments); status := http.StatusOK; if !result.Success { status = http.StatusBadRequest }; writeJSON(w,status,result) }
