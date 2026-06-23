package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/isyuricunha/nostos/internal/providers"
)

type providersHandler struct {
	service *providers.Service
}

func newProvidersHandler(service *providers.Service) *providersHandler {
	return &providersHandler{service: service}
}

func (h *providersHandler) Routes(r chi.Router) {
	r.Get("/providers", h.list)
	r.Post("/providers", h.create)
	r.Route("/providers/{providerID}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
		r.Post("/test", h.test)
		r.Post("/models/refresh", h.refreshModels)
		r.Get("/models", h.models)
	})
	r.Get("/models", h.modelsByQuery)
}

func (h *providersHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), providerPrincipal(r))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "providers_failed", "Unable to list providers.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": jsonSlice(items)})
}

func (h *providersHandler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID"))
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": item})
}

func (h *providersHandler) create(w http.ResponseWriter, r *http.Request) {
	var input providers.ProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Create(r.Context(), providerPrincipal(r), input)
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"provider": item})
}

func (h *providersHandler) update(w http.ResponseWriter, r *http.Request) {
	var input providers.ProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := h.service.Update(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID"), input)
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": item})
}

func (h *providersHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID")); err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *providersHandler) test(w http.ResponseWriter, r *http.Request) {
	if err := h.service.TestConnection(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID")); err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *providersHandler) refreshModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.service.RefreshModels(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID"))
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": jsonSlice(models)})
}

func (h *providersHandler) models(w http.ResponseWriter, r *http.Request) {
	models, err := h.service.ListModels(r.Context(), providerPrincipal(r), chi.URLParam(r, "providerID"))
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": jsonSlice(models)})
}

func (h *providersHandler) modelsByQuery(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	models, err := h.service.ListModels(r.Context(), providerPrincipal(r), providerID)
	if err != nil {
		h.writeProviderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": jsonSlice(models)})
}

func (h *providersHandler) writeProviderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, providers.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "provider_not_found", "The provider was not found.", nil)
	case errors.Is(err, providers.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_provider", err.Error(), nil)
	case errors.Is(err, providers.ErrSecretMissing):
		writeError(w, r, http.StatusBadRequest, "provider_secret_missing", err.Error(), nil)
	default:
		writeError(w, r, http.StatusBadGateway, "provider_unavailable", "The selected provider is unavailable.", map[string]any{"reason": err.Error()})
	}
}

func providerPrincipal(r *http.Request) providers.PrincipalContext {
	principal := Principal(r.Context())
	return providers.PrincipalContext{
		WorkspaceID: principal.User.WorkspaceID,
		UserID:      principal.User.ID,
		IPAddress:   clientIP(r),
		UserAgent:   r.UserAgent(),
	}
}
