package warehouse

import (
	"context"
	"fmt"
	"os"

)

// Querier executes read-only SQL.
type Querier interface {
	Query(ctx context.Context, sql string, rowLimit int) ([]map[string]any, error)
	Close() error
}

func NewQuerier(cfg WarehouseConfig) (Querier, error) {
	switch cfg.WarehouseDriver() {
	case "none", "":
		return nil, nil
	case "postgres":
		creds := os.Getenv(cfg.WarehouseCredentialsEnv())
		if creds == "" {
			return nil, fmt.Errorf("warehouse credentials not set — configure %s", cfg.WarehouseCredentialsEnv())
		}
		return newPostgresQuerier(creds)
	case "bigquery":
		return newBigQueryQuerier(cfg)
	default:
		return nil, fmt.Errorf("unknown warehouse driver %q", cfg.WarehouseDriver())
	}
}
