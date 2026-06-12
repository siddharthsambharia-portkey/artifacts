package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"golang.org/x/oauth2"
)

type OIDCAuthenticator struct {
	cfg          *config.Config
	provider     *oidc.Provider
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	sessions     *SessionStore
	memSessions  map[string]*memSession
	mu           sync.RWMutex
	useDB        bool
}

type memSession struct {
	User      *User
	ExpiresAt time.Time
}

func NewOIDCAuthenticator(cfg *config.Config, database *db.DB) (*OIDCAuthenticator, error) {
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, cfg.Auth.OIDC.Issuer)
	if err != nil {
		return nil, fmt.Errorf("connect to OIDC issuer %s: %w — check issuer URL and network", cfg.Auth.OIDC.Issuer, err)
	}
	secret := ""
	if cfg.Auth.OIDC.ClientSecretEnv != "" {
		secret = os.Getenv(cfg.Auth.OIDC.ClientSecretEnv)
	}
	scheme := "https"
	if cfg.TLS.Mode == "off" || cfg.Domain == "localhost" {
		scheme = "http"
	}
	oauth2Config := oauth2.Config{
		ClientID:     cfg.Auth.OIDC.ClientID,
		ClientSecret: secret,
		RedirectURL:  fmt.Sprintf("%s://%s/auth/callback", scheme, cfg.Domain),
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.Auth.OIDC.ClientID})
	o := &OIDCAuthenticator{
		cfg: cfg, provider: provider, oauth2Config: oauth2Config, verifier: verifier,
		memSessions: make(map[string]*memSession),
	}
	if database != nil {
		o.sessions = NewSessionStore(database)
		o.useDB = true
	}
	return o, nil
}

func (o *OIDCAuthenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" || r.URL.Path == "/auth/callback" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("artifact_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		u, ok := o.lookupSession(r.Context(), cookie.Value)
		if !ok {
			o.clearSessionCookie(w)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (o *OIDCAuthenticator) lookupSession(ctx context.Context, id string) (*User, bool) {
	if o.useDB && o.sessions != nil {
		u, err := o.sessions.Get(ctx, id)
		return u, err == nil
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	s, ok := o.memSessions[id]
	if !ok || time.Now().After(s.ExpiresAt) {
		return nil, false
	}
	return s.User, true
}

func (o *OIDCAuthenticator) storeSession(ctx context.Context, u *User) (string, error) {
	if o.useDB && o.sessions != nil {
		return o.sessions.Create(ctx, u, 24*time.Hour)
	}
	sid := randomState()
	o.mu.Lock()
	o.memSessions[sid] = &memSession{User: u, ExpiresAt: time.Now().Add(24 * time.Hour)}
	o.mu.Unlock()
	return sid, nil
}

func (o *OIDCAuthenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	secure := o.cfg.TLS.Mode != "off" && o.cfg.Domain != "localhost"
	http.SetCookie(w, &http.Cookie{Name: "artifact_oauth_state", Value: state, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 300})
	http.Redirect(w, r, o.oauth2Config.AuthCodeURL(state), http.StatusFound)
}

func (o *OIDCAuthenticator) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("artifact_oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, `{"error":"Invalid OAuth state. Try logging in again."}`, http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	token, err := o.oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"OAuth exchange failed: %v. Check client_id and client_secret."}`, err), http.StatusBadRequest)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, `{"error":"No id_token in OAuth response. Ensure openid scope is granted."}`, http.StatusBadRequest)
		return
	}
	idToken, err := o.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"ID token verification failed: %v"}`, err), http.StatusBadRequest)
		return
	}
	groupsClaim := o.cfg.Auth.OIDC.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, `{"error":"Failed to parse ID token claims."}`, http.StatusBadRequest)
		return
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	var groups []string
	if g, ok := claims[groupsClaim].([]any); ok {
		for _, v := range g {
			if s, ok := v.(string); ok {
				groups = append(groups, s)
			}
		}
	}
	if len(groups) == 0 {
		groups = []string{"employees"}
	}
	u := &User{Email: email, Name: name, Groups: groups}
	sid, err := o.storeSession(r.Context(), u)
	if err != nil {
		http.Error(w, `{"error":"Failed to create session."}`, http.StatusInternalServerError)
		return
	}
	secure := o.cfg.TLS.Mode != "off" && o.cfg.Domain != "localhost"
	domain := o.cfg.Domain
	cookie := &http.Cookie{
		Name: "artifact_session", Value: sid, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 86400,
	}
	if domain != "localhost" {
		cookie.Domain = "." + domain
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (o *OIDCAuthenticator) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "artifact_session", Value: "", MaxAge: -1, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func randomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// ParseGroupsJSON for tests.
func ParseGroupsJSON(raw string) []string {
	var groups []string
	_ = json.Unmarshal([]byte(raw), &groups)
	return groups
}
