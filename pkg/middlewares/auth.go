package middlewares

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"
)

type User struct {
	ID        string
	Uuid      string
	Email     string
	Roles     []string
	Mobile    *string `json:"mobile"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Gender    *string `json:"gender"`
	Birthdate *string `json:"birthdate"`
}

// Context key for user info
var userKey = &struct{}{}

// GetUser retrieves user info from context
func GetUser(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey).(*User)
	return user, ok
}

// AuthServiceFunc checks token and returns user info (mock signature)
type AuthServiceFunc func(token string) (*User, error)

// AuthMiddleware validates access_token and injects user info into context
func AuthMiddleware(authService AuthServiceFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				http.Error(w, "missing access token", http.StatusUnauthorized)
				return
			}

			user, err := authService(token)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				http.Error(w, `{"Code": 401, "message": "invalid access token"}`, http.StatusUnauthorized)
				return
			}

			// Set user in context for HTTP handlers
			ctx := context.WithValue(r.Context(), userKey, user)

			// Set headers for gRPC-Gateway to forward as metadata
			r.Header.Set("x-user-id", user.ID)
			r.Header.Set("x-user-uuid", user.Uuid)
			r.Header.Set("x-user-roles", strings.Join(user.Roles, ","))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	if cookie, err := r.Cookie("access_token"); err == nil {
		return cookie.Value
	}
	return ""
}

// GetUserFromContext tries to extract user info and roles from context or gRPC metadata
func GetUserFromContext(ctx context.Context) User {
	// Try context first (HTTP)
	if user, ok := GetUser(ctx); ok && user != nil {
		return *user
	}
	// Try gRPC metadata (gRPC-Gateway)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// Try x-user-id, x-user-uuid, x-user-roles (string fields)
		id := ""
		uuid := ""
		email := ""
		roles := []string{}
		if ids := md.Get("x-user-id"); len(ids) > 0 {
			id = ids[0]
		}
		if uuids := md.Get("x-user-uuid"); len(uuids) > 0 {
			uuid = uuids[0]
		}
		if emails := md.Get("x-user-email"); len(emails) > 0 {
			email = emails[0]
		}
		if r := md.Get("x-user-roles"); len(r) > 0 {
			roles = strings.Split(r[0], ",")
		}
		// If at least id or uuid is present, return a User
		if id != "" || uuid != "" {
			return User{ID: id, Uuid: uuid, Email: email, Roles: roles}
		}
	}

	return User{}
}
