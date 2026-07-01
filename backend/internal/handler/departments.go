package handler

import (
	"errors"
	"net/http"
	"strings"

	"idp-platform/backend/internal/departments"
	"idp-platform/backend/internal/httpjson"
)

type departmentsHandler struct{ service *departments.Service }
type departmentRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

func (h departmentsHandler) list(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Tree(r.Context())
	if err != nil {
		writeDepartmentError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}
func (h departmentsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req departmentRequest
	if httpjson.DecodeJSON(r, &req) != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.service.Create(r.Context(), departments.Input{Name: req.Name, ParentID: emptyStringToNil(req.ParentID)})
	if err != nil {
		writeDepartmentError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, result)
}
func (h departmentsHandler) update(w http.ResponseWriter, r *http.Request) {
	var req departmentRequest
	if httpjson.DecodeJSON(r, &req) != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	err := h.service.Update(r.Context(), r.PathValue("id"), departments.Input{Name: req.Name, ParentID: emptyStringToNil(req.ParentID)})
	if err != nil {
		writeDepartmentError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h departmentsHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeDepartmentError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func writeDepartmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, departments.ErrNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "Department not found")
	case errors.Is(err, departments.ErrInvalid):
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_HIERARCHY", "Department hierarchy is invalid")
	case errors.Is(err, departments.ErrInUse):
		httpjson.WriteError(w, http.StatusConflict, "DEPARTMENT_IN_USE", "Department has employees or child departments")
	default:
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", strings.TrimSpace("Internal server error"))
	}
}
