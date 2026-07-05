package handler

import (
	"errors"
	"net/http"
	"time"

	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/notification"
)

type notificationHandler struct {
	preferences *notification.PreferencesService
	inApp       *notification.InAppService
}

func (h notificationHandler) getPreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.preferences.Get(r.Context(), claims.UserID)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h notificationHandler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	var input notification.Preferences
	if err := httpjson.DecodeJSON(r, &input); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	result, err := h.preferences.Update(r.Context(), claims.UserID, input)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h notificationHandler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := httpjson.DecodeJSON(r, &input); err != nil || input.Token == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid unsubscribe token")
		return
	}
	if err := h.preferences.Unsubscribe(r.Context(), input.Token); err != nil {
		if errors.Is(err, notification.ErrInvalidUnsubscribeToken) {
			httpjson.WriteError(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid unsubscribe token")
			return
		}
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h notificationHandler) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	query := r.URL.Query()
	dateFrom, err := parseOptionalDate(query.Get("date_from"))
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_DATE", "Invalid date_from")
		return
	}
	dateTo, err := parseOptionalDate(query.Get("date_to"))
	if err != nil || dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_DATE", "Invalid date range")
		return
	}
	result, err := h.inApp.List(r.Context(), claims.UserID, query.Get("kind"), query.Get("unread") == "true", query.Get("sort") == "oldest", dateFrom, dateTo)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, result)
}

func parseOptionalDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	return &parsed, err
}

func (h notificationHandler) markRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	if err := h.inApp.MarkRead(r.Context(), claims.UserID, r.PathValue("id")); err != nil {
		if errors.Is(err, notification.ErrNotificationNotFound) {
			httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Notification not found")
			return
		}
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h notificationHandler) markAllRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	if err := h.inApp.MarkAllRead(r.Context(), claims.UserID); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
