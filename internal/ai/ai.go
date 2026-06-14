package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

type Handler struct {
	cfg    *config.Config
	db     *db.DB
	client *http.Client
}

func NewHandler(cfg *config.Config, database *db.DB) *Handler {
	return &Handler{
		cfg: cfg, db: database,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

type chatRequest struct {
	Messages []message `json:"messages"`
	Model    string    `json:"model,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type quotaLimitError struct {
	msg string
}

func (e quotaLimitError) Error() string { return e.msg }

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AI.UpstreamURL == "" {
		http.Error(w, `{"error":"AI is not configured. Set ai.upstream_url in artifact.yaml."}`, http.StatusServiceUnavailable)
		return
	}
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)

	if err := h.checkCallQuota(r, u.Email); err != nil {
		if isQuotaLimit(err) {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusTooManyRequests)
		} else {
			http.Error(w, `{"error":"quota check failed"}`, http.StatusInternalServerError)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4<<20) // 4 MiB JSON cap for long prompts
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON body."}`, http.StatusBadRequest)
		return
	}
	if len(h.cfg.AI.ModelsAllowlist) > 0 && req.Model != "" {
		allowed := false
		for _, m := range h.cfg.AI.ModelsAllowlist {
			if m == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, fmt.Sprintf(`{"error":"Model %q is not in ai.models_allowlist."}`, req.Model), http.StatusForbidden)
			return
		}
	}

	upstreamURL, err := safeUpstreamURL(h.cfg.AI.UpstreamURL, "/chat/completions")
	if err != nil {
		http.Error(w, `{"error":"Invalid ai.upstream_url in config."}`, http.StatusInternalServerError)
		return
	}
	body, _ := json.Marshal(req)
	upstream, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"Failed to create upstream request."}`, http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Authorization", "Bearer "+os.Getenv(h.cfg.AI.APIKeyEnv))
	upstream.Header.Set("x-artifact-user", u.Email)
	upstream.Header.Set("x-artifact-site", site)
	resp, err := h.client.Do(upstream)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"AI upstream unreachable: %v. Check ai.upstream_url."}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(w, "%s\n", line)
			if flusher != nil {
				flusher.Flush()
			}
		}
	} else {
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		io.Copy(w, resp.Body)
	}
	h.recordUsage(r.Context(), u.Email, site)
}

func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AI.ImageModel == "" {
		http.Error(w, `{"error":"Image generation is not configured. Set ai.image_model in artifact.yaml."}`, http.StatusServiceUnavailable)
		return
	}
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)

	if err := h.checkCallQuota(r, u.Email); err != nil {
		if isQuotaLimit(err) {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusTooManyRequests)
		} else {
			http.Error(w, `{"error":"quota check failed"}`, http.StatusInternalServerError)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON body."}`, http.StatusBadRequest)
		return
	}
	upstreamURL, err := safeUpstreamURL(h.cfg.AI.UpstreamURL, "/images/generations")
	if err != nil {
		http.Error(w, `{"error":"Invalid ai.upstream_url in config."}`, http.StatusInternalServerError)
		return
	}
	body, _ := json.Marshal(map[string]any{"model": h.cfg.AI.ImageModel, "prompt": req.Prompt, "n": 1})
	upstream, _ := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(body))
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Authorization", "Bearer "+os.Getenv(h.cfg.AI.APIKeyEnv))
	upstream.Header.Set("x-artifact-user", u.Email)
	upstream.Header.Set("x-artifact-site", site)
	resp, err := h.client.Do(upstream)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"AI upstream unreachable: %v"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
	h.recordUsage(r.Context(), u.Email, site)
}

func (h *Handler) checkCallQuota(r *http.Request, email string) error {
	max := h.cfg.Governance.Quotas.AIDailyCallsPerUser
	if max <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	total, err := h.db.CountAIUsageSince(r.Context(), email, cutoff)
	if err != nil {
		return fmt.Errorf("quota check failed: %w", err)
	}
	if total >= max {
		return quotaLimitError{msg: fmt.Sprintf("daily AI request limit (%d) reached. Try again tomorrow or ask an admin", max)}
	}
	return nil
}

func (h *Handler) recordUsage(ctx context.Context, email, site string) {
	if err := h.db.InsertAIUsage(ctx, &db.AIUsage{UserEmail: email, Site: site, Timestamp: time.Now()}); err != nil {
		slog.Default().Warn("ai usage insert failed", "err", err)
	}
}

func isQuotaLimit(err error) bool {
	var ql quotaLimitError
	return errors.As(err, &ql)
}

func safeUpstreamURL(base, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid scheme")
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String(), nil
}
