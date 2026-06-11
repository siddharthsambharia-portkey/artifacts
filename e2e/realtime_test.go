package e2e

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/realtime"
	"github.com/siddharthsambharia-portkey/artifacts/internal/server"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
	"github.com/coder/websocket"
	"log/slog"
	"os"
)

func TestDBSubscribeEvent(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Listen = ":19556"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "storage")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv, err := server.New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws://live-poll.localhost:19556/ws?room=votes", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	received := make(chan bool, 1)
	go func() {
		for {
			_, data, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var msg realtime.DBEvent
			if json.Unmarshal(data, &msg) == nil && msg.Type == "create" && msg.Collection == "votes" {
				received <- true
				return
			}
		}
	}()

	database := srv.DB()
	doc := &db.Document{
		ID: "test1", Site: "live-poll", Collection: "votes",
		Data: json.RawMessage(`{"option":"Tacos"}`),
		CreatedBy: auth.DevUser.Email, UpdatedBy: auth.DevUser.Email,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	database.CreateDocument(context.Background(), doc)
	srv.Hub().PublishDocumentEvent("live-poll", "votes", "create", doc)

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive DB create event over websocket")
	}
}

func TestGovernedModeDeployDenied(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Governance.Mode = "governed"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "storage")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	cache := sites.NewDeployCache(store, 64)
	deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)

	other := &auth.User{Email: "other@corp.com", Name: "Other", Groups: []string{"employees"}}
	exampleDir := filepath.Join("..", "examples", "guestbook")
	_, err = deployer.Deploy(context.Background(), "owned", exampleDir, auth.DevUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = deployer.Deploy(context.Background(), "owned", exampleDir, other)
	if err == nil {
		t.Fatal("expected governed mode to deny redeploy by non-owner")
	}
}
