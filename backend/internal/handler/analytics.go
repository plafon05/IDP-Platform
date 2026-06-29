package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"idp-platform/backend/internal/analytics"
	"idp-platform/backend/internal/httpjson"
)

type analyticsHandler struct{ service *analytics.Service }

func (h analyticsHandler) overview(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	to := time.Now()
	from := to.AddDate(-1, 0, 0)
	var err error
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		from, err = time.Parse(time.DateOnly, value)
		if err != nil {
			httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid analytics period")
			return
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		to, err = time.Parse(time.DateOnly, value)
		if err != nil {
			httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid analytics period")
			return
		}
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if from.After(to) || (status != "" && status != "draft" && status != "active" && status != "completed" && status != "cancelled") {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid analytics filters")
		return
	}
	result, err := h.service.Overview(r.Context(), access, analytics.Filters{From: from, To: to, Status: status})
	if errors.Is(err, analytics.ErrForbidden) {
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}
	if err != nil {
		slog.Error("analytics request failed", "error", err)
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}
