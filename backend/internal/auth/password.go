package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var dummyPasswordHash = func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing-protection"), bcryptCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}()

var ErrWeakPassword = errors.New("password must be at least 8 characters and include one uppercase letter and one digit")

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func ComparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CompareDummyPassword keeps the unknown-user login path comparable in cost.
func CompareDummyPassword(password string) { _ = ComparePassword(dummyPasswordHash, password) }

func ValidatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return ErrWeakPassword
	}

	hasUpper := false
	hasDigit := false
	for _, r := range password {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}

	if !hasUpper || !hasDigit {
		return ErrWeakPassword
	}

	return nil
}
