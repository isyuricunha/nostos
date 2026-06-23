package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/isyuricunha/nostos/internal/replies"
)

type repliesHandler struct {
	service *replies.Service
}

func newRepliesHandler(service *replies.Service) *repliesHandler {
	return &repliesHandler{service: service}
}

func (h *repliesHandler) Routes(r chi.Router) {
	r.Get("/reply-presets", h.listPresets)
	r.Post("/reply-presets", h.createPreset)
	r.Put("/reply-presets/{presetID}", h.updatePreset)
	r.Delete("/reply-presets/{presetID}", h.deletePreset)
	r.Post("/reply-presets/reset", h.resetDefaults)
	r.Get("/reply-drafts", h.listDrafts)
	r.Post("/reply-drafts", h.generateDraft)
}

func (h *repliesHandler) listPresets(w http.ResponseWriter, r *http.Request) {
	presets, err := h.service.ListPresets(r.Context(), repliesPrincipal(r))
	if err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": jsonSlice(presets)})
}

func (h *repliesHandler) createPreset(w http.ResponseWriter, r *http.Request) {
	var input replies.PresetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	preset, err := h.service.CreatePreset(r.Context(), repliesPrincipal(r), input)
	if err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"preset": preset})
}

func (h *repliesHandler) updatePreset(w http.ResponseWriter, r *http.Request) {
	var input replies.PresetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	preset, err := h.service.UpdatePreset(r.Context(), repliesPrincipal(r), chi.URLParam(r, "presetID"), input)
	if err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preset": preset})
}

func (h *repliesHandler) deletePreset(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeletePreset(r.Context(), repliesPrincipal(r), chi.URLParam(r, "presetID")); err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *repliesHandler) resetDefaults(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ResetDefaults(r.Context(), repliesPrincipal(r)); err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *repliesHandler) listDrafts(w http.ResponseWriter, r *http.Request) {
	drafts, err := h.service.ListDrafts(r.Context(), repliesPrincipal(r), r.URL.Query().Get("source_message_id"))
	if err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": jsonSlice(drafts)})
}

func (h *repliesHandler) generateDraft(w http.ResponseWriter, r *http.Request) {
	var input replies.DraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	draft, err := h.service.GenerateDraft(r.Context(), repliesPrincipal(r), input)
	if err != nil {
		h.writeReplyError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"draft": draft})
}

func (h *repliesHandler) writeReplyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, replies.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "reply_not_found", "The reply resource was not found.", nil)
	case errors.Is(err, replies.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_reply_request", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "reply_failed", "The reply request failed.", map[string]any{"reason": err.Error()})
	}
}

func repliesPrincipal(r *http.Request) replies.PrincipalContext {
	principal := Principal(r.Context())
	return replies.PrincipalContext{WorkspaceID: principal.User.WorkspaceID, UserID: principal.User.ID}
}
