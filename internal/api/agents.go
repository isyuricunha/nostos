package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/agents"
)

type agentsHandler struct {
	service *agents.Service
}

func newAgentsHandler(service *agents.Service) *agentsHandler {
	return &agentsHandler{service: service}
}

func (h *agentsHandler) Routes(r chi.Router) {
	r.Get("/agents", h.list)
	r.Post("/agents", h.create)
	r.Put("/agents/{agentID}", h.update)
	r.Post("/agents/{agentID}/duplicate", h.duplicate)
	r.Delete("/agents/{agentID}", h.delete)
}

func (h *agentsHandler) list(w http.ResponseWriter, r *http.Request) {
	if err := h.service.EnsureDefaultAgents(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "agents_failed", "Unable to prepare default agents.", nil)
		return
	}
	items, err := h.service.List(r.Context(), agentsPrincipal(r))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "agents_failed", "Unable to list agents.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": items})
}

func (h *agentsHandler) create(w http.ResponseWriter, r *http.Request) {
	var input agents.AgentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Create(r.Context(), agentsPrincipal(r), input)
	if err != nil {
		h.writeAgentError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agent": item})
}

func (h *agentsHandler) update(w http.ResponseWriter, r *http.Request) {
	var input agents.AgentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Update(r.Context(), agentsPrincipal(r), chi.URLParam(r, "agentID"), input)
	if err != nil {
		h.writeAgentError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": item})
}

func (h *agentsHandler) duplicate(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Duplicate(r.Context(), agentsPrincipal(r), chi.URLParam(r, "agentID"))
	if err != nil {
		h.writeAgentError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agent": item})
}

func (h *agentsHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), agentsPrincipal(r), chi.URLParam(r, "agentID")); err != nil {
		h.writeAgentError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *agentsHandler) writeAgentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, agents.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "agent_not_found", "The agent was not found.", nil)
	case errors.Is(err, agents.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_agent", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "agent_failed", "The agent request failed.", nil)
	}
}

func agentsPrincipal(r *http.Request) agents.PrincipalContext {
	principal := Principal(r.Context())
	return agents.PrincipalContext{WorkspaceID: principal.User.WorkspaceID, UserID: principal.User.ID}
}
