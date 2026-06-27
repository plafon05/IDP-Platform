package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/tasks"
)

type tasksHandler struct{ service *tasks.Service }

type taskRequest struct {
	Title          string           `json:"title"`
	Description    *string          `json:"description"`
	CategoryID     *string          `json:"category_id"`
	Priority       string           `json:"priority"`
	DueDate        *string          `json:"due_date"`
	Status         string           `json:"status"`
	Progress       int              `json:"progress"`
	ManagerRating  *string          `json:"manager_rating"`
	ManagerComment *string          `json:"manager_comment"`
	CompetencyIDs  []string         `json:"competency_ids"`
	TagIDs         []string         `json:"tag_ids"`
	Resources      []tasks.Resource `json:"resources"`
}

type taskProgressRequest struct {
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
	SelfRating  *string `json:"self_rating"`
	SelfComment *string `json:"self_comment"`
}

func (h tasksHandler) list(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.List(r.Context(), access, idpIDFromTasksPath(r))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h tasksHandler) create(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	input, ok := decodeTaskInput(w, r)
	if !ok {
		return
	}
	result, err := h.service.Create(r.Context(), access, idpIDFromTasksPath(r), input)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, result)
}

func (h tasksHandler) get(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.Get(r.Context(), access, taskIDFromPath(r))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h tasksHandler) update(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	input, ok := decodeTaskInput(w, r)
	if !ok {
		return
	}
	result, err := h.service.Update(r.Context(), access, taskIDFromPath(r), input)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h tasksHandler) updateProgress(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	var req taskProgressRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.service.UpdateProgress(r.Context(), access, taskIDFromProgressPath(r), tasks.ProgressInput{
		Status: strings.TrimSpace(req.Status), Progress: req.Progress,
		SelfRating: emptyStringToNil(req.SelfRating), SelfComment: emptyStringToNil(req.SelfComment),
	})
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h tasksHandler) delete(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	if err := h.service.Delete(r.Context(), access, taskIDFromPath(r)); err != nil {
		writeTaskError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h tasksHandler) audit(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.Audit(r.Context(), access, taskIDFromAuditPath(r))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func decodeTaskInput(w http.ResponseWriter, r *http.Request) (tasks.Input, bool) {
	var req taskRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return tasks.Input{}, false
	}
	var dueDate *time.Time
	if req.DueDate != nil && strings.TrimSpace(*req.DueDate) != "" {
		parsed, err := time.Parse(time.DateOnly, strings.TrimSpace(*req.DueDate))
		if err != nil {
			httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "due_date must use YYYY-MM-DD format")
			return tasks.Input{}, false
		}
		dueDate = &parsed
	}
	return tasks.Input{
		Title: strings.TrimSpace(req.Title), Description: emptyStringToNil(req.Description),
		CategoryID: emptyStringToNil(req.CategoryID), Priority: strings.TrimSpace(req.Priority), DueDate: dueDate,
		Status: strings.TrimSpace(req.Status), Progress: req.Progress,
		ManagerRating: emptyStringToNil(req.ManagerRating), ManagerComment: emptyStringToNil(req.ManagerComment),
		CompetencyIDs: req.CompetencyIDs, TagIDs: req.TagIDs, Resources: req.Resources,
	}, true
}

func idpIDFromTasksPath(r *http.Request) string {
	return strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/idps/"), "/tasks")
}

func taskIDFromPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
}

func taskIDFromProgressPath(r *http.Request) string {
	return strings.TrimSuffix(taskIDFromPath(r), "/progress")
}

func taskIDFromAuditPath(r *http.Request) string {
	return strings.TrimSuffix(taskIDFromPath(r), "/audit")
}

func writeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "TASK_NOT_FOUND", "Task or IDP not found")
	case errors.Is(err, tasks.ErrForbidden):
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	case errors.Is(err, tasks.ErrIDPState):
		httpjson.WriteError(w, http.StatusConflict, "IDP_STATE_CONFLICT", "IDP status does not allow this task change")
	case errors.Is(err, tasks.ErrInvalidInput):
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid task data")
	default:
		slog.Error("task request failed", "error", err)
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
