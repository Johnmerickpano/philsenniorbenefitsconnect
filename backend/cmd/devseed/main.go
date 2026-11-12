package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/auth"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
)

func main() {
	username := strings.TrimSpace(os.Getenv("PSBC_SEED_USERNAME"))
	password := os.Getenv("PSBC_SEED_PASSWORD")
	role := schema.UserRole(strings.TrimSpace(os.Getenv("PSBC_SEED_ROLE")))
	if role == "" {
		role = schema.RoleOSCAStaff
	}
	switch role {
	case schema.RoleOSCAStaff, schema.RoleSeniorCitizen:
	default:
		log.Fatal("PSBC_SEED_ROLE must be OSCA_STAFF or SENIOR_CITIZEN")
	}

	if username == "" || password == "" {
		log.Fatal("PSBC_SEED_USERNAME and PSBC_SEED_PASSWORD are required")
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	session, err := database.NewScyllaSession(cfg)
	if err != nil {
		log.Fatalf("connect to ScyllaDB: %v", err)
	}
	defer session.Close()

	repository, err := auth.NewRepository(session)
	if err != nil {
		log.Fatalf("initialize authentication repository: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	user := &schema.User{
		UserID:             gocql.TimeUUID(),
		Username:           username,
		PasswordHash:       passwordHash,
		UserRole:           role,
		FullName:           "Development Test Account",
		IsActive:           true,
		FailedLoginCount:   0,
		MustChangePassword: false,
		CreatedAt:          time.Now().UTC(),
	}

	if err := repository.CreateUser(ctx, user); err != nil {
		if err == auth.ErrUsernameTaken {
			log.Fatalf("username %q already exists; no account was changed", username)
		}
		log.Fatalf("create development account: %v", err)
	}

	fmt.Printf("Development account created successfully. Username: %s\n", username)
	fmt.Printf("Role: %s\n", role)
	fmt.Println("Active: true")
}
