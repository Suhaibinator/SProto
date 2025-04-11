package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/Suhaibinator/SProto/internal/api/response" // We'll create this package next
)

// AuthMiddleware creates a middleware function that checks for a static bearer token.
func AuthMiddleware(requiredToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if token is provided and valid
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Println("AuthMiddleware: Missing Authorization header")
				response.Error(w, http.StatusUnauthorized, "Unauthorized: Missing Authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				log.Println("AuthMiddleware: Invalid Authorization header format")
				response.Error(w, http.StatusUnauthorized, "Unauthorized: Invalid Authorization header format")
				return
			}

			token := parts[1]
			if token != requiredToken {
				log.Println("AuthMiddleware: Invalid token")
				response.Error(w, http.StatusUnauthorized, "Unauthorized: Invalid token")
				return
			}

			// Token is valid, proceed to the next handler
			// Optionally, add user info to context if using more complex auth
			ctx := context.WithValue(r.Context(), "isAuthenticated", true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ApplyAuth selectively applies the authentication middleware only if the token is not empty.
// If the token is empty, it allows all requests through for that handler.
func ApplyAuth(handler http.Handler, requiredToken string) http.Handler {
	if requiredToken == "" {
		log.Println("Warning: Auth token is empty, authentication is disabled for protected routes.")
		return handler // No auth required if token is not set
	}
	authMiddleware := AuthMiddleware(requiredToken)
	return authMiddleware(handler)
}
