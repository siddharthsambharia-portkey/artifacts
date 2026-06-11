package auth

import (
	"net/http"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

var DevUser = &User{
	Email:  "dev@localhost",
	Name:   "Dev User",
	Title:  "Engineer",
	Team:   "Platform",
	Slack:  "@devuser",
	Avatar: "",
	Groups: []string{"employees", "admins"},
}

type DevAuthenticator struct{}

func NewDevAuthenticator() *DevAuthenticator {
	return &DevAuthenticator{}
}

func (d *DevAuthenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := DevUser
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (d *DevAuthenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

type Authenticator interface {
	Middleware(next http.Handler) http.Handler
	LoginHandler(w http.ResponseWriter, r *http.Request)
}

// CallbackCapable authenticators handle OIDC callback routes.
type CallbackCapable interface {
	CallbackHandler(w http.ResponseWriter, r *http.Request)
}

func NewAuthenticator(cfg *config.Config, database *db.DB) (Authenticator, error) {
	switch cfg.Auth.Mode {
	case "dev":
		return NewDevAuthenticator(), nil
	case "header-trust":
		return NewHeaderTrustAuthenticator(cfg), nil
	case "oidc":
		return NewOIDCAuthenticator(cfg, database)
	default:
		return NewDevAuthenticator(), nil
	}
}
