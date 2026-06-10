package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"idp-platform/backend/internal/auth"
	"idp-platform/backend/internal/httpjson"
	"idp-platform/backend/internal/users"
)

type usersHandler struct {
	service *users.Service
}

type createUserRequest struct {
	Email      string   `json:"email"`
	Password   string   `json:"password"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	MiddleName *string  `json:"middle_name"`
	Position   string   `json:"position"`
	Roles      []string `json:"roles"`
}

type updateUserRequest struct {
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	MiddleName *string  `json:"middle_name"`
	Position   string   `json:"position"`
	IsActive   bool     `json:"is_active"`
	Roles      []string `json:"roles"`
}

func (h usersHandler) list(w http.ResponseWriter, r *http.Request) {
	page := intQuery(r, "page", 1)
	limit := intQuery(r, "limit", 50)
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	result, err := h.service.List(r.Context(), users.ListParams{
		Page:  page,
		Limit: limit,
		Query: query,
	})
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, result)
}

func (h usersHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if err := validateCreateUser(req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	user, err := h.service.Create(r.Context(), users.CreateInput{
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  strings.TrimSpace(req.FirstName),
		LastName:   strings.TrimSpace(req.LastName),
		MiddleName: emptyStringToNil(req.MiddleName),
		Position:   strings.TrimSpace(req.Position),
		Roles:      req.Roles,
	})
	if err != nil {
		writeUsersError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusCreated, user)
}

func (h usersHandler) get(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Get(r.Context(), userIDFromPath(r))
	if err != nil {
		writeUsersError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

func (h usersHandler) update(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if err := validateUpdateUser(req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	user, err := h.service.Update(r.Context(), userIDFromPath(r), users.UpdateInput{
		FirstName:  strings.TrimSpace(req.FirstName),
		LastName:   strings.TrimSpace(req.LastName),
		MiddleName: emptyStringToNil(req.MiddleName),
		Position:   strings.TrimSpace(req.Position),
		IsActive:   req.IsActive,
		Roles:      req.Roles,
	})
	if err != nil {
		writeUsersError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

func (h usersHandler) deactivate(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Deactivate(r.Context(), userIDFromPath(r)); err != nil {
		writeUsersError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func userIDFromPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
}

func validateCreateUser(req createUserRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if strings.TrimSpace(req.FirstName) == "" {
		return errors.New("first_name is required")
	}
	if strings.TrimSpace(req.LastName) == "" {
		return errors.New("last_name is required")
	}
	if strings.TrimSpace(req.Position) == "" {
		return errors.New("position is required")
	}
	return nil
}

func validateUpdateUser(req updateUserRequest) error {
	if strings.TrimSpace(req.FirstName) == "" {
		return errors.New("first_name is required")
	}
	if strings.TrimSpace(req.LastName) == "" {
		return errors.New("last_name is required")
	}
	if strings.TrimSpace(req.Position) == "" {
		return errors.New("position is required")
	}
	return nil
}

func writeUsersError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		httpjson.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, users.ErrEmailExists):
		httpjson.WriteError(w, http.StatusConflict, "EMAIL_EXISTS", "Email already exists")
	case errors.Is(err, auth.ErrWeakPassword):
		httpjson.WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", auth.ErrWeakPassword.Error())
	default:
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func emptyStringToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
