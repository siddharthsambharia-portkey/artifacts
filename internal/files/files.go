package files

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

type FilesConfig interface {
	SiteFromHost(host string) string
	UploadMaxMB() int
}

type Handler struct {
	cfg   FilesConfig
	store storage.Store
	db    *db.DB
}

func NewHandler(cfg FilesConfig, store storage.Store, database *db.DB) *Handler {
	return &Handler{cfg: cfg, store: store, db: database}
}

type UploadResponse struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type FileRecord struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	URL         string    `json:"url"`
	UploadedBy  string    `json:"uploaded_by"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)
	maxBytes := int64(h.cfg.UploadMaxMB()) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, `{"error":"File too large or invalid multipart form. Check upload_max_mb in artifact.yaml."}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"Missing file field. Send multipart form with a 'file' field."}`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	id := randomID()
	ext := filepath.Ext(header.Filename)
	storagePath := fmt.Sprintf("uploads/%s/%s%s", site, id, ext)
	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = mime.TypeByExtension(ext)
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	if isDangerousContentType(ct) {
		http.Error(w, `{"error":"Executable content types are not allowed. Upload images, PDFs, or data files only."}`, http.StatusBadRequest)
		return
	}
	if err := h.store.Put(r.Context(), storagePath, file, header.Size, ct); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Upload failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("/api/v1/files/%s", id)
	_ = h.db.InsertFile(r.Context(), &db.FileRecord{
		ID:          id,
		Site:        site,
		Filename:    header.Filename,
		ContentType: ct,
		SizeBytes:   header.Size,
		StoragePath: storagePath,
		UploadedBy:  u.Email,
		UploadedAt:  time.Now(),
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{ID: id, URL: url, Filename: header.Filename, Size: header.Size})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.SiteFromHost(r.Host)
	recs, err := h.db.ListFiles(r.Context(), site, 100)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"List files failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	files := make([]FileRecord, 0, len(recs))
	for _, rec := range recs {
		files = append(files, FileRecord{
			ID:          rec.ID,
			Filename:    rec.Filename,
			ContentType: rec.ContentType,
			Size:        rec.SizeBytes,
			URL:         fmt.Sprintf("/api/v1/files/%s", rec.ID),
			UploadedBy:  rec.UploadedBy,
			UploadedAt:  rec.UploadedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/files/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			id = parts[0]
		}
	}
	if id == "" {
		http.NotFound(w, r)
		return
	}
	site := h.cfg.SiteFromHost(r.Host)
	var storagePath string
	rec, err := h.db.GetFileByID(r.Context(), site, id)
	if err != nil || rec == nil {
		objs, _ := h.store.List(r.Context(), fmt.Sprintf("uploads/%s/%s", site, id))
		if len(objs) == 0 {
			http.NotFound(w, r)
			return
		}
		storagePath = objs[0].Path
	} else {
		storagePath = rec.StoragePath
	}
	rc, info, err := h.store.Get(r.Context(), storagePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+id+"\"")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, rc)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/files/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			id = parts[0]
		}
	}
	if id == "" {
		http.NotFound(w, r)
		return
	}
	site := h.cfg.SiteFromHost(r.Host)
	rec, err := h.db.GetFileByID(r.Context(), site, id)
	if err != nil || rec == nil {
		http.NotFound(w, r)
		return
	}
	if err := h.store.Delete(r.Context(), rec.StoragePath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Delete failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	if err := h.db.DeleteFile(r.Context(), site, id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Delete failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isDangerousContentType(ct string) bool {
	dangerous := []string{"text/html", "application/javascript", "text/javascript", "application/xhtml+xml"}
	for _, d := range dangerous {
		if strings.HasPrefix(ct, d) {
			return true
		}
	}
	return false
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
