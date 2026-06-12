package warehouse

import (
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

func TestSelectOnly(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		match bool
	}{
		{"plain select", "SELECT 1", true},
		{"leading whitespace lowercase", "  select x", true},
		{"insert rejected", "INSERT INTO t VALUES (1)", false},
		{"cte rejected today", "WITH t AS (SELECT 1) SELECT * FROM t", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectOnly.MatchString(tt.sql); got != tt.match {
				t.Fatalf("selectOnly.MatchString(%q) = %v, want %v", tt.sql, got, tt.match)
			}
		})
	}
}

func TestDenyPatterns(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		match bool
	}{
		{"insert keyword", "SELECT 1; INSERT INTO t VALUES (1)", true},
		{"drop keyword", "SELECT * FROM t WHERE x='DROP'", true},
		{"delete keyword", "SELECT 1 WHERE DELETE=1", true},
		{
			name:  "union not denied today",
			sql:   "SELECT 1 UNION SELECT 2",
			match: false,
			// gap closed in plans/006
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := denyPatterns.MatchString(tt.sql); got != tt.match {
				t.Fatalf("denyPatterns.MatchString(%q) = %v, want %v", tt.sql, got, tt.match)
			}
		})
	}
}

func TestDatasetAllowed(t *testing.T) {
	cfg := config.DefaultDev()
	cfg.Warehouse.AllowedDatasets = []string{"analytics.public"}
	h := &Handler{cfg: cfg}

	tests := []struct {
		name    string
		sql     string
		allowed bool
	}{
		{
			name:    "allowed dataset",
			sql:     "SELECT * FROM analytics.public.orders",
			allowed: true,
		},
		{
			name: "union substring bypass",
			// BUG: substring match — see plans/006
			sql:     "SELECT * FROM analytics.public.orders UNION SELECT * FROM secret.salaries",
			allowed: true,
		},
		{
			name:    "no allowed dataset mentioned",
			sql:     "SELECT * FROM secret.salaries",
			allowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.datasetAllowed(tt.sql); got != tt.allowed {
				t.Fatalf("datasetAllowed(%q) = %v, want %v", tt.sql, got, tt.allowed)
			}
		})
	}
}

func TestDatasetAllowedEmptyAllowlist(t *testing.T) {
	cfg := config.DefaultDev()
	cfg.Warehouse.AllowedDatasets = nil
	h := &Handler{cfg: cfg}
	if h.datasetAllowed("SELECT * FROM analytics.public.orders") {
		t.Fatal("expected deny when allowlist is empty")
	}
}
