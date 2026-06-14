package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/users"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(cfg config.Config, dbPool *pgxpool.Pool, avatarStore AvatarStore) http.Handler {
	mux := http.NewServeMux()
	authService := auth.NewService(cfg, dbPool)
	authHandlers := authHandler{cfg: cfg, service: authService}
	usersHandlers := usersHandler{service: users.NewService(dbPool), avatarStore: avatarStore}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", readinessHandler(cfg, dbPool))
	mux.HandleFunc("GET /api/v1/health", healthHandler)
	mux.HandleFunc("GET /api/v1/ready", readinessHandler(cfg, dbPool))
	mux.HandleFunc("POST /api/v1/auth/login", authHandlers.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandlers.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandlers.logout)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", authHandlers.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", authHandlers.resetPassword)
	mux.Handle("GET /api/v1/users/me", authMiddleware(cfg, http.HandlerFunc(authHandlers.me)))
	mux.Handle("PUT /api/v1/users/me", authMiddleware(cfg, http.HandlerFunc(usersHandlers.updateProfile)))
	mux.Handle("PUT /api/v1/users/me/password", authMiddleware(cfg, http.HandlerFunc(usersHandlers.changePassword)))
	mux.Handle("PUT /api/v1/users/me/avatar", authMiddleware(cfg, http.HandlerFunc(usersHandlers.updateAvatar)))
	mux.Handle("GET /api/v1/users", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.list))))
	mux.Handle("POST /api/v1/users", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.create))))
	mux.Handle("POST /api/v1/users/import", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.importCSV))))
	mux.Handle("GET /api/v1/users/subordinates", authMiddleware(cfg, http.HandlerFunc(usersHandlers.subordinates)))
	mux.Handle("GET /api/v1/users/{id}/idps", authMiddleware(cfg, http.HandlerFunc(usersHandlers.idps)))
	mux.Handle("GET /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.get))))
	mux.Handle("PUT /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.update))))
	mux.Handle("DELETE /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.deactivate))))

	notFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	})

	return cors(cfg.CORSOrigins)(recoverer(timeout(60*time.Second, logger(routeOrNotFound(mux, notFound)))))
}

func (h authHandler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not found")
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func readinessHandler(cfg config.Config, dbPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := dbPool.Ping(ctx); err != nil {
			httpjson.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"env":    cfg.AppEnv,
				"checks": map[string]string{
					"database": "unavailable",
				},
			})
			return
		}

		httpjson.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "ready",
			"env":    cfg.AppEnv,
			"checks": map[string]string{
				"database": "ok",
			},
		})
	}
}

func cors(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func routeOrNotFound(mux *http.ServeMux, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, pattern := mux.Handler(r)
		if pattern == "" {
			notFound.ServeHTTP(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

func timeout(duration time.Duration, next http.Handler) http.Handler {
	return http.TimeoutHandler(next, duration, `{"error":{"code":"TIMEOUT","message":"Request timed out"}}`)
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered", "error", recovered)
				httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
