package handler

import (
	"log/slog"
	"net/http"

	"idp-platform/backend/internal/dashboard"
	"idp-platform/backend/internal/httpjson"
)

type dashboardHandler struct{ service *dashboard.Service }

func (h dashboardHandler) get(w http.ResponseWriter, r *http.Request) {
	access, ok := idpAccess(r)
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.Get(r.Context(), access)
	if err != nil {
		slog.Error("dashboard request failed", "error", err)
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}
