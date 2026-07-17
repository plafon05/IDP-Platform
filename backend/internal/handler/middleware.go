package handler

import (
	"context"
	"net/http"
	"strings"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/httpjson"

	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const claimsContextKey contextKey = "access_claims"

func authMiddleware(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := accessClaimsFromContext(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}
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

// liveAccessMiddleware refreshes account state for every bearer-token request.
// This makes role removals and deactivations effective immediately instead of
// waiting for the access token TTL.
func liveAccessMiddleware(cfg config.Config, db *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := auth.ParseAccessToken(cfg, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
			return
		}
		var active bool
		if err := db.QueryRow(r.Context(), `SELECT is_active FROM users WHERE id=$1`, claims.UserID).Scan(&active); err != nil || !active {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User is inactive or unavailable")
			return
		}
		rows, err := db.Query(r.Context(), `SELECT role FROM user_roles WHERE user_id=$1 ORDER BY role`, claims.UserID)
		if err != nil {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User roles are unavailable")
			return
		}
		roles := make([]string, 0, 3)
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				rows.Close()
				httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User roles are unavailable")
				return
			}
			roles = append(roles, role)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User roles are unavailable")
			return
		}
		claims.Roles = roles
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
