package notify

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

type fakeSlackPoster struct {
	calls []struct {
		url  string
		body []byte
	}
	err error
}

func (f *fakeSlackPoster) Post(_ context.Context, webhookURL string, body []byte) error {
	f.calls = append(f.calls, struct {
		url  string
		body []byte
	}{webhookURL, body})
	return f.err
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return database
}

func postSlack(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/slack", bytes.NewReader([]byte(body)))
	req.Host = "guestbook.localhost:8443"
	req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
	w := httptest.NewRecorder()
	h.Slack(w, req)
	return w
}

func TestSlackModeOff(t *testing.T) {
	stub := &fakeSlackPoster{}
	cfg := config.DefaultDev()
	cfg.Notify.Slack.Mode = "off"
	h := NewHandler(cfg, newTestDB(t), stub)

	w := postSlack(t, h, `{"channel":"general","message":"hello"}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d: %s", w.Code, w.Body.String())
	}
	if len(stub.calls) != 0 {
		t.Fatalf("expected 0 poster calls, got %d", len(stub.calls))
	}
}

func TestSlackUnlistedChannel(t *testing.T) {
	stub := &fakeSlackPoster{}
	cfg := config.DefaultDev()
	cfg.Notify.Slack.Mode = "webhook"
	cfg.Notify.Slack.ChannelAllowlist = []string{"general"}
	h := NewHandler(cfg, newTestDB(t), stub)

	w := postSlack(t, h, `{"channel":"random","message":"hello"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d: %s", w.Code, w.Body.String())
	}
	if len(stub.calls) != 0 {
		t.Fatalf("expected 0 poster calls, got %d", len(stub.calls))
	}
}

func TestSlackValidPost(t *testing.T) {
	stub := &fakeSlackPoster{}
	cfg := config.DefaultDev()
	cfg.Notify.Slack.Mode = "webhook"
	cfg.Notify.Slack.SecretEnv = "TEST_SLACK_WEBHOOK"
	cfg.Notify.Slack.ChannelAllowlist = []string{"general"}
	t.Setenv("TEST_SLACK_WEBHOOK", "https://hooks.slack.com/test")
	h := NewHandler(cfg, newTestDB(t), stub)

	w := postSlack(t, h, `{"channel":"general","message":"hello"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if len(stub.calls) != 1 {
		t.Fatalf("expected 1 poster call, got %d", len(stub.calls))
	}
	if stub.calls[0].url != "https://hooks.slack.com/test" {
		t.Fatalf("unexpected webhook URL: %s", stub.calls[0].url)
	}
	var payload map[string]string
	if err := json.Unmarshal(stub.calls[0].body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["text"] != "hello" {
		t.Fatalf("expected message 'hello', got %q", payload["text"])
	}
}

func TestSlackAuditRowInserted(t *testing.T) {
	stub := &fakeSlackPoster{}
	database := newTestDB(t)
	cfg := config.DefaultDev()
	cfg.Notify.Slack.Mode = "webhook"
	cfg.Notify.Slack.SecretEnv = "TEST_SLACK_WEBHOOK_AUDIT"
	cfg.Notify.Slack.ChannelAllowlist = []string{"general"}
	t.Setenv("TEST_SLACK_WEBHOOK_AUDIT", "https://hooks.slack.com/test")
	h := NewHandler(cfg, database, stub)

	w := postSlack(t, h, `{"channel":"general","message":"audit-check"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}

	rows, err := database.SearchAudit(context.Background(), "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Action == "slack_notify" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected audit row with action=slack_notify")
	}
}
