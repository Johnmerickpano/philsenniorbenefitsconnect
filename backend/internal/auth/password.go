package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const PasswordHashCost = 12

var (
	ErrEmptyPassword   = errors.New("password cannot be empty")
	ErrPasswordTooLong = errors.New("password exceeds bcrypt's 72-byte limit")
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	if len([]byte(password)) > 72 {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		PasswordHashCost,
	)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func VerifyPassword(password, passwordHash string) bool {
	if password == "" || passwordHash == "" {
		return false
	}

	if len([]byte(password)) > 72 {
		return false
	}

	return bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	) == nil
}
