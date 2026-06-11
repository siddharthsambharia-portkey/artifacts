package config

import "testing"

func TestSiteFromHost(t *testing.T) {
	cfg := DefaultDev()
	tests := []struct {
		host string
		want string
	}{
		{"guestbook.localhost:8443", "guestbook"},
		{"admin.localhost:8443", ""},
		{"localhost:8443", ""},
	}
	for _, tt := range tests {
		got := cfg.SiteFromHost(tt.host)
		if got != tt.want {
			t.Errorf("SiteFromHost(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestHeaderTrustRequiresSecret(t *testing.T) {
	cfg := DefaultDev()
	cfg.Auth.Mode = "header-trust"
	cfg.Auth.HeaderTrust.ProxySecretEnv = "ARTIFACT_PROXY_SECRET"
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error when proxy secret not set")
	}
}

func TestValidSiteName(t *testing.T) {
	cfg := DefaultDev()
	cfg.Governance.Mode = "governed"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
