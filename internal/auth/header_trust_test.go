package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

func headerTrustConfig(groupsHeader string) *config.Config {
	return &config.Config{
		Auth: config.Auth{
			Mode: "header-trust",
			HeaderTrust: config.HeaderTrust{
				EmailHeader:  "X-Email",
				NameHeader:   "X-Name",
				GroupsHeader: groupsHeader,
			},
		},
	}
}

func callMiddleware(t *testing.T, h *HeaderTrustAuthenticator, req *http.Request) (*http.Response, *User) {
	t.Helper()
	var capturedUser *User
	handler := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result(), capturedUser
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHeaderTrust_GroupsFromHeader(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	req.Header.Set("X-Email", "alice@example.com")
	req.Header.Set("X-Auth-Request-Groups", "admins,employees")

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if user == nil {
		t.Fatal("expected user in context, got nil")
	}
	if !equalStrings(user.Groups, []string{"admins", "employees"}) {
		t.Errorf("groups = %v, want [admins employees]", user.Groups)
	}
}

func TestHeaderTrust_MissingGroupsHeaderFallsBack(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	req.Header.Set("X-Email", "bob@example.com")
	// No X-Auth-Request-Groups header set.

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !equalStrings(user.Groups, []string{"employees"}) {
		t.Errorf("groups = %v, want [employees]", user.Groups)
	}
}

func TestHeaderTrust_EmptyGroupsHeaderFallsBack(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	req.Header.Set("X-Email", "carol@example.com")
	req.Header.Set("X-Auth-Request-Groups", "")

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !equalStrings(user.Groups, []string{"employees"}) {
		t.Errorf("groups = %v, want [employees]", user.Groups)
	}
}

func TestHeaderTrust_NoGroupsHeaderConfiguredFallsBack(t *testing.T) {
	// GroupsHeader not configured: old behavior preserved regardless of request headers.
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig(""), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	req.Header.Set("X-Email", "dave@example.com")
	req.Header.Set("X-Auth-Request-Groups", "admins,super-admins")

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !equalStrings(user.Groups, []string{"employees"}) {
		t.Errorf("groups = %v, want [employees]", user.Groups)
	}
}

func TestHeaderTrust_InvalidProxyAuthReturns401(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "wrong-secret")
	req.Header.Set("X-Email", "eve@example.com")

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if user != nil {
		t.Errorf("expected no user in context, got %v", user)
	}
}

func TestHeaderTrust_MissingEmailReturns401(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	// No email header set.

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if user != nil {
		t.Errorf("expected no user in context, got %v", user)
	}
}

func TestHeaderTrust_GroupsWhitespaceTrimmed(t *testing.T) {
	h := &HeaderTrustAuthenticator{cfg: headerTrustConfig("X-Auth-Request-Groups"), secret: "test-secret"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Artifact-Proxy-Auth", "test-secret")
	req.Header.Set("X-Email", "frank@example.com")
	req.Header.Set("X-Auth-Request-Groups", "admins , employees , hr-team")

	resp, user := callMiddleware(t, h, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !equalStrings(user.Groups, []string{"admins", "employees", "hr-team"}) {
		t.Errorf("groups = %v, want [admins employees hr-team]", user.Groups)
	}
}
