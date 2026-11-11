package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type publicUser struct {
	UserID             string  `json:"user_id"`
	Username           string  `json:"username"`
	UserRole           string  `json:"user_role"`
	FullName           string  `json:"full_name"`
	Email              *string `json:"email,omitempty"`
	IsActive           bool    `json:"is_active"`
	BeneficiaryID      *string `json:"beneficiary_id,omitempty"`
	MustChangePassword bool    `json:"must_change_password"`
}

type loginResponse struct {
	User             publicUser `json:"user"`
	AccessToken      string     `json:"access_token"`
	RefreshToken     string     `json:"refresh_token"`
	TokenType        string     `json:"token_type"`
	AccessExpiresAt  time.Time  `json:"access_expires_at"`
	RefreshExpiresAt time.Time  `json:"refresh_expires_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewLoginHandler(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var request loginRequest
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "invalid JSON request",
			})
			return
		}

		var extra interface{}
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "request must contain one JSON object",
			})
			return
		}

		request.Username = strings.TrimSpace(request.Username)
		if request.Username == "" || request.Password == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "username and password are required",
			})
			return
		}

		if service == nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Error: "authentication service unavailable",
			})
			return
		}

		result, err := service.Login(r.Context(), request.Username, request.Password)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidCredentials):
				writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error: "invalid username or password",
				})
			case errors.Is(err, ErrAccountLocked):
				writeJSON(w, http.StatusLocked, errorResponse{
					Error: "account is temporarily locked",
				})
			case errors.Is(err, ErrAccountInactive):
				writeJSON(w, http.StatusForbidden, errorResponse{
					Error: "account is inactive",
				})
			default:
				log.Printf("authentication login failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{
					Error: "login could not be completed",
				})
			}
			return
		}

		var beneficiaryID *string
		if result.User.BeneficiaryID != nil {
			value := result.User.BeneficiaryID.String()
			beneficiaryID = &value
		}

		response := loginResponse{
			User: publicUser{
				UserID:             result.User.UserID.String(),
				Username:           result.User.Username,
				UserRole:           string(result.User.UserRole),
				FullName:           result.User.FullName,
				Email:              result.User.Email,
				IsActive:           result.User.IsActive,
				BeneficiaryID:      beneficiaryID,
				MustChangePassword: result.User.MustChangePassword,
			},
			AccessToken:      result.AccessToken,
			RefreshToken:     result.RefreshToken,
			TokenType:        result.TokenType,
			AccessExpiresAt:  result.AccessExpiresAt,
			RefreshExpiresAt: result.RefreshExpiresAt,
		}

		writeJSON(w, http.StatusOK, response)
	})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

// refreshRequest contains the refresh token supplied by the client.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// NewRefreshHandler validates and rotates a refresh token.
func NewRefreshHandler(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{
				Error: "method not allowed",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var request refreshRequest
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "invalid JSON request",
			})
			return
		}

		var extra interface{}
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "request must contain one JSON object",
			})
			return
		}

		request.RefreshToken = strings.TrimSpace(request.RefreshToken)
		if request.RefreshToken == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "refresh_token is required",
			})
			return
		}

		if service == nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{
				Error: "authentication service unavailable",
			})
			return
		}

		result, err := service.Refresh(r.Context(), request.RefreshToken)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidToken):
				writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error: "invalid or expired refresh token",
				})
			case errors.Is(err, ErrAccountInactive):
				writeJSON(w, http.StatusForbidden, errorResponse{
					Error: "account is inactive",
				})
			default:
				log.Printf("authentication refresh failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{
					Error: "token refresh could not be completed",
				})
			}
			return
		}

		var beneficiaryID *string
		if result.User.BeneficiaryID != nil {
			value := result.User.BeneficiaryID.String()
			beneficiaryID = &value
		}

		response := loginResponse{
			User: publicUser{
				UserID:             result.User.UserID.String(),
				Username:           result.User.Username,
				UserRole:           string(result.User.UserRole),
				FullName:           result.User.FullName,
				Email:              result.User.Email,
				IsActive:           result.User.IsActive,
				BeneficiaryID:      beneficiaryID,
				MustChangePassword: result.User.MustChangePassword,
			},
			AccessToken:      result.AccessToken,
			RefreshToken:     result.RefreshToken,
			TokenType:        result.TokenType,
			AccessExpiresAt:  result.AccessExpiresAt,
			RefreshExpiresAt: result.RefreshExpiresAt,
		}

		writeJSON(w, http.StatusOK, response)
	})
}
