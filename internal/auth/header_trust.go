package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type HeaderTrustAuthenticator struct {
	cfg    *config.Config
	secret string
}

func NewHeaderTrustAuthenticator(cfg *config.Config) *HeaderTrustAuthenticator {
	secret := os.Getenv(cfg.Auth.HeaderTrust.ProxySecretEnv)
	return &HeaderTrustAuthenticator{cfg: cfg, secret: secret}
}

func (h *HeaderTrustAuthenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyAuth := r.Header.Get("X-Artifact-Proxy-Auth")
		if subtle.ConstantTimeCompare([]byte(proxyAuth), []byte(h.secret)) != 1 {
			http.Error(w, `{"error":"Missing or invalid proxy authentication. Configure your identity proxy to send X-Artifact-Proxy-Auth."}`, http.StatusUnauthorized)
			return
		}
		email := r.Header.Get(h.cfg.Auth.HeaderTrust.EmailHeader)
		if email == "" {
			http.Error(w, `{"error":"Identity proxy did not send user email header. Check header_trust.email_header in artifact.yaml."}`, http.StatusUnauthorized)
			return
		}
		name := r.Header.Get(h.cfg.Auth.HeaderTrust.NameHeader)
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		u := &User{Email: email, Name: name, Groups: []string{"employees"}}
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *HeaderTrustAuthenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Login is handled by your identity proxy. Access Artifact through your corporate SSO."}`, http.StatusBadRequest)
}
