package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
)

type Handler struct {
	cfg *config.Config
	db  *db.DB
	gov *governance.Governor
}

func NewHandler(cfg *config.Config, database *db.DB, gov *governance.Governor) *Handler {
	return &Handler{cfg: cfg, db: database, gov: gov}
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
	out, err := h.db.ListAIUsageSummary(r.Context(), 100)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []db.UsageSummary{}
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

func (h *Handler) SetVisibility(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	site := chi.URLParam(r, "site")
	var body struct {
		Visibility string   `json:"visibility"`
		Groups     []string `json:"groups"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	switch body.Visibility {
	case "private", "group", "public":
	default:
		writeError(w, "visibility must be private, group, or public", http.StatusBadRequest)
		return
	}
	if body.Visibility == "group" && len(body.Groups) == 0 {
		writeError(w, "groups required when visibility is group", http.StatusBadRequest)
		return
	}
	ok, err := h.db.UpdateSiteVisibility(r.Context(), site, body.Visibility, body.Groups)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		writeError(w, "site not found", http.StatusNotFound)
		return
	}
	u := auth.UserFromContext(r.Context())
	detail, _ := json.Marshal(map[string]any{"visibility": body.Visibility, "groups": body.Groups})
	_ = h.db.InsertAudit(r.Context(), &db.AuditEntry{
		Timestamp: time.Now(), UserEmail: u.Email, Site: site,
		Action: "visibility_change", Detail: string(detail),
	})
	writeJSON(w, map[string]string{"status": "ok"})
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
	if u == nil || !h.gov.IsAdmin(u) {
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
