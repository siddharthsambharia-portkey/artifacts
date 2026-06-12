package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

func testHandler(t *testing.T, dailyLimit int) (*Handler, *db.DB) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	cfg.Governance.Quotas.AIDailyCallsPerUser = dailyLimit
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	return NewHandler(cfg, database), database
}

func quotaReq(t *testing.T, email string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/chat", nil)
	return req.WithContext(context.Background())
}

func TestCheckCallQuota(t *testing.T) {
	const email = "alice@co"

	t.Run("under limit passes", func(t *testing.T) {
		h, database := testHandler(t, 2)
		if err := database.InsertAIUsage(context.Background(), &db.AIUsage{
			UserEmail: email, Site: "site", Timestamp: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		if err := h.checkCallQuota(quotaReq(t, email), email); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("at limit returns limit error", func(t *testing.T) {
		h, database := testHandler(t, 2)
		for i := 0; i < 2; i++ {
			if err := database.InsertAIUsage(context.Background(), &db.AIUsage{
				UserEmail: email, Site: "site", Timestamp: time.Now(),
			}); err != nil {
				t.Fatal(err)
			}
		}
		err := h.checkCallQuota(quotaReq(t, email), email)
		if err == nil {
			t.Fatal("expected limit error")
		}
		if !isQuotaLimit(err) {
			t.Fatalf("expected quota limit error, got %v", err)
		}
	})

	t.Run("rolling window excludes old rows", func(t *testing.T) {
		h, database := testHandler(t, 2)
		for i := 0; i < 5; i++ {
			if err := database.InsertAIUsage(context.Background(), &db.AIUsage{
				UserEmail: email, Site: "site", Timestamp: time.Now().Add(-25 * time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.checkCallQuota(quotaReq(t, email), email); err != nil {
			t.Fatalf("expected pass for old rows only, got %v", err)
		}
	})

	t.Run("disabled quota always passes", func(t *testing.T) {
		h, database := testHandler(t, 0)
		for i := 0; i < 10; i++ {
			if err := database.InsertAIUsage(context.Background(), &db.AIUsage{
				UserEmail: email, Site: "site", Timestamp: time.Now(),
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.checkCallQuota(quotaReq(t, email), email); err != nil {
			t.Fatalf("expected pass when quota disabled, got %v", err)
		}
	})
}
