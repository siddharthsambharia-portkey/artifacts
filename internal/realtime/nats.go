package realtime

import (
	"log/slog"
	"os"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/nats-io/nats.go"
)

// ConnectNATS connects to NATS if ARTIFACT_NATS_URL or NATS_URL is set.
func ConnectNATS(cfg *config.Config, hub *Hub, logger *slog.Logger) (*nats.Conn, error) {
	url := os.Getenv("ARTIFACT_NATS_URL")
	if url == "" {
		url = os.Getenv("NATS_URL")
	}
	if url == "" {
		return nil, nil
	}
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	hub.SetNATS(nc)
	logger.Info("NATS pubsub enabled", "url", url)
	return nc, nil
}
