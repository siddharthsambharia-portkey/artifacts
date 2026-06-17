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
		groups := resolveGroups(r, h.cfg.Auth.HeaderTrust.GroupsHeader)
		u := &User{Email: email, Name: name, Groups: groups}
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *HeaderTrustAuthenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Login is handled by your identity proxy. Access Artifact through your corporate SSO."}`, http.StatusBadRequest)
}

// resolveGroups reads the configured groups header and splits its comma-separated
// values into trimmed group names. When no groups header is configured, the header
// is absent, or it contains no non-empty tokens, it falls back to ["employees"].
func resolveGroups(r *http.Request, groupsHeader string) []string {
	if groupsHeader == "" {
		return []string{"employees"}
	}
	raw := r.Header.Get(groupsHeader)
	if raw == "" {
		return []string{"employees"}
	}
	parts := strings.Split(raw, ",")
	groups := make([]string, 0, len(parts))
	for _, p := range parts {
		if g := strings.TrimSpace(p); g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		return []string{"employees"}
	}
	return groups
}
