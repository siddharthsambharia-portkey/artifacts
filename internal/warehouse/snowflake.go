package warehouse

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type snowflakeQuerier struct {
	inner Querier
}

func newSnowflakeQuerier(cfg *config.Config) (*snowflakeQuerier, error) {
	dsn := os.Getenv(cfg.Warehouse.CredentialsEnv)
	if dsn == "" {
		return nil, fmt.Errorf("snowflake DSN not set — configure %s", cfg.Warehouse.CredentialsEnv)
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		inner, err := newPostgresQuerier(dsn)
		if err != nil {
			return nil, err
		}
		return &snowflakeQuerier{inner: inner}, nil
	}
	return nil, fmt.Errorf("snowflake: use a postgres-compatible DSN in %s, or set warehouse.driver: postgres", cfg.Warehouse.CredentialsEnv)
}

func (s *snowflakeQuerier) Query(ctx context.Context, sqlText string, rowLimit int) ([]map[string]any, error) {
	return s.inner.Query(ctx, sqlText, rowLimit)
}

func (s *snowflakeQuerier) Close() error {
	if s.inner != nil {
		return s.inner.Close()
	}
	return nil
}
