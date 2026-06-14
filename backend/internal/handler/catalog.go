package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"idp-platform/backend/internal/catalog"
	"idp-platform/backend/internal/httpjson"
)

type catalogHandler struct {
	service *catalog.Service
}

type competencyRequest struct {
	Name        string                    `json:"name"`
	Description *string                   `json:"description"`
	Category    string                    `json:"category"`
	IsActive    *bool                     `json:"is_active"`
	Levels      []catalog.CompetencyLevel `json:"levels"`
}

type namedItemRequest struct {
	Name string `json:"name"`
}

func (h catalogHandler) listCompetencies(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	result, err := h.service.ListCompetencies(r.Context(), includeInactive)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h catalogHandler) createCompetency(w http.ResponseWriter, r *http.Request) {
	var req competencyRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if err := validateCompetency(req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	item, err := h.service.CreateCompetency(r.Context(), competencyInput(req))
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusCreated, item)
}

func (h catalogHandler) updateCompetency(w http.ResponseWriter, r *http.Request) {
	var req competencyRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if err := validateCompetency(req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	item, err := h.service.UpdateCompetency(r.Context(), catalogIDFromPath(r, "/api/v1/competencies/"), competencyInput(req))
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, item)
}

func (h catalogHandler) archiveCompetency(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ArchiveCompetency(r.Context(), catalogIDFromPath(r, "/api/v1/competencies/")); err != nil {
		writeCatalogError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h catalogHandler) listTaskCategories(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListTaskCategories(r.Context())
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h catalogHandler) createTaskCategory(w http.ResponseWriter, r *http.Request) {
	h.createNamed(w, r, h.service.CreateTaskCategory)
}

func (h catalogHandler) updateTaskCategory(w http.ResponseWriter, r *http.Request) {
	h.updateNamed(w, r, "/api/v1/task-categories/", h.service.UpdateTaskCategory)
}

func (h catalogHandler) deleteTaskCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteTaskCategory(r.Context(), catalogIDFromPath(r, "/api/v1/task-categories/")); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h catalogHandler) listTags(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListTags(r.Context())
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h catalogHandler) createTag(w http.ResponseWriter, r *http.Request) {
	h.createNamed(w, r, h.service.CreateTag)
}

func (h catalogHandler) updateTag(w http.ResponseWriter, r *http.Request) {
	h.updateNamed(w, r, "/api/v1/tags/", h.service.UpdateTag)
}

func (h catalogHandler) deleteTag(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteTag(r.Context(), catalogIDFromPath(r, "/api/v1/tags/")); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h catalogHandler) createNamed(w http.ResponseWriter, r *http.Request, create func(ctx context.Context, name string) (*catalog.NamedItem, error)) {
	var req namedItemRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	item, err := create(r.Context(), req.Name)
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusCreated, item)
}

func (h catalogHandler) updateNamed(w http.ResponseWriter, r *http.Request, prefix string, update func(ctx context.Context, id, name string) (*catalog.NamedItem, error)) {
	var req namedItemRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	item, err := update(r.Context(), catalogIDFromPath(r, prefix), req.Name)
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, item)
}

func validateCompetency(req competencyRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(req.Category) == "" {
		return errors.New("category is required")
	}
	return nil
}

func competencyInput(req competencyRequest) catalog.CompetencyInput {
	return catalog.CompetencyInput{
		Name:        strings.TrimSpace(req.Name),
		Description: emptyStringToNil(req.Description),
		Category:    strings.TrimSpace(req.Category),
		IsActive:    boolValue(req.IsActive, true),
		Levels:      req.Levels,
	}
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func catalogIDFromPath(r *http.Request, prefix string) string {
	return strings.TrimPrefix(r.URL.Path, prefix)
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Catalog item not found")
	case errors.Is(err, catalog.ErrNameExists):
		httpjson.WriteError(w, http.StatusConflict, "NAME_EXISTS", "Name already exists")
	case errors.Is(err, catalog.ErrInUse):
		httpjson.WriteError(w, http.StatusConflict, "CATALOG_ITEM_IN_USE", "Catalog item is already in use")
	case errors.Is(err, catalog.ErrInvalidInput):
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid catalog input")
	default:
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
