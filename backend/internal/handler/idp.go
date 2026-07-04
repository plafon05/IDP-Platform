package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/idp"
)

type idpHandler struct {
	service *idp.Service
}

type idpRequest struct {
	EmployeeID   string                 `json:"employee_id"`
	Title        string                 `json:"title"`
	Goals        *string                `json:"goals"`
	StartDate    string                 `json:"start_date"`
	EndDate      string                 `json:"end_date"`
	Competencies []idp.CompetencyTarget `json:"competencies"`
}

type idpStatusRequest struct {
	Status  string  `json:"status"`
	Comment *string `json:"comment"`
	Reason  *string `json:"reason"`
}

func (h idpHandler) list(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	result, err := h.service.List(r.Context(), access, idp.ListParams{
		EmployeeID: strings.TrimSpace(r.URL.Query().Get("employeeId")),
		ManagerID:  strings.TrimSpace(r.URL.Query().Get("managerId")),
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
		Page:       positiveInt(r.URL.Query().Get("page"), 1),
		Limit:      positiveInt(r.URL.Query().Get("limit"), 50),
	})
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func positiveInt(value string, fallback int) int {
	result, err := strconv.Atoi(value)
	if err != nil || result < 1 {
		return fallback
	}
	return result
}

func (h idpHandler) create(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	input, ok := decodeIDPInput(w, r)
	if !ok {
		return
	}
	result, err := h.service.Create(r.Context(), access, input)
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, result)
}

func (h idpHandler) get(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	result, err := h.service.Get(r.Context(), access, idpIDFromPath(r))
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h idpHandler) export(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	doc, err := h.service.Export(r.Context(), access, r.PathValue("id"))
	if err != nil {
		writeIDPError(w, err)
		return
	}
	format := r.PathValue("format")
	var data []byte
	var contentType, extension string
	switch format {
	case "xlsx":
		data, err = doc.XLSX()
		contentType, extension = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
	case "pdf":
		data, err = doc.PDF()
		contentType, extension = "application/pdf", "pdf"
	default:
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_FORMAT", "Export format must be xlsx or pdf")
		return
	}
	if err != nil {
		writeIDPError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="idp.`+extension+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h idpHandler) update(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	input, valid := decodeIDPInput(w, r)
	if !valid {
		return
	}
	result, err := h.service.Update(r.Context(), access, idpIDFromPath(r), input)
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h idpHandler) changeStatus(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	var req idpStatusRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.service.ChangeStatus(r.Context(), access, idpIDFromStatusPath(r), idp.StatusInput{
		Status:  strings.TrimSpace(req.Status),
		Comment: emptyStringToNil(req.Comment),
		Reason:  emptyStringToNil(req.Reason),
	})
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h idpHandler) archive(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	if err := h.service.Archive(r.Context(), access, idpIDFromPath(r)); err != nil {
		writeIDPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h idpHandler) audit(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.Audit(r.Context(), access, idpIDFromAuditPath(r))
	if err != nil {
		writeIDPError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func decodeIDPInput(w http.ResponseWriter, r *http.Request) (idp.Input, bool) {
	var req idpRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return idp.Input{}, false
	}

	startDate, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "start_date must use YYYY-MM-DD format")
		return idp.Input{}, false
	}
	endDate, err := time.Parse(time.DateOnly, req.EndDate)
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_date must use YYYY-MM-DD format")
		return idp.Input{}, false
	}

	return idp.Input{
		EmployeeID:   strings.TrimSpace(req.EmployeeID),
		Title:        strings.TrimSpace(req.Title),
		Goals:        emptyStringToNil(req.Goals),
		StartDate:    startDate,
		EndDate:      endDate,
		Competencies: req.Competencies,
	}, true
}

func idpAccess(r *http.Request) (idp.Access, bool) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		return idp.Access{}, false
	}
	return idp.Access{
		UserID:  claims.UserID,
		IsHR:    hasRole(claims.Roles, "hr_admin"),
		Manager: hasRole(claims.Roles, "manager"),
	}, true
}

func idpIDFromPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/v1/idps/")
}

func idpIDFromStatusPath(r *http.Request) string {
	return strings.TrimSuffix(idpIDFromPath(r), "/status")
}

func idpIDFromAuditPath(r *http.Request) string {
	return strings.TrimSuffix(idpIDFromPath(r), "/audit")
}

func writeIDPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, idp.ErrNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "IDP_NOT_FOUND", "IDP not found")
	case errors.Is(err, idp.ErrForbidden):
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	case errors.Is(err, idp.ErrInvalidTransition):
		httpjson.WriteError(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Invalid IDP status transition")
	case errors.Is(err, idp.ErrIncompleteTasks):
		httpjson.WriteError(w, http.StatusConflict, "IDP_HAS_INCOMPLETE_TASKS", "Complete all active tasks before completing the IDP")
	case errors.Is(err, idp.ErrEmployeeNoManager):
		httpjson.WriteError(w, http.StatusConflict, "EMPLOYEE_HAS_NO_MANAGER", "Employee must have a manager")
	case errors.Is(err, idp.ErrEmployeeInactive):
		httpjson.WriteError(w, http.StatusConflict, "EMPLOYEE_INACTIVE", "Employee is inactive")
	case errors.Is(err, idp.ErrInvalidInput):
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid IDP data")
	default:
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
