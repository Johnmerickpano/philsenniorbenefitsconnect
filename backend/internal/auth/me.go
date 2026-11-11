package auth

import (
	"net/http"
)

func NewMeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := AuthenticatedUserFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "authentication required",
			})
			return
		}

		writeJSON(w, http.StatusOK, struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}{
			UserID: user.UserID,
			Role:   string(user.Role),
		})
	})
}
