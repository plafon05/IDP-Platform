package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"idp-platform/backend/internal/auth"
)

type recordingAvatarStore struct{ calls int }

func (s *recordingAvatarStore) PutAvatar(context.Context, string, string, string, io.Reader, int64) (string, error) {
	s.calls++
	return "", nil
}

func TestUpdateAvatarRejectsInvalidFilesBeforeStorage(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		payload  []byte
	}{
		{name: "missing file"},
		{name: "text file", filename: "avatar.txt", payload: []byte("not an image")},
		{name: "file over limit", filename: "avatar.png", payload: append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("a"), maxAvatarSize)...)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingAvatarStore{}
			handler := usersHandler{avatarStore: store}
			req := avatarRequest(t, test.filename, test.payload)
			req = req.WithContext(context.WithValue(req.Context(), claimsContextKey, &auth.AccessClaims{UserID: "user-1"}))
			response := httptest.NewRecorder()

			handler.updateAvatar(response, req)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want 400: %s", response.Code, response.Body.String())
			}
			if store.calls != 0 {
				t.Fatal("storage was called for an invalid avatar")
			}
		})
	}
}

func TestUpdateAvatarRequiresAuthentication(t *testing.T) {
	store := &recordingAvatarStore{}
	response := httptest.NewRecorder()
	usersHandler{avatarStore: store}.updateAvatar(response, avatarRequest(t, "avatar.png", []byte("\x89PNG\r\n\x1a\n")))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", response.Code)
	}
	if store.calls != 0 {
		t.Fatal("storage was called without authentication")
	}
}

func avatarRequest(t *testing.T, filename string, payload []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if filename != "" {
		part, err := writer.CreateFormFile("avatar", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
