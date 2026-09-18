package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
)

type Services struct {
	Auth         *application.AuthService
	Projects     *application.ProjectService
	Integrations *application.IntegrationService
	Webhooks     *application.GitHubWebhookService
	Incidents    *application.IncidentService
	Actions      *application.ActionService
}

type Handler struct {
	auth           *application.AuthService
	projects       *application.ProjectService
	environments   *application.EnvironmentService
	investigations *application.InvestigationService
	integrations   *application.IntegrationService
	webhooks       *application.GitHubWebhookService
	incidents      *application.IncidentService
	actions        *application.ActionService
}

func NewRouter(environments *application.EnvironmentService, investigations *application.InvestigationService, extras ...Services) http.Handler {
	h := &Handler{environments: environments, investigations: investigations}
	if len(extras) > 0 {
		h.auth = extras[0].Auth
		h.projects = extras[0].Projects
		h.integrations = extras[0].Integrations
		h.webhooks = extras[0].Webhooks
		h.incidents = extras[0].Incidents
		h.actions = extras[0].Actions
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "control-plane"})
	})

	if h.auth != nil {
		mux.HandleFunc("POST /api/auth/register", h.register)
		mux.HandleFunc("POST /api/auth/login", h.login)
		mux.HandleFunc("POST /api/auth/logout", h.logout)
		mux.HandleFunc("GET /api/auth/me", h.me)
		mux.HandleFunc("POST /api/auth/members", h.createMember)
	}
	if h.projects != nil {
		mux.HandleFunc("GET /api/projects", h.listProjects)
		mux.HandleFunc("POST /api/projects", h.createProject)
	}
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
	if h.webhooks != nil {
		mux.HandleFunc("POST /api/webhooks/github/{id}", h.handleGitHubWebhook)
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

	return withCORS(withAuthentication(mux, h.auth))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		OrganizationName string `json:"organizationName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	session, err := h.auth.Register(r.Context(), input.Email, input.Password, input.OrganizationName)
	if err != nil {
		writeJSON(w, authErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	session, err := h.auth.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeJSON(w, authErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context(), bearerToken(r)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

func (h *Handler) createMember(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	user, err := h.auth.CreateMember(r.Context(), principal, input.Email, input.Password)
	if err != nil {
		writeJSON(w, authErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	items, err := h.projects.List(r.Context(), principal.OrganizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	item, err := h.projects.Create(r.Context(), principal.OrganizationID, input.Name, input.Description)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) listEnvironments(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	items, err := h.environments.List(r.Context(), principal.OrganizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createEnvironment(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input application.CreateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	env, err := h.environments.Create(r.Context(), principal.OrganizationID, input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, env)
}

func (h *Handler) investigate(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Question      string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.EnvironmentID == "" || input.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environmentId and question are required"})
		return
	}
	result, err := h.investigations.Investigate(r.Context(), principal.OrganizationID, input.EnvironmentID, input.Question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) connectGitHub(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Owner         string `json:"owner"`
		Repo          string `json:"repo"`
		AccessToken   string `json:"accessToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	item, err := h.integrations.ConnectGitHub(r.Context(), principal.OrganizationID, input.EnvironmentID, input.Owner, input.Repo, input.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) createIncident(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var input struct {
		EnvironmentID string `json:"environmentId"`
		Question      string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	incident, err := h.incidents.Create(r.Context(), principal.OrganizationID, input.EnvironmentID, input.Question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, incident)
}

func (h *Handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	items, err := h.incidents.List(r.Context(), principal.OrganizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) proposeAction(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		IncidentID    string `json:"incidentId"`
		EnvironmentID string `json:"environmentId"`
		Action        string `json:"action"`
		Target        string `json:"target"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	action, err := h.actions.Propose(r.Context(), principal.OrganizationID, input.IncidentID, input.EnvironmentID, input.Action, input.Target, input.Reason)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, action)
}

func (h *Handler) approveAction(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	action, err := h.actions.Approve(r.Context(), principal.OrganizationID, r.PathValue("id"), principal.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, action)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	items, err := h.actions.Audit(r.Context(), principal.OrganizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func authErrorStatus(err error) int {
	switch {
	case errors.Is(err, application.ErrInvalidCredentials), errors.Is(err, application.ErrInvalidSession):
		return http.StatusUnauthorized
	case errors.Is(err, application.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, application.ErrEmailExists):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
