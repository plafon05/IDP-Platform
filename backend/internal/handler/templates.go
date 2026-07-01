package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/idp"
	"idp-platform/backend/internal/templates"
)

type templatesHandler struct{ service *templates.Service }
type applyTemplateRequest struct {
	EmployeeID string `json:"employee_id"`
	Title      string `json:"title"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

func templateAccess(w http.ResponseWriter, r *http.Request) (idp.Access, bool) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
	}
	return access, ok
}

func (h templatesHandler) list(w http.ResponseWriter, r *http.Request) {
	a, ok := templateAccess(w, r)
	if !ok {
		return
	}
	x, e := h.service.List(r.Context(), a)
	if e != nil {
		writeTemplateError(w, e)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, x)
}
func (h templatesHandler) create(w http.ResponseWriter, r *http.Request) {
	a, ok := templateAccess(w, r)
	if !ok {
		return
	}
	var in templates.Input
	if httpjson.DecodeJSON(r, &in) != nil {
		httpjson.WriteError(w, 400, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	x, e := h.service.Create(r.Context(), a, in)
	if e != nil {
		writeTemplateError(w, e)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, x)
}
func (h templatesHandler) update(w http.ResponseWriter, r *http.Request) {
	a, ok := templateAccess(w, r)
	if !ok {
		return
	}
	var in templates.Input
	if httpjson.DecodeJSON(r, &in) != nil {
		httpjson.WriteError(w, 400, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	x, e := h.service.Update(r.Context(), a, r.PathValue("id"), in)
	if e != nil {
		writeTemplateError(w, e)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, x)
}
func (h templatesHandler) archive(w http.ResponseWriter, r *http.Request) {
	a, ok := templateAccess(w, r)
	if !ok {
		return
	}
	if e := h.service.Archive(r.Context(), a, r.PathValue("id")); e != nil {
		writeTemplateError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h templatesHandler) apply(w http.ResponseWriter, r *http.Request) {
	a, ok := templateAccess(w, r)
	if !ok {
		return
	}
	var in applyTemplateRequest
	if httpjson.DecodeJSON(r, &in) != nil {
		httpjson.WriteError(w, 400, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	start, e1 := time.Parse(time.DateOnly, in.StartDate)
	end, e2 := time.Parse(time.DateOnly, in.EndDate)
	if e1 != nil || e2 != nil {
		writeTemplateError(w, templates.ErrInvalidInput)
		return
	}
	id, e := h.service.Apply(r.Context(), a, r.PathValue("id"), templates.ApplyInput{EmployeeID: strings.TrimSpace(in.EmployeeID), Title: strings.TrimSpace(in.Title), StartDate: start, EndDate: end})
	if e != nil {
		writeTemplateError(w, e)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}
func writeTemplateError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, templates.ErrForbidden):
		httpjson.WriteError(w, 403, "FORBIDDEN", "Insufficient permissions")
	case errors.Is(e, templates.ErrNotFound):
		httpjson.WriteError(w, 404, "NOT_FOUND", "Template not found")
	case errors.Is(e, templates.ErrInvalidInput):
		httpjson.WriteError(w, 400, "VALIDATION_ERROR", "Invalid template data")
	default:
		slog.Error("template request failed", "error", e)
		httpjson.WriteError(w, 500, "INTERNAL_ERROR", "Internal server error")
	}
}
