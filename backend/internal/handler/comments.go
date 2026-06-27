package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"idp-platform/backend/internal/comments"
	"idp-platform/backend/internal/httpjson"
)

type commentsHandler struct{ service *comments.Service }
type commentRequest struct {
	Content string `json:"content"`
}

func (h commentsHandler) listIDP(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, "idp", nestedEntityID(r, "/api/v1/idps/"))
}
func (h commentsHandler) createIDP(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "idp", nestedEntityID(r, "/api/v1/idps/"))
}
func (h commentsHandler) listTask(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, "task", nestedEntityID(r, "/api/v1/tasks/"))
}
func (h commentsHandler) createTask(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "task", nestedEntityID(r, "/api/v1/tasks/"))
}

func (h commentsHandler) list(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.List(r.Context(), access, entityType, entityID)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h commentsHandler) create(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	var req commentRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.service.Create(r.Context(), access, entityType, entityID, req.Content)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, result)
}

func (h commentsHandler) update(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	var req commentRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.service.Update(r.Context(), access, commentIDFromPath(r), req.Content)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h commentsHandler) delete(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	if err := h.service.Delete(r.Context(), access, commentIDFromPath(r)); err != nil {
		writeCommentError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func nestedEntityID(r *http.Request, prefix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/comments")
}
func commentIDFromPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/v1/comments/")
}

func writeCommentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, comments.ErrNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment or related entity not found")
	case errors.Is(err, comments.ErrForbidden):
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	case errors.Is(err, comments.ErrEditExpired):
		httpjson.WriteError(w, http.StatusConflict, "EDIT_WINDOW_EXPIRED", "Comment can only be edited within 10 minutes")
	case errors.Is(err, comments.ErrInvalidInput):
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Comment must contain from 1 to 5000 characters")
	default:
		slog.Error("comment request failed", "error", err)
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
