package sites

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

type ReadAuthorizer func(ctx context.Context, user *auth.User, site string) error

type StaticHandler struct {
	cfg   *config.Config
	store storage.Store
	cache *DeployCache
	authz ReadAuthorizer
}

func NewStaticHandler(cfg *config.Config, store storage.Store, cache *DeployCache, authz ReadAuthorizer) *StaticHandler {
	return &StaticHandler{cfg: cfg, store: store, cache: cache, authz: authz}
}

// allowedOrigin reports whether origin (e.g. "https://site-a.artifact.corp.com")
// is another site on this Artifact instance, per ADR 0004.
func (h *StaticHandler) allowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.Split(u.Host, ":")[0]
	d := h.cfg.Domain
	return host == d || strings.HasSuffix(host, "."+d)
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.SiteFromHost(r.Host)
	if site == "" {
		http.NotFound(w, r)
		return
	}
	if h.authz != nil {
		if err := h.authz(r.Context(), auth.UserFromContext(r.Context()), site); err != nil {
			http.Error(w, "You do not have access to this site.", http.StatusForbidden)
			return
		}
	}
	ctx := r.Context()
	deployID, err := h.cache.CurrentDeployID(ctx, site)
	if err != nil || deployID == "" {
		h.renderNoSite(w, site)
		return
	}
	reqPath := r.URL.Path
	if reqPath == "/" {
		reqPath = "/index.html"
	}
	reqPath = strings.TrimPrefix(reqPath, "/")
	if strings.Contains(reqPath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	storagePath := fmt.Sprintf("sites/%s/deploys/%s/%s", site, deployID, reqPath)
	rc, info, err := h.store.Get(ctx, storagePath)
	if err != nil {
		if reqPath != "index.html" {
			http.NotFound(w, r)
			return
		}
		h.renderNoSite(w, site)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("ETag", `"`+info.ETag+`"`)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if isAttachmentType(info.ContentType) {
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
	}
	if origin := r.Header.Get("Origin"); origin != "" && h.allowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Vary", "Origin")
	if r.Header.Get("If-None-Match") == `"`+info.ETag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	io.Copy(w, rc)
}

func (h *StaticHandler) renderNoSite(w http.ResponseWriter, site string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>No such site</title>
<style>body{font-family:system-ui;max-width:600px;margin:4rem auto;padding:0 1rem;color:#333}
h1{color:#666}code{background:#f4f4f4;padding:2px 6px;border-radius:4px}</style></head>
<body><h1>No such site</h1>
<p>The site <code>%s</code> has not been deployed yet.</p>
<p>Run <code>artifact deploy</code> from your project folder to publish it.</p>
</body></html>`, site)
}

func isAttachmentType(ct string) bool {
	dangerous := []string{"text/html", "application/javascript", "text/javascript", "application/xhtml+xml"}
	for _, d := range dangerous {
		if strings.HasPrefix(ct, d) {
			return false
		}
	}
	ext := mime.TypeByExtension(filepath.Ext(ct))
	_ = ext
	return strings.HasPrefix(ct, "application/") && !strings.HasPrefix(ct, "application/json")
}
