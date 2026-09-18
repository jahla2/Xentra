package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func (h *Handler) createRunnerEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		HealthURL string `json:"healthUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	enrollment, err := h.runners.CreateEnrollmentWithHealthURL(
		r.Context(), principal.OrganizationID, input.ProjectID, input.Name, input.Type, input.HealthURL,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, enrollment)
}

func (h *Handler) pollRunner(w http.ResponseWriter, r *http.Request) {
	token := runnerToken(r)
	var discovery domain.Discovery
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&discovery); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid discovery payload"})
			return
		}
	}
	task, found, err := h.runners.Poll(r.Context(), r.PathValue("id"), token, discovery)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) completeRunnerTask(w http.ResponseWriter, r *http.Request) {
	token := runnerToken(r)
	var result domain.RunnerTaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid result payload"})
		return
	}
	if err := h.runners.Complete(
		r.Context(), r.PathValue("id"), r.PathValue("taskId"), token, result,
	); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func runnerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Runner ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

