package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/tasks"
)

type tasksHandler struct {
	service *tasks.Service
}

func newTasksHandler(service *tasks.Service) *tasksHandler {
	return &tasksHandler{service: service}
}

func (h *tasksHandler) Routes(r chi.Router) {
	r.Get("/tasks", h.listTasks)
	r.Post("/tasks", h.createTask)
	r.Put("/tasks/{taskID}", h.updateTask)
	r.Delete("/tasks/{taskID}", h.deleteTask)
	r.Post("/tasks/{taskID}/run", h.runNow)
	r.Get("/task-runs", h.listRuns)
	r.Get("/task-runs/{runID}", h.getRun)
	r.Post("/task-runs/{runID}/cancel", h.cancelRun)
	r.Post("/task-runs/{runID}/retry", h.retryRun)
}

func (h *tasksHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.ListTaskRecords(r.Context(), tasksPrincipal(r))
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": records})
}

func (h *tasksHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var input tasks.TaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, schedule, err := h.service.CreateTask(r.Context(), tasksPrincipal(r), input)
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"task": task, "schedule": schedule})
}

func (h *tasksHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	var input tasks.TaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, schedule, err := h.service.UpdateTask(r.Context(), tasksPrincipal(r), chi.URLParam(r, "taskID"), input)
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task, "schedule": schedule})
}

func (h *tasksHandler) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteTask(r.Context(), tasksPrincipal(r), chi.URLParam(r, "taskID")); err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *tasksHandler) runNow(w http.ResponseWriter, r *http.Request) {
	run, err := h.service.RunNow(r.Context(), tasksPrincipal(r), chi.URLParam(r, "taskID"))
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

func (h *tasksHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.service.ListRuns(r.Context(), tasksPrincipal(r), r.URL.Query().Get("task_id"))
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (h *tasksHandler) getRun(w http.ResponseWriter, r *http.Request) {
	record, err := h.service.GetRunRecord(r.Context(), tasksPrincipal(r), chi.URLParam(r, "runID"))
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *tasksHandler) cancelRun(w http.ResponseWriter, r *http.Request) {
	if err := h.service.CancelRun(r.Context(), tasksPrincipal(r), chi.URLParam(r, "runID")); err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *tasksHandler) retryRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.service.RetryRun(r.Context(), tasksPrincipal(r), chi.URLParam(r, "runID"))
	if err != nil {
		h.writeTaskError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

func (h *tasksHandler) writeTaskError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "task_not_found", "The task resource was not found.", nil)
	case errors.Is(err, tasks.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_task", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "task_failed", "The task request failed.", map[string]any{"reason": err.Error()})
	}
}

func tasksPrincipal(r *http.Request) tasks.PrincipalContext {
	principal := Principal(r.Context())
	return tasks.PrincipalContext{
		WorkspaceID: principal.User.WorkspaceID,
		UserID:      principal.User.ID,
		IPAddress:   clientIP(r),
		UserAgent:   r.UserAgent(),
	}
}
