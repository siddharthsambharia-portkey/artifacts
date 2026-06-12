package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/realtime"
	"github.com/go-chi/chi/v5"
)

type API struct {
	cfg   *config.Config
	db    *db.DB
	gov   *governance.Governor
	events realtime.EventPublisher
}

func NewAPI(cfg *config.Config, database *db.DB, gov *governance.Governor, events realtime.EventPublisher) *API {
	return &API{cfg: cfg, db: database, gov: gov, events: events}
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/me", a.handleMe)
	r.Route("/db", func(r chi.Router) {
		r.Post("/{collection}", a.handleCreate)
		r.Get("/{collection}", a.handleList)
		r.Put("/{collection}/{id}", a.handleUpdate)
		r.Delete("/{collection}/{id}", a.handleDelete)
	})
	r.Route("/kv", func(r chi.Router) {
		r.Put("/{key}", a.handleKVSet)
		r.Get("/{key}", a.handleKVGet)
	})
	return r
}

func (a *API) siteFromRequest(r *http.Request) string {
	site := a.cfg.SiteFromHost(r.Host)
	if site == "" {
		site = r.Header.Get("X-Artifact-Site")
	}
	return site
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	writeJSON(w, u)
}

func (a *API) handleCreate(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	collection := chi.URLParam(r, "collection")

	if err := a.checkQuota(r, site); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	existing, _ := a.db.GetSite(r.Context(), site)
	if err := a.gov.CanWriteDB(r.Context(), u, site, existing); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
	var data json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "Invalid JSON body.", http.StatusBadRequest)
		return
	}
	id := generateID()
	now := time.Now()
	doc := &db.Document{
		ID: id, Site: site, Collection: collection, Data: data,
		CreatedBy: u.Email, UpdatedBy: u.Email, CreatedAt: now, UpdatedAt: now,
	}
	if err := a.db.CreateDocument(r.Context(), doc); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.publishDoc(site, collection, "create", doc)
	_ = a.db.InsertAudit(r.Context(), &db.AuditEntry{
		Timestamp: now, UserEmail: u.Email, Site: site,
		Action: "db_create", Detail: collection + "/" + id,
	})
	writeJSON(w, doc)
}

func (a *API) handleList(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	crossSite := r.URL.Query().Get("site")
	if crossSite != "" {
		if !a.gov.IsTrustMode() {
			target, _ := a.db.GetSite(r.Context(), crossSite)
			if err := a.gov.CanReadSite(r.Context(), u, crossSite, target); err != nil {
				writeError(w, err.Error(), http.StatusForbidden)
				return
			}
		}
		site = crossSite
	}
	collection := chi.URLParam(r, "collection")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	docs, err := a.db.ListDocuments(r.Context(), site, collection, limit, true)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if docs == nil {
		docs = []db.Document{}
	}
	writeJSON(w, docs)
}

func (a *API) handleUpdate(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	id := chi.URLParam(r, "id")
	collection := chi.URLParam(r, "collection")
	existing, _ := a.db.GetSite(r.Context(), site)
	if err := a.gov.CanWriteDB(r.Context(), u, site, existing); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	doc, err := a.db.GetDocument(r.Context(), site, id)
	if err != nil || doc == nil {
		writeError(w, "Document not found.", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
	var data json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "Invalid JSON body.", http.StatusBadRequest)
		return
	}
	doc.Data = data
	doc.Collection = collection
	doc.UpdatedBy = u.Email
	doc.UpdatedAt = time.Now()
	if err := a.db.UpdateDocument(r.Context(), doc); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.publishDoc(site, collection, "update", doc)
	writeJSON(w, doc)
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	id := chi.URLParam(r, "id")
	collection := chi.URLParam(r, "collection")
	existing, _ := a.db.GetSite(r.Context(), site)
	if err := a.gov.CanWriteDB(r.Context(), u, site, existing); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	doc, _ := a.db.GetDocument(r.Context(), site, id)
	if err := a.db.DeleteDocument(r.Context(), site, id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if doc != nil {
		a.publishDoc(site, collection, "delete", doc)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleKVSet(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	key := chi.URLParam(r, "key")
	existing, _ := a.db.GetSite(r.Context(), site)
	if err := a.gov.CanWriteDB(r.Context(), u, site, existing); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "Invalid JSON body.", http.StatusBadRequest)
		return
	}
	if err := a.db.SetKV(r.Context(), site, key, body.Value); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"key": key, "value": body.Value})
}

func (a *API) handleKVGet(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := a.siteFromRequest(r)
	key := chi.URLParam(r, "key")
	existing, _ := a.db.GetSite(r.Context(), site)
	if err := a.gov.CanReadSite(r.Context(), u, site, existing); err != nil {
		writeError(w, err.Error(), http.StatusForbidden)
		return
	}
	v, err := a.db.GetKV(r.Context(), site, key)
	if err != nil {
		writeError(w, "Key not found.", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"key": key, "value": v})
}

func (a *API) publishDoc(site, collection, eventType string, doc *db.Document) {
	if a.events == nil {
		return
	}
	if hub, ok := a.events.(*realtime.Hub); ok {
		hub.PublishDocumentEvent(site, collection, eventType, doc)
	}
}

func (a *API) checkQuota(r *http.Request, site string) error {
	max := a.cfg.Governance.Quotas.DBMaxDocsPerSite
	if max <= 0 {
		return nil
	}
	n, err := a.db.CountDocuments(r.Context(), site)
	if err != nil {
		return err
	}
	if n >= max {
		return fmt.Errorf("site %q has %d documents (limit %d). Delete old data or ask an admin to raise db_max_docs_per_site", site, n, max)
	}
	return nil
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

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return time.Now().Format("20060102150405") + hex.EncodeToString(b)
}
