package handler

import (
	"encoding/csv"
	"errors"
	"io"
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

type updateProfileRequest struct {
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	MiddleName *string `json:"middle_name"`
	Position   string  `json:"position"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

const maxCSVImportSize = 2 << 20

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

func (h usersHandler) importCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxCSVImportSize); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_FILE", "CSV file is required")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_FILE", "CSV file is required")
		return
	}
	defer file.Close()

	rows, rowErrors, err := parseUsersCSV(io.LimitReader(file, maxCSVImportSize))
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_CSV", err.Error())
		return
	}

	result := h.service.Import(r.Context(), users.ImportInput{Rows: rows})
	result.Failed += len(rowErrors)
	result.Errors = append(rowErrors, result.Errors...)

	httpjson.WriteJSON(w, http.StatusOK, result)
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

func (h usersHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	var req updateProfileRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if err := validateUpdateProfile(req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	user, err := h.service.UpdateProfile(r.Context(), claims.UserID, users.UpdateProfileInput{
		FirstName:  strings.TrimSpace(req.FirstName),
		LastName:   strings.TrimSpace(req.LastName),
		MiddleName: emptyStringToNil(req.MiddleName),
		Position:   strings.TrimSpace(req.Position),
	})
	if err != nil {
		writeUsersError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

func (h usersHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := accessClaimsFromContext(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
		return
	}

	var req changePasswordRequest
	if err := httpjson.DecodeJSON(r, &req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	if strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "current_password and new_password are required")
		return
	}

	if err := h.service.ChangePassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		writeUsersError(w, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func parseUsersCSV(reader io.Reader) ([]users.CreateInput, []users.ImportRowError, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, errors.New("invalid CSV format")
	}
	if len(records) < 2 {
		return nil, nil, errors.New("CSV must include header and at least one user row")
	}
	if !validUsersCSVHeader(records[0]) {
		return nil, nil, errors.New("CSV header must be email,password,first_name,last_name,middle_name,position,roles")
	}

	rows := make([]users.CreateInput, 0, len(records)-1)
	rowErrors := make([]users.ImportRowError, 0)
	for index, record := range records[1:] {
		rowNumber := index + 2
		if len(record) != 7 {
			rowErrors = append(rowErrors, users.ImportRowError{Row: rowNumber, Message: "row must include 7 columns"})
			continue
		}

		input := users.CreateInput{
			Email:      strings.TrimSpace(record[0]),
			Password:   record[1],
			FirstName:  strings.TrimSpace(record[2]),
			LastName:   strings.TrimSpace(record[3]),
			MiddleName: stringToNil(record[4]),
			Position:   strings.TrimSpace(record[5]),
			Roles:      parseCSVRoles(record[6]),
		}

		if err := validateCreateUser(createUserRequest{
			Email:     input.Email,
			Password:  input.Password,
			FirstName: input.FirstName,
			LastName:  input.LastName,
			Position:  input.Position,
		}); err != nil {
			rowErrors = append(rowErrors, users.ImportRowError{Row: rowNumber, Email: input.Email, Message: err.Error()})
			continue
		}

		rows = append(rows, input)
	}

	return rows, rowErrors, nil
}

func validUsersCSVHeader(header []string) bool {
	expected := []string{"email", "password", "first_name", "last_name", "middle_name", "position", "roles"}
	if len(header) != len(expected) {
		return false
	}
	for index, column := range header {
		if strings.TrimSpace(column) != expected[index] {
			return false
		}
	}
	return true
}

func parseCSVRoles(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, "|")
}

func validateUpdateUser(req updateUserRequest) error {
	return validateUpdateProfile(updateProfileRequest{
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: req.MiddleName,
		Position:   req.Position,
	})
}

func validateUpdateProfile(req updateProfileRequest) error {
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
	case errors.Is(err, users.ErrInvalidCurrentPassword):
		httpjson.WriteError(w, http.StatusBadRequest, "INVALID_CURRENT_PASSWORD", "Current password is invalid")
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

func stringToNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
