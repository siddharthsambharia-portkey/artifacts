package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

type SlackPoster interface {
	Post(ctx context.Context, webhookURL string, body []byte) error
}

type httpSlackPoster struct{}

func (p *httpSlackPoster) Post(ctx context.Context, webhookURL string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func NewHTTPPoster() SlackPoster {
	return &httpSlackPoster{}
}

type Handler struct {
	cfg    *config.Config
	db     *db.DB
	poster SlackPoster
}

func NewHandler(cfg *config.Config, database *db.DB, poster SlackPoster) *Handler {
	return &Handler{cfg: cfg, db: database, poster: poster}
}

func (h *Handler) Slack(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Notify.Slack.Mode == "off" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Channel string `json:"channel"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON body."}`, http.StatusBadRequest)
		return
	}
	if !h.channelAllowed(req.Channel) {
		http.Error(w, fmt.Sprintf(`{"error":"Channel %q is not in notify.slack.channel_allowlist. Ask an admin to add it."}`, req.Channel), http.StatusForbidden)
		return
	}
	secret := os.Getenv(h.cfg.Notify.Slack.SecretEnv)
	if secret == "" {
		http.Error(w, `{"error":"Slack secret not configured. Set the env var referenced by notify.slack.secret_env."}`, http.StatusServiceUnavailable)
		return
	}
	payload := map[string]string{"text": req.Message, "channel": req.Channel}
	body, _ := json.Marshal(payload)
	ctx := r.Context()
	if err := h.poster.Post(ctx, secret, body); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to post to Slack: %v"}`, err), http.StatusBadGateway)
		return
	}
	_ = h.db.InsertAudit(ctx, &db.AuditEntry{
		Timestamp: time.Now(), UserEmail: u.Email,
		Site: site, Action: "slack_notify", Detail: req.Channel,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (h *Handler) channelAllowed(channel string) bool {
	if len(h.cfg.Notify.Slack.ChannelAllowlist) == 0 {
		return true
	}
	channel = strings.TrimPrefix(channel, "#")
	for _, c := range h.cfg.Notify.Slack.ChannelAllowlist {
		if strings.TrimPrefix(c, "#") == channel {
			return true
		}
	}
	return false
}
