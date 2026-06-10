package handler

import (
	"context"
	"net/http"
	"strings"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/httpjson"
)

type contextKey string

const claimsContextKey contextKey = "access_claims"

func authMiddleware(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Bearer token is required")
			return
		}

		claims, err := auth.ParseAccessToken(cfg, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := accessClaimsFromContext(r.Context())
		if !ok {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
			return
		}

		for _, currentRole := range claims.Roles {
			if currentRole == role {
				next.ServeHTTP(w, r)
				return
			}
		}

		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	})
}

func accessClaimsFromContext(ctx context.Context) (*auth.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*auth.AccessClaims)
	return claims, ok
}
