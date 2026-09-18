package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
)

type Services struct {
	Integrations *application.IntegrationService
	Incidents    *application.IncidentService
	Actions      *application.ActionService
}

type Handler struct {
	environments   *application.EnvironmentService
	investigations *application.InvestigationService
	integrations   *application.IntegrationService
	incidents      *application.IncidentService
	actions        *application.ActionService
}

func NewRouter(environments *application.EnvironmentService, investigations *application.InvestigationService, extras ...Services) http.Handler {
	h := &Handler{environments: environments, investigations: investigations}
	if len(extras) > 0 {
		h.integrations = extras[0].Integrations
		h.incidents = extras[0].Incidents
		h.actions = extras[0].Actions
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status":"ok","service":"control-plane"})
	})
	if environments != nil {
		mux.HandleFunc("GET /api/environments", h.listEnvironments)
		mux.HandleFunc("POST /api/environments", h.createEnvironment)
	}
	if investigations != nil {
		mux.HandleFunc("POST /api/investigations", h.investigate)
	}
	if h.integrations != nil {
		mux.HandleFunc("POST /api/integrations/github", h.connectGitHub)
	}
	if h.incidents != nil {
		mux.HandleFunc("GET /api/incidents", h.listIncidents)
		mux.HandleFunc("POST /api/incidents", h.createIncident)
	}
	if h.actions != nil {
		mux.HandleFunc("POST /api/actions", h.proposeAction)
		mux.HandleFunc("POST /api/actions/{id}/approve", h.approveAction)
		mux.HandleFunc("GET /api/audit", h.listAudit)
	}
	return withCORS(mux)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) listEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := h.environments.List(r.Context())
	if err != nil { writeJSON(w, 500, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, items)
}

func (h *Handler) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var input application.CreateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid request"}); return }
	env, err := h.environments.Create(r.Context(), input)
	if err != nil { writeJSON(w, 502, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 201, env)
}

func (h *Handler) investigate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.EnvironmentID == "" || input.Question == "" {
		writeJSON(w, 400, map[string]string{"error":"environmentId and question are required"}); return
	}
	result, err := h.investigations.Investigate(r.Context(), input.EnvironmentID, input.Question)
	if err != nil { writeJSON(w, 502, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, result)
}

func (h *Handler) connectGitHub(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Owner string `json:"owner"`
		Repo string `json:"repo"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid request"}); return }
	item, err := h.integrations.ConnectGitHub(r.Context(), input.EnvironmentID, input.Owner, input.Repo, input.AccessToken)
	if err != nil { writeJSON(w, 400, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 201, item)
}

func (h *Handler) createIncident(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid request"}); return }
	incident, err := h.incidents.Create(r.Context(), input.EnvironmentID, input.Question)
	if err != nil { writeJSON(w, 502, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 201, incident)
}

func (h *Handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	items, err := h.incidents.List(r.Context())
	if err != nil { writeJSON(w, 500, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, items)
}

func (h *Handler) proposeAction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IncidentID string `json:"incidentId"`
		EnvironmentID string `json:"environmentId"`
		Action string `json:"action"`
		Target string `json:"target"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid request"}); return }
	action, err := h.actions.Propose(r.Context(), input.IncidentID, input.EnvironmentID, input.Action, input.Target, input.Reason)
	if err != nil { writeJSON(w, 400, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 201, action)
}

func (h *Handler) approveAction(w http.ResponseWriter, r *http.Request) {
	var input struct { ApprovedBy string `json:"approvedBy"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid request"}); return }
	action, err := h.actions.Approve(r.Context(), r.PathValue("id"), input.ApprovedBy)
	if err != nil { writeJSON(w, 400, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, action)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	items, err := h.actions.Audit(r.Context())
	if err != nil { writeJSON(w, 500, map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, items)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}
