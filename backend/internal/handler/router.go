package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/catalog"
	"idp-platform/backend/internal/comments"
	"idp-platform/backend/internal/config"
	"idp-platform/backend/internal/dashboard"
	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/idp"
	"idp-platform/backend/internal/notification"
	"idp-platform/backend/internal/tasks"
	"idp-platform/backend/internal/users"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(cfg config.Config, dbPool *pgxpool.Pool, avatarStore AvatarStore, publisher notification.Publisher) http.Handler {
	mux := http.NewServeMux()
	authService := auth.NewService(cfg, dbPool, publisher)
	authHandlers := authHandler{cfg: cfg, service: authService}
	usersHandlers := usersHandler{service: users.NewService(dbPool), avatarStore: avatarStore}
	catalogHandlers := catalogHandler{service: catalog.NewService(dbPool)}
	idpHandlers := idpHandler{service: idp.NewService(dbPool)}
	taskHandlers := tasksHandler{service: tasks.NewService(dbPool)}
	commentHandlers := commentsHandler{service: comments.NewService(dbPool)}
	dashboardHandlers := dashboardHandler{service: dashboard.NewService(dbPool)}

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
	mux.Handle("GET /api/v1/dashboard", authMiddleware(cfg, http.HandlerFunc(dashboardHandlers.get)))
	mux.Handle("PUT /api/v1/users/me", authMiddleware(cfg, http.HandlerFunc(usersHandlers.updateProfile)))
	mux.Handle("PUT /api/v1/users/me/password", authMiddleware(cfg, http.HandlerFunc(usersHandlers.changePassword)))
	mux.Handle("PUT /api/v1/users/me/avatar", authMiddleware(cfg, http.HandlerFunc(usersHandlers.updateAvatar)))
	mux.Handle("GET /api/v1/users", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.list))))
	mux.Handle("POST /api/v1/users", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.create))))
	mux.Handle("POST /api/v1/users/import", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.importCSV))))
	mux.Handle("GET /api/v1/users/subordinates", authMiddleware(cfg, http.HandlerFunc(usersHandlers.subordinates)))
	mux.Handle("GET /api/v1/users/{id}/idps", authMiddleware(cfg, http.HandlerFunc(usersHandlers.idps)))
	mux.Handle("PATCH /api/v1/users/{id}/activate", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.activate))))
	mux.Handle("GET /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.get))))
	mux.Handle("PUT /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.update))))
	mux.Handle("DELETE /api/v1/users/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(usersHandlers.deactivate))))
	mux.Handle("GET /api/v1/competencies", authMiddleware(cfg, http.HandlerFunc(catalogHandlers.listCompetencies)))
	mux.Handle("POST /api/v1/competencies", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.createCompetency))))
	mux.Handle("PUT /api/v1/competencies/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.updateCompetency))))
	mux.Handle("DELETE /api/v1/competencies/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.archiveCompetency))))
	mux.Handle("GET /api/v1/task-categories", authMiddleware(cfg, http.HandlerFunc(catalogHandlers.listTaskCategories)))
	mux.Handle("POST /api/v1/task-categories", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.createTaskCategory))))
	mux.Handle("PUT /api/v1/task-categories/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.updateTaskCategory))))
	mux.Handle("DELETE /api/v1/task-categories/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.deleteTaskCategory))))
	mux.Handle("GET /api/v1/tags", authMiddleware(cfg, http.HandlerFunc(catalogHandlers.listTags)))
	mux.Handle("POST /api/v1/tags", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.createTag))))
	mux.Handle("PUT /api/v1/tags/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.updateTag))))
	mux.Handle("DELETE /api/v1/tags/{id}", authMiddleware(cfg, requireRole("hr_admin", http.HandlerFunc(catalogHandlers.deleteTag))))
	mux.Handle("GET /api/v1/idps", authMiddleware(cfg, http.HandlerFunc(idpHandlers.list)))
	mux.Handle("POST /api/v1/idps", authMiddleware(cfg, http.HandlerFunc(idpHandlers.create)))
	mux.Handle("GET /api/v1/idps/{id}", authMiddleware(cfg, http.HandlerFunc(idpHandlers.get)))
	mux.Handle("PUT /api/v1/idps/{id}", authMiddleware(cfg, http.HandlerFunc(idpHandlers.update)))
	mux.Handle("PATCH /api/v1/idps/{id}/status", authMiddleware(cfg, http.HandlerFunc(idpHandlers.changeStatus)))
	mux.Handle("DELETE /api/v1/idps/{id}", authMiddleware(cfg, http.HandlerFunc(idpHandlers.archive)))
	mux.Handle("GET /api/v1/idps/{id}/audit", authMiddleware(cfg, http.HandlerFunc(idpHandlers.audit)))
	mux.Handle("GET /api/v1/idps/{id}/tasks", authMiddleware(cfg, http.HandlerFunc(taskHandlers.list)))
	mux.Handle("POST /api/v1/idps/{id}/tasks", authMiddleware(cfg, http.HandlerFunc(taskHandlers.create)))
	mux.Handle("GET /api/v1/tasks/{id}", authMiddleware(cfg, http.HandlerFunc(taskHandlers.get)))
	mux.Handle("PUT /api/v1/tasks/{id}", authMiddleware(cfg, http.HandlerFunc(taskHandlers.update)))
	mux.Handle("PATCH /api/v1/tasks/{id}/progress", authMiddleware(cfg, http.HandlerFunc(taskHandlers.updateProgress)))
	mux.Handle("DELETE /api/v1/tasks/{id}", authMiddleware(cfg, http.HandlerFunc(taskHandlers.delete)))
	mux.Handle("GET /api/v1/tasks/{id}/audit", authMiddleware(cfg, http.HandlerFunc(taskHandlers.audit)))
	mux.Handle("GET /api/v1/idps/{id}/comments", authMiddleware(cfg, http.HandlerFunc(commentHandlers.listIDP)))
	mux.Handle("POST /api/v1/idps/{id}/comments", authMiddleware(cfg, http.HandlerFunc(commentHandlers.createIDP)))
	mux.Handle("GET /api/v1/tasks/{id}/comments", authMiddleware(cfg, http.HandlerFunc(commentHandlers.listTask)))
	mux.Handle("POST /api/v1/tasks/{id}/comments", authMiddleware(cfg, http.HandlerFunc(commentHandlers.createTask)))
	mux.Handle("PUT /api/v1/comments/{id}", authMiddleware(cfg, http.HandlerFunc(commentHandlers.update)))
	mux.Handle("DELETE /api/v1/comments/{id}", authMiddleware(cfg, http.HandlerFunc(commentHandlers.delete)))

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
