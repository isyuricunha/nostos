package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/mcp"
)

type mcpHandler struct {
	service *mcp.Service
}

func newMCPHandler(service *mcp.Service) *mcpHandler {
	return &mcpHandler{service: service}
}

func (h *mcpHandler) Routes(r chi.Router) {
	r.Get("/mcp-servers", h.listServers)
	r.Post("/mcp-servers", h.createServer)
	r.Put("/mcp-servers/{serverID}", h.updateServer)
	r.Delete("/mcp-servers/{serverID}", h.deleteServer)
	r.Post("/mcp-servers/{serverID}/test", h.discoverTools)
	r.Post("/mcp-servers/{serverID}/discover", h.discoverTools)
	r.Get("/mcp-tools", h.listTools)
	r.Put("/mcp-tools/{toolID}/permission", h.updateToolPermission)
}

func (h *mcpHandler) listServers(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListServers(r.Context(), mcpPrincipal(r))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "mcp_servers_failed", "Unable to list MCP servers.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": items})
}

func (h *mcpHandler) createServer(w http.ResponseWriter, r *http.Request) {
	var input mcp.ServerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.CreateServer(r.Context(), mcpPrincipal(r), input)
	if err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"server": item})
}

func (h *mcpHandler) updateServer(w http.ResponseWriter, r *http.Request) {
	var input mcp.ServerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.UpdateServer(r.Context(), mcpPrincipal(r), chi.URLParam(r, "serverID"), input)
	if err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": item})
}

func (h *mcpHandler) deleteServer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteServer(r.Context(), mcpPrincipal(r), chi.URLParam(r, "serverID")); err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *mcpHandler) discoverTools(w http.ResponseWriter, r *http.Request) {
	tools, err := h.service.DiscoverTools(r.Context(), mcpPrincipal(r), chi.URLParam(r, "serverID"))
	if err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (h *mcpHandler) listTools(w http.ResponseWriter, r *http.Request) {
	tools, err := h.service.ListTools(r.Context(), mcpPrincipal(r), r.URL.Query().Get("server_id"))
	if err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (h *mcpHandler) updateToolPermission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PermissionMode string `json:"permission_mode"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := h.service.UpdateToolPermission(r.Context(), mcpPrincipal(r), chi.URLParam(r, "toolID"), input.PermissionMode); err != nil {
		h.writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *mcpHandler) writeMCPError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, mcp.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "mcp_not_found", "The MCP resource was not found.", nil)
	case errors.Is(err, mcp.ErrInvalidInput), errors.Is(err, mcp.ErrSecretKey):
		writeError(w, r, http.StatusBadRequest, "invalid_mcp_request", err.Error(), nil)
	default:
		writeError(w, r, http.StatusBadGateway, "mcp_unavailable", "The MCP server is unavailable.", map[string]any{"reason": err.Error()})
	}
}

func mcpPrincipal(r *http.Request) mcp.PrincipalContext {
	principal := Principal(r.Context())
	return mcp.PrincipalContext{
		WorkspaceID: principal.User.WorkspaceID,
		UserID:      principal.User.ID,
		IPAddress:   clientIP(r),
		UserAgent:   r.UserAgent(),
	}
}
