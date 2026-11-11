package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "PhilSenior@2026"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == password {
		t.Fatal("password hash must not equal plaintext password")
	}

	if !strings.HasPrefix(hash, "$2a$12$") {
		t.Errorf("unexpected bcrypt hash format: %s", hash[:7])
	}

	if !VerifyPassword(password, hash) {
		t.Fatal("VerifyPassword() rejected the correct password")
	}
}

func TestHashPasswordProducesDifferentHashes(t *testing.T) {
	password := "PhilSenior@2026"

	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("first HashPassword() error: %v", err)
	}

	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword() error: %v", err)
	}

	if first == second {
		t.Fatal("bcrypt should generate different salted hashes")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("CorrectPassword@2026")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if VerifyPassword("WrongPassword@2026", hash) {
		t.Fatal("VerifyPassword() accepted an incorrect password")
	}
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	_, err := HashPassword("")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("expected ErrEmptyPassword, got %v", err)
	}
}

func TestHashPasswordRejectsLongPassword(t *testing.T) {
	password := strings.Repeat("a", 73)

	_, err := HashPassword(password)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestVerifyPasswordRejectsEmptyInputs(t *testing.T) {
	if VerifyPassword("", "some-hash") {
		t.Fatal("accepted empty password")
	}

	if VerifyPassword("some-password", "") {
		t.Fatal("accepted empty password hash")
	}
}
