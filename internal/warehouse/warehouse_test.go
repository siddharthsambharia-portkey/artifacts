package warehouse

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
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
		{"union denied", "SELECT 1 UNION SELECT 2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := denyPatterns.MatchString(tt.sql); got != tt.match {
				t.Fatalf("denyPatterns.MatchString(%q) = %v, want %v", tt.sql, got, tt.match)
			}
		})
	}
}

func TestDenyTokens(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		match bool
	}{
		{"semicolon", "SELECT 1; SELECT 2", true},
		{"line comment", "SELECT 1 -- drop table", true},
		{"block comment", "SELECT 1 /* secret */", true},
		{"plain select", "SELECT a, b FROM t WHERE x = 'val'", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := denyTokens.MatchString(tt.sql); got != tt.match {
				t.Fatalf("denyTokens.MatchString(%q) = %v, want %v", tt.sql, got, tt.match)
			}
		})
	}
}

func TestQueryGuards(t *testing.T) {
	cfg := config.DefaultDev()
	cfg.Warehouse.AllowedDatasets = []string{"analytics.public"}
	h := &Handler{cfg: cfg}

	tests := []struct {
		name    string
		sql     string
		allowed bool
	}{
		{
			name:    "plain select with filters",
			sql:     "SELECT a, b FROM analytics.public.t WHERE x = 'val' ORDER BY a DESC LIMIT 50",
			allowed: true,
		},
		{
			name:    "high limit still passes guards",
			sql:     "SELECT * FROM analytics.public.t LIMIT 999999",
			allowed: true,
		},
		{
			name:    "union exfiltration denied",
			sql:     "SELECT * FROM analytics.public.orders UNION SELECT * FROM secret.salaries",
			allowed: false,
		},
		{
			name:    "semicolon multi-statement denied",
			sql:     "SELECT * FROM analytics.public.t; SELECT * FROM secret.salaries",
			allowed: false,
		},
		{
			name:    "line comment denied",
			sql:     "SELECT * FROM analytics.public.t -- bypass",
			allowed: false,
		},
		{
			name:    "block comment denied",
			sql:     "SELECT * FROM analytics.public.t /* bypass */",
			allowed: false,
		},
		{
			name:    "insert denied",
			sql:     "INSERT INTO analytics.public.t VALUES (1)",
			allowed: false,
		},
		{
			name:    "cte denied",
			sql:     "WITH t AS (SELECT 1) SELECT * FROM analytics.public.t",
			allowed: false,
		},
		{
			name: "union in quoted literal false positive",
			// Accepted trade-off: keyword guard matches inside string literals.
			sql:     "SELECT * FROM analytics.public.t WHERE note = 'union all hands'",
			allowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryAllowed(h, tt.sql); got != tt.allowed {
				t.Fatalf("queryAllowed(%q) = %v, want %v", tt.sql, got, tt.allowed)
			}
		})
	}
}

func queryAllowed(h *Handler, sql string) bool {
	if !selectOnly.MatchString(sql) {
		return false
	}
	if denyPatterns.MatchString(sql) {
		return false
	}
	if denyTokens.MatchString(sql) {
		return false
	}
	return h.datasetAllowed(sql)
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
			name: "union substring still matches datasetAllowed",
			// datasetAllowed is substring-based; UNION is blocked upstream by denyPatterns (plans/006).
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

type fakeQuerier struct {
	rows      []map[string]any
	lastLimit int
}

func (f *fakeQuerier) Query(_ context.Context, _ string, rowLimit int) ([]map[string]any, error) {
	f.lastLimit = rowLimit
	return f.rows, nil
}

func (f *fakeQuerier) Close() error { return nil }

func testHandler(t *testing.T, fq *fakeQuerier, rowLimit int) *Handler {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	cfg.Warehouse.AllowedDatasets = []string{"analytics.public"}
	cfg.Warehouse.RowLimit = rowLimit
	cfg.Governance.Quotas.WarehouseDailyQueriesPerUser = 0

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &Handler{cfg: cfg, db: database, querier: fq}
}

func TestQueryTruncatedFlag(t *testing.T) {
	fq := &fakeQuerier{
		rows: []map[string]any{
			{"id": 1},
			{"id": 2},
		},
	}
	h := testHandler(t, fq, 2)

	body := `{"sql":"SELECT id FROM analytics.public.t LIMIT 50"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/query", bytes.NewReader([]byte(body)))
	req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
	w := httptest.NewRecorder()
	h.Query(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if fq.lastLimit != 2 {
		t.Fatalf("querier rowLimit = %d, want 2", fq.lastLimit)
	}

	var resp struct {
		Rows      []map[string]any `json:"rows"`
		Truncated bool             `json:"truncated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated {
		t.Fatal("expected truncated=true when row count equals limit")
	}
}

func TestQueryNotTruncated(t *testing.T) {
	fq := &fakeQuerier{
		rows: []map[string]any{{"id": 1}},
	}
	h := testHandler(t, fq, 10)

	body := `{"sql":"SELECT id FROM analytics.public.t"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/query", bytes.NewReader([]byte(body)))
	req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
	w := httptest.NewRecorder()
	h.Query(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	var resp struct {
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Truncated {
		t.Fatal("expected truncated=false when row count below limit")
	}
}
