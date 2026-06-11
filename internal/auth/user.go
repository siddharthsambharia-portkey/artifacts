package auth

import (
	"context"
	"net/http"
)

type User struct {
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Title   string   `json:"title,omitempty"`
	Team    string   `json:"team,omitempty"`
	Slack   string   `json:"slack,omitempty"`
	Avatar  string   `json:"avatar,omitempty"`
	Groups  []string `json:"groups,omitempty"`
	IsAdmin bool     `json:"-"`
}

type contextKey string

const userKey contextKey = "artifact_user"

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil {
			http.Error(w, `{"error":"You must be signed in. Run artifact login or visit /login."}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
