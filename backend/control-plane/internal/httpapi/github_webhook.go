package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
)

const maxGitHubWebhookBody = 2 << 20

func (h *Handler) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhooks == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook service unavailable"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubWebhookBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook body"})
		return
	}

	result, err := h.webhooks.Process(
		r.Context(),
		r.PathValue("id"),
		r.Header.Get("X-GitHub-Event"),
		r.Header.Get("X-GitHub-Delivery"),
		r.Header.Get("X-Hub-Signature-256"),
		body,
	)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, application.ErrInvalidWebhookSignature) {
			status = http.StatusUnauthorized
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if result.Duplicate || result.Ignored {
		writeJSON(w, http.StatusAccepted, result)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
