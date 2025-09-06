package middlewares

import (
	"context"
	"net/http"
)

type User struct {
	ID        string
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
	info, ok := ctx.Value(userKey).(*User)
	return info, ok
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
				http.Error(w, `{"Code": 401, "message": "invalid access token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, user)
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
