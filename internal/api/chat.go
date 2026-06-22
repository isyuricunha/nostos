package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/chat"
)

type chatHandler struct {
	service *chat.Service
}

func newChatHandler(service *chat.Service) *chatHandler {
	return &chatHandler{service: service}
}

func (h *chatHandler) Routes(r chi.Router) {
	r.Get("/conversations", h.listConversations)
	r.Post("/conversations", h.createConversation)
	r.Route("/conversations/{conversationID}", func(r chi.Router) {
		r.Get("/", h.getConversation)
		r.Patch("/", h.updateConversation)
		r.Delete("/", h.deleteConversation)
		r.Get("/messages", h.listMessages)
		r.Post("/runs", h.run)
	})
	r.Post("/chat-runs/{runID}/cancel", h.cancelRun)
	r.Post("/messages/{messageID}/regenerate", h.regenerate)
	r.Patch("/messages/{messageID}", h.editMessage)
}

func (h *chatHandler) listConversations(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListConversations(r.Context(), chatPrincipal(r), r.URL.Query().Get("search"))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "conversations_failed", "Unable to list conversations.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (h *chatHandler) createConversation(w http.ResponseWriter, r *http.Request) {
	var input chat.Conversation
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.CreateConversation(r.Context(), chatPrincipal(r), input)
	if err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": item})
}

func (h *chatHandler) getConversation(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetConversation(r.Context(), chatPrincipal(r), chi.URLParam(r, "conversationID"))
	if err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": item})
}

func (h *chatHandler) updateConversation(w http.ResponseWriter, r *http.Request) {
	var input chat.UpdateConversationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.UpdateConversation(r.Context(), chatPrincipal(r), chi.URLParam(r, "conversationID"), input)
	if err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": item})
}

func (h *chatHandler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteConversation(r.Context(), chatPrincipal(r), chi.URLParam(r, "conversationID")); err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *chatHandler) listMessages(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListMessages(r.Context(), chatPrincipal(r), chi.URLParam(r, "conversationID"))
	if err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": items})
}

func (h *chatHandler) run(w http.ResponseWriter, r *http.Request) {
	var input chat.RunInput
	if !decodeJSON(w, r, &input) {
		return
	}
	h.stream(w, r, func(sink chat.StreamSink) error {
		return h.service.Run(r.Context(), chatPrincipal(r), chi.URLParam(r, "conversationID"), input, sink)
	})
}

func (h *chatHandler) cancelRun(w http.ResponseWriter, r *http.Request) {
	if err := h.service.CancelRun(r.Context(), chatPrincipal(r), chi.URLParam(r, "runID")); err != nil {
		h.writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *chatHandler) regenerate(w http.ResponseWriter, r *http.Request) {
	var input chat.RunInput
	if !decodeJSON(w, r, &input) {
		return
	}
	h.stream(w, r, func(sink chat.StreamSink) error {
		return h.service.Regenerate(r.Context(), chatPrincipal(r), chi.URLParam(r, "messageID"), input, sink)
	})
}

func (h *chatHandler) editMessage(w http.ResponseWriter, r *http.Request) {
	var input chat.RunInput
	if !decodeJSON(w, r, &input) {
		return
	}
	h.stream(w, r, func(sink chat.StreamSink) error {
		return h.service.EditAndBranch(r.Context(), chatPrincipal(r), chi.URLParam(r, "messageID"), input, sink)
	})
}

func (h *chatHandler) stream(w http.ResponseWriter, r *http.Request, run func(chat.StreamSink) error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "stream_unavailable", "Streaming is not available.", nil)
		return
	}
	sink := func(event string, payload any) error {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if err := run(sink); err != nil {
		_ = sink("run_failed", map[string]string{"message": err.Error()})
	}
}

func (h *chatHandler) writeChatError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, chat.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "chat_not_found", "The chat resource was not found.", nil)
	case errors.Is(err, chat.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_chat_request", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "chat_failed", "The chat request failed.", map[string]any{"reason": err.Error()})
	}
}

func chatPrincipal(r *http.Request) chat.PrincipalContext {
	principal := Principal(r.Context())
	return chat.PrincipalContext{WorkspaceID: principal.User.WorkspaceID, UserID: principal.User.ID}
}
