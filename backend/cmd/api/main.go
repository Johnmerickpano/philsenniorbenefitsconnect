package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/auth"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	session, err := database.NewScyllaSession(cfg)
	if err != nil {
		log.Fatalf("initialize ScyllaDB: %v", err)
	}
	defer session.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), cfg.ScyllaConnectTimeout)
	defer cancelPing()

	if err := database.Ping(pingCtx, session); err != nil {
		log.Fatalf("ScyllaDB is not ready: %v", err)
	}
	log.Println("ScyllaDB connection established")

	authRepository, err := auth.NewRepository(session)
	if err != nil {
		log.Fatalf("initialize authentication repository: %v", err)
	}

	authService, err := auth.NewService(authRepository, cfg)
	if err != nil {
		log.Fatalf("initialize authentication service: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := database.Ping(ctx, session); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("READY"))
	})

	mux.Handle("POST /api/v1/auth/login", auth.NewLoginHandler(authService))
	mux.Handle("POST /api/v1/auth/refresh", auth.NewRefreshHandler(authService))
	mux.Handle("GET /api/v1/auth/me", authService.Authenticate(auth.NewMeHandler()))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("API server listening on %s", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server: %v", err)
		}
	case <-signalCtx.Done():
		log.Println("Shutting down API server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}
}
