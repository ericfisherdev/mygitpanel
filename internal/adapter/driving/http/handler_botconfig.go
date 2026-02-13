package httphandler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// ListBots returns all configured bot usernames.
func (h *Handler) ListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.botConfigStore.ListAll(r.Context())
	if err != nil {
		h.logger.Error("failed to list bots", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]BotConfigResponse, 0, len(bots))
	for _, bot := range bots {
		resp = append(resp, toBotConfigResponse(bot))
	}

	writeJSON(w, http.StatusOK, resp)
}

// AddBot adds a new bot username to the configuration.
func (h *Handler) AddBot(w http.ResponseWriter, r *http.Request) {
	var req AddBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	bot := model.BotConfig{
		Username: username,
		AddedAt:  time.Now().UTC(),
	}

	saved, err := h.botConfigStore.Add(r.Context(), bot)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, "bot username already exists")
			return
		}
		h.logger.Error("failed to add bot", "username", username, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, toBotConfigResponse(saved))
}

// RemoveBot removes a bot username from the configuration.
func (h *Handler) RemoveBot(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if err := h.botConfigStore.Remove(r.Context(), username); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "bot not found")
			return
		}
		h.logger.Error("failed to remove bot", "username", username, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
