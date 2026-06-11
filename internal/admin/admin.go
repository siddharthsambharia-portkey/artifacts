package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
)

type Handler struct {
	cfg *config.Config
	db  *db.DB
}

func NewHandler(cfg *config.Config, database *db.DB) *Handler {
	return &Handler{cfg: cfg, db: database}
}

func (h *Handler) Audit(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	site := r.URL.Query().Get("site")
	user := r.URL.Query().Get("user")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	entries, err := h.db.SearchAudit(r.Context(), site, user, limit)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	if entries == nil {
		entries = []db.AuditEntry{}
	}
	writeJSON(w, entries)
}

func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT user_email, site, SUM(tokens) as total_tokens, COUNT(*) as requests
		 FROM ai_usage GROUP BY user_email, site ORDER BY total_tokens DESC LIMIT 100`)
	type usageRow struct {
		UserEmail   string `json:"user_email"`
		Site        string `json:"site"`
		TotalTokens int    `json:"total_tokens"`
		Requests    int    `json:"requests"`
	}
	var out []usageRow
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u usageRow
			rows.Scan(&u.UserEmail, &u.Site, &u.TotalTokens, &u.Requests)
			out = append(out, u)
		}
	}
	if out == nil {
		out = []usageRow{}
	}
	writeJSON(w, out)
}

func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	writeJSON(w, map[string]any{
		"governance_mode": h.cfg.Governance.Mode,
		"quotas":          h.cfg.Governance.Quotas,
		"domain":          h.cfg.Domain,
		"auth_mode":       h.cfg.Auth.Mode,
		"storage_driver":  h.cfg.Storage.Driver,
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	sites, _ := h.db.ListSites(r.Context())
	var totalBytes int64
	for _, s := range sites {
		totalBytes += s.SizeBytes
	}
	writeJSON(w, map[string]any{
		"site_count":  len(sites),
		"total_bytes": totalBytes,
		"governance":  h.cfg.Governance.Mode,
	})
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := auth.UserFromContext(r.Context())
	if u == nil || !governance.New(h.cfg).IsAdmin(u) {
		writeError(w, "Admin access required. Your account must be in the admins group.", http.StatusForbidden)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
