package warehouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

var selectOnly = regexp.MustCompile(`(?i)^\s*SELECT\b`)
var denyPatterns = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|GRANT|REVOKE|EXEC|EXECUTE|UNION|INTO|MERGE|CALL)\b`)
var denyTokens = regexp.MustCompile(`;|--|/\*`)

type Handler struct {
	cfg     *config.Config
	db      *db.DB
	querier Querier
}

func NewHandler(cfg *config.Config, database *db.DB) (*Handler, error) {
	q, err := NewQuerier(cfg)
	if err != nil {
		return nil, err
	}
	return &Handler{cfg: cfg, db: database, querier: q}, nil
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	if h.querier == nil {
		http.Error(w, `{"error":"Warehouse is not configured. Set warehouse.driver and credentials in artifact.yaml."}`, http.StatusServiceUnavailable)
		return
	}
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)

	if err := h.checkDailyQuota(r, u.Email); err != nil {
		if isQuotaLimit(err) {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusTooManyRequests)
		} else {
			http.Error(w, `{"error":"quota check failed"}`, http.StatusInternalServerError)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
	var req struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON body."}`, http.StatusBadRequest)
		return
	}
	if !selectOnly.MatchString(req.SQL) {
		http.Error(w, `{"error":"Only SELECT queries are allowed. Warehouse access is read-only."}`, http.StatusForbidden)
		return
	}
	if denyPatterns.MatchString(req.SQL) {
		http.Error(w, `{"error":"Query contains forbidden keywords. Only SELECT is allowed."}`, http.StatusForbidden)
		return
	}
	if denyTokens.MatchString(req.SQL) {
		http.Error(w, `{"error":"Query contains forbidden tokens (;, comments) — submit a single plain SELECT."}`, http.StatusForbidden)
		return
	}
	if !h.datasetAllowed(req.SQL) {
		http.Error(w, `{"error":"Query references a dataset not in warehouse.allowed_datasets. Ask an admin to add it."}`, http.StatusForbidden)
		return
	}
	limit := h.cfg.Warehouse.RowLimit
	if limit <= 0 {
		limit = 10000
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	rows, err := h.querier.Query(ctx, req.SQL, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Query failed: %v"}`, err), http.StatusBadRequest)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	detail := req.SQL
	if len(detail) > 200 {
		detail = detail[:200]
	}
	_ = h.db.InsertAudit(r.Context(), &db.AuditEntry{
		Timestamp: time.Now(), UserEmail: u.Email,
		Site: site, Action: "warehouse_query", Detail: detail,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"rows": rows, "truncated": len(rows) >= limit})
}

func (h *Handler) datasetAllowed(sql string) bool {
	if len(h.cfg.Warehouse.AllowedDatasets) == 0 {
		return false
	}
	lower := strings.ToLower(sql)
	for _, ds := range h.cfg.Warehouse.AllowedDatasets {
		if strings.Contains(lower, strings.ToLower(ds)) {
			return true
		}
	}
	return false
}

type quotaLimitError struct {
	msg string
}

func (e quotaLimitError) Error() string { return e.msg }

func (h *Handler) checkDailyQuota(r *http.Request, email string) error {
	max := h.cfg.Governance.Quotas.WarehouseDailyQueriesPerUser
	if max <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	count, err := h.db.CountWarehouseQueriesSince(r.Context(), cutoff)
	if err != nil {
		return fmt.Errorf("quota check failed: %w", err)
	}
	if count >= max {
		return quotaLimitError{msg: fmt.Sprintf("daily warehouse query limit (%d) reached. Try again tomorrow or ask an admin", max)}
	}
	return nil
}

func isQuotaLimit(err error) bool {
	var ql quotaLimitError
	return errors.As(err, &ql)
}

// ValidateUpstreamURL ensures AI upstream is SSRF-safe (used by ai package pattern).
func ValidateUpstreamURL(cfgURL, allowed string) error {
	u, err := url.Parse(cfgURL)
	if err != nil {
		return err
	}
	a, err := url.Parse(allowed)
	if err != nil {
		return err
	}
	if u.Host != a.Host {
		return fmt.Errorf("upstream host mismatch")
	}
	return nil
}
