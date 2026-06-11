package warehouse

import (
	"context"
	"fmt"
	"os"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

// Querier executes read-only SQL.
type Querier interface {
	Query(ctx context.Context, sql string, rowLimit int) ([]map[string]any, error)
	Close() error
}

func NewQuerier(cfg *config.Config) (Querier, error) {
	switch cfg.Warehouse.Driver {
	case "none", "":
		return nil, nil
	case "postgres":
		creds := os.Getenv(cfg.Warehouse.CredentialsEnv)
		if creds == "" {
			return nil, fmt.Errorf("warehouse credentials not set — configure %s", cfg.Warehouse.CredentialsEnv)
		}
		return newPostgresQuerier(creds)
	case "bigquery":
		return newBigQueryQuerier(cfg)
	case "snowflake":
		return newSnowflakeQuerier(cfg)
	default:
		return nil, fmt.Errorf("unknown warehouse driver %q", cfg.Warehouse.Driver)
	}
}
