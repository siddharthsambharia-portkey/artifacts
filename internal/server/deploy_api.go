package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
)

const maxDeployFiles = 10000

type deployResponse struct {
	Site       string   `json:"site"`
	URL        string   `json:"url"`
	DeployID   string   `json:"deploy_id"`
	FileCount  int      `json:"file_count"`
	TotalBytes int64    `json:"total_bytes"`
	Warnings   []string `json:"warnings"`
}

type deployConflictResponse struct {
	Error          string `json:"error"`
	Exists         bool   `json:"exists"`
	LastDeployedBy string `json:"last_deployed_by"`
	Owner          string `json:"owner"`
}

type stageError struct {
	msg  string
	code int
}

func (e *stageError) Error() string { return e.msg }

type prepareError struct {
	msg  string
	code int
}

func (e *prepareError) Error() string { return e.msg }

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	maxBytes := int64(s.cfg.Governance.Quotas.SiteMaxMB)*1024*1024 + (8 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, "Upload too large or malformed multipart form.", http.StatusBadRequest)
		return
	}

	site := r.FormValue("site")
	if site == "" {
		writeError(w, "Site name is required.", http.StatusBadRequest)
		return
	}

	tmp, err := os.MkdirTemp("", "artifact-deploy-*")
	if err != nil {
		writeError(w, "Failed to create staging directory.", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmp)

	fileHeaders := r.MultipartForm.File["files"]
	zipHeaders := r.MultipartForm.File["zip"]

	if len(zipHeaders) > 0 && len(fileHeaders) > 0 {
		writeError(w, "Send either zip or files, not both.", http.StatusBadRequest)
		return
	}
	if len(zipHeaders) == 0 && len(fileHeaders) == 0 {
		writeError(w, "No files uploaded. Send multipart parts named files or zip.", http.StatusBadRequest)
		return
	}

	var fileCount int
	var totalBytes int64
	quotaBytes := int64(s.cfg.Governance.Quotas.SiteMaxMB) * 1024 * 1024

	if len(fileHeaders) > 0 {
		if len(fileHeaders) > maxDeployFiles {
			writeError(w, fmt.Sprintf("Too many files (max %d).", maxDeployFiles), http.StatusRequestEntityTooLarge)
			return
		}
		for _, fh := range fileHeaders {
			n, err := stageMultipartFile(fh, tmp)
			if err != nil {
				if se, ok := err.(*stageError); ok {
					writeError(w, se.msg, se.code)
					return
				}
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fileCount++
			totalBytes += n
		}
	} else {
		n, count, err := stageZipFile(zipHeaders[0], tmp, quotaBytes)
		if err != nil {
			if se, ok := err.(*stageError); ok {
				writeError(w, se.msg, se.code)
				return
			}
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fileCount = count
		totalBytes = n
	}

	existing, _ := s.db.GetSite(r.Context(), site)
	if existing != nil && r.FormValue("confirm_overwrite") != "true" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(deployConflictResponse{
			Error:          fmt.Sprintf("Site %q already exists.", site),
			Exists:         true,
			LastDeployedBy: existing.DeployedBy,
			Owner:          existing.Owner,
		})
		return
	}

	root, warnings, err := prepareDeployRoot(tmp)
	if err != nil {
		if pe, ok := err.(*prepareError); ok {
			writeError(w, pe.msg, pe.code)
			return
		}
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u := auth.UserFromContext(r.Context())
	url, err := s.deployer.Deploy(r.Context(), site, root, u)
	if err != nil {
		writeError(w, err.Error(), mapDeployError(err))
		return
	}

	rec, _ := s.db.GetSite(r.Context(), site)
	deployID := ""
	if rec != nil {
		deployID = rec.DeployID
	}

	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, deployResponse{
		Site: site, URL: url, DeployID: deployID,
		FileCount: fileCount, TotalBytes: totalBytes,
		Warnings: warnings,
	})
}

func stageMultipartFile(fh *multipart.FileHeader, dest string) (int64, error) {
	rel, err := sanitizeRelPath(fh.Filename)
	if err != nil {
		return 0, &stageError{msg: err.Error(), code: http.StatusBadRequest}
	}
	target := filepath.Join(dest, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return 0, err
	}
	src, err := fh.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	n, err := io.Copy(dst, src)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func stageZipFile(fh *multipart.FileHeader, dest string, quotaBytes int64) (totalBytes int64, fileCount int, err error) {
	src, err := fh.Open()
	if err != nil {
		return 0, 0, err
	}
	defer src.Close()

	zipTmp, err := os.CreateTemp("", "artifact-zip-*")
	if err != nil {
		return 0, 0, err
	}
	zipPath := zipTmp.Name()
	defer os.Remove(zipPath)

	if _, err := io.Copy(zipTmp, src); err != nil {
		zipTmp.Close()
		return 0, 0, err
	}
	if err := zipTmp.Close(); err != nil {
		return 0, 0, err
	}

	zipFile, err := os.Open(zipPath)
	if err != nil {
		return 0, 0, err
	}
	defer zipFile.Close()

	fi, err := zipFile.Stat()
	if err != nil {
		return 0, 0, err
	}
	zr, err := zip.NewReader(zipFile, fi.Size())
	if err != nil {
		return 0, 0, &stageError{msg: "Invalid zip archive.", code: http.StatusBadRequest}
	}

	if len(zr.File) > maxDeployFiles {
		return 0, 0, &stageError{
			msg:  fmt.Sprintf("Too many files in zip (max %d).", maxDeployFiles),
			code: http.StatusRequestEntityTooLarge,
		}
	}

	var remaining = quotaBytes
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return 0, 0, &stageError{msg: "zip contains symlinks", code: http.StatusBadRequest}
		}
		rel, err := sanitizeRelPath(f.Name)
		if err != nil {
			return 0, 0, &stageError{msg: err.Error(), code: http.StatusBadRequest}
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return 0, 0, err
		}
		rc, err := f.Open()
		if err != nil {
			return 0, 0, err
		}
		dst, err := os.Create(target)
		if err != nil {
			rc.Close()
			return 0, 0, err
		}
		n, err := io.Copy(dst, io.LimitReader(rc, remaining+1))
		rc.Close()
		dst.Close()
		if err != nil {
			return 0, 0, err
		}
		if n > remaining {
			return 0, 0, &stageError{
				msg:  fmt.Sprintf("Extracted content exceeds %d MB quota.", quotaBytes/(1024*1024)),
				code: http.StatusRequestEntityTooLarge,
			}
		}
		remaining -= n
		totalBytes += n
		fileCount++
	}
	return totalBytes, fileCount, nil
}

func sanitizeRelPath(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(normalized, "..") || strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("invalid file path %q in upload", name)
	}
	rel := filepath.ToSlash(filepath.Clean(normalized))
	if rel == "" || rel == "." || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "..") || strings.Contains(rel, "../") {
		return "", fmt.Errorf("invalid file path %q in upload", name)
	}
	return rel, nil
}

func prepareDeployRoot(tmp string) (string, []string, error) {
	root := tmp
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return "", nil, err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		root = filepath.Join(tmp, entries[0].Name())
	}

	if isSourceProject(root) {
		return "", nil, &prepareError{
			msg:  "This looks like a source project (package.json found). Artifact hosts static files — run your build locally (e.g. npm run build) and drop the output folder (dist/, build/, or out/).",
			code: http.StatusUnprocessableEntity,
		}
	}

	indexPath := filepath.Join(root, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return root, []string{}, nil
	}

	htmlFiles := findHTMLFiles(root)
	var warnings []string
	switch len(htmlFiles) {
	case 0:
		return "", nil, &prepareError{
			msg:  "No HTML files found. Drop a folder with an index.html.",
			code: http.StatusUnprocessableEntity,
		}
	case 1:
		name := filepath.Base(htmlFiles[0])
		data, err := os.ReadFile(htmlFiles[0])
		if err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(indexPath, data, 0644); err != nil {
			return "", nil, err
		}
		warnings = append(warnings, fmt.Sprintf("No index.html found — %s will load at your site's root.", name))
	default:
		warnings = append(warnings, "No index.html found — visitors will see a 404 at the root path.")
	}
	return root, warnings, nil
}

func isSourceProject(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "index.html")); err == nil {
		return false
	}
	markers := []string{
		"package.json",
		"next.config.js", "next.config.mjs", "next.config.ts",
		"vite.config.js", "vite.config.ts",
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			return true
		}
	}
	return false
}

func findHTMLFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var html []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".html") {
			html = append(html, filepath.Join(root, e.Name()))
		}
	}
	return html
}

func mapDeployError(err error) int {
	msg := err.Error()
	if strings.Contains(msg, "governed mode") {
		return http.StatusForbidden
	}
	if strings.Contains(msg, "exceeds") {
		return http.StatusRequestEntityTooLarge
	}
	if strings.Contains(msg, "invalid site name") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
