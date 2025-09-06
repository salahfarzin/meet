package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

func CreateStack(m ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(m) - 1; i >= 0; i-- {
			next = m[i](next)
		}

		return next
	}
}
