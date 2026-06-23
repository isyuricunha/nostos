package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/isyuricunha/nostos/internal/memory"
)

type memoriesHandler struct {
	service *memory.Service
}

func newMemoriesHandler(service *memory.Service) *memoriesHandler {
	return &memoriesHandler{service: service}
}

func (h *memoriesHandler) Routes(r chi.Router) {
	r.Get("/memories", h.list)
	r.Post("/memories", h.create)
	r.Put("/memories/{memoryID}", h.update)
	r.Delete("/memories/{memoryID}", h.delete)
	r.Get("/chat-runs/{runID}/memories", h.usedByRun)
	r.Delete("/chat-runs/{runID}/memories/{memoryID}", h.removeFromRun)
}

func (h *memoriesHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), memoriesPrincipal(r), r.URL.Query().Get("search"))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "memories_failed", "Unable to list memories.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": jsonSlice(items)})
}

func (h *memoriesHandler) create(w http.ResponseWriter, r *http.Request) {
	var input memory.MemoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Create(r.Context(), memoriesPrincipal(r), input)
	if err != nil {
		h.writeMemoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"memory": item})
}

func (h *memoriesHandler) update(w http.ResponseWriter, r *http.Request) {
	var input memory.MemoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Update(r.Context(), memoriesPrincipal(r), chi.URLParam(r, "memoryID"), input)
	if err != nil {
		h.writeMemoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memory": item})
}

func (h *memoriesHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), memoriesPrincipal(r), chi.URLParam(r, "memoryID")); err != nil {
		h.writeMemoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *memoriesHandler) usedByRun(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.UsedByRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		h.writeMemoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": jsonSlice(items)})
}

func (h *memoriesHandler) removeFromRun(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveFromRun(r.Context(), chi.URLParam(r, "runID"), chi.URLParam(r, "memoryID")); err != nil {
		h.writeMemoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *memoriesHandler) writeMemoryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, memory.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "memory_not_found", "The memory was not found.", nil)
	case errors.Is(err, memory.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_memory", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "memory_failed", "The memory request failed.", nil)
	}
}

func memoriesPrincipal(r *http.Request) memory.PrincipalContext {
	principal := Principal(r.Context())
	return memory.PrincipalContext{WorkspaceID: principal.User.WorkspaceID, UserID: principal.User.ID}
}
