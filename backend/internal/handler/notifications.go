package handler

import (
	"errors"
	"net/http"

	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/notification"
)

type notificationHandler struct {
	service *notification.PreferencesService
}

func (h notificationHandler) getPreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}
	result, err := h.service.Get(r.Context(), claims.UserID)
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
	result, err := h.service.Update(r.Context(), claims.UserID, input)
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
	if err := h.service.Unsubscribe(r.Context(), input.Token); err != nil {
		if errors.Is(err, notification.ErrInvalidUnsubscribeToken) {
			httpjson.WriteError(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid unsubscribe token")
			return
		}
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
