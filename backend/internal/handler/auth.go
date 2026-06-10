package handler

import (
	"errors"
	"net/http"
	"strings"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/httpjson"
)

type authHandler struct {
	cfg     config.Config
	service *auth.Service
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken          string    `json:"access_token"`
	AccessTokenExpiresAt string    `json:"access_token_expires_at"`
	User                 auth.User `json:"user"`
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email and password are required")
		return
	}

	tokens, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, tokens)
	writeTokenResponse(w, tokens)
}

func (h authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cfg.RefreshCookieName)
	if err != nil || cookie.Value == "" {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Refresh token is required")
		return
	}

	tokens, err := h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	h.setRefreshCookie(w, tokens)
	writeTokenResponse(w, tokens)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cfg.RefreshCookieName)
	if err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.RefreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: http.SameSiteStrictMode,
	})

	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h authHandler) setRefreshCookie(w http.ResponseWriter, tokens *auth.TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.RefreshCookieName,
		Value:    tokens.RefreshToken,
		Path:     "/",
		Expires:  tokens.RefreshTokenExpiresAt,
		HttpOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: http.SameSiteStrictMode,
	})
}

func writeTokenResponse(w http.ResponseWriter, tokens *auth.TokenPair) {
	httpjson.WriteJSON(w, http.StatusOK, tokenResponse{
		AccessToken:          tokens.AccessToken,
		AccessTokenExpiresAt: tokens.AccessTokenExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User:                 tokens.User,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpjson.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
	case errors.Is(err, auth.ErrUserBlocked):
		httpjson.WriteError(w, http.StatusForbidden, "USER_BLOCKED", "User account is blocked")
	case errors.Is(err, auth.ErrUserLocked):
		httpjson.WriteError(w, http.StatusTooManyRequests, "USER_LOCKED", "User account is temporarily locked")
	case errors.Is(err, auth.ErrInvalidToken):
		httpjson.WriteError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Invalid refresh token")
	default:
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
