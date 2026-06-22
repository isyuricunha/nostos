package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/feedback"
)

type feedbackHandler struct {
	service *feedback.Service
}

func newFeedbackHandler(service *feedback.Service) *feedbackHandler {
	return &feedbackHandler{service: service}
}

func (h *feedbackHandler) Routes(r chi.Router) {
	r.Get("/feedback", h.list)
	r.Get("/feedback/stats", h.stats)
	r.Put("/messages/{messageID}/feedback", h.upsert)
	r.Delete("/messages/{messageID}/feedback", h.delete)
}

func (h *feedbackHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListForConversation(r.Context(), feedbackPrincipal(r), r.URL.Query().Get("conversation_id"))
	if err != nil {
		h.writeFeedbackError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feedback": items})
}

func (h *feedbackHandler) upsert(w http.ResponseWriter, r *http.Request) {
	var input feedback.FeedbackInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Upsert(r.Context(), feedbackPrincipal(r), chi.URLParam(r, "messageID"), input)
	if err != nil {
		h.writeFeedbackError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feedback": item})
}

func (h *feedbackHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), feedbackPrincipal(r), chi.URLParam(r, "messageID")); err != nil {
		h.writeFeedbackError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *feedbackHandler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context(), feedbackPrincipal(r))
	if err != nil {
		h.writeFeedbackError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

func (h *feedbackHandler) writeFeedbackError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, feedback.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "feedback_not_found", "The feedback resource was not found.", nil)
	case errors.Is(err, feedback.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_feedback", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "feedback_failed", "The feedback request failed.", map[string]any{"reason": err.Error()})
	}
}

func feedbackPrincipal(r *http.Request) feedback.PrincipalContext {
	principal := Principal(r.Context())
	return feedback.PrincipalContext{WorkspaceID: principal.User.WorkspaceID, UserID: principal.User.ID}
}
