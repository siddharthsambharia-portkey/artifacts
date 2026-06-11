package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func migrate(ctx context.Context, db *sql.DB, driver string) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`)
	if err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		var version int
		fmt.Sscanf(e.Name(), "%d_", &version)
		var exists int
		db.QueryRowContext(ctx, `SELECT 1 FROM schema_migrations WHERE version=?`, version).Scan(&exists)
		if exists == 1 {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		sqlText := string(data)
		if driver == "postgres" {
			sqlText = postgresShim(sqlText)
		}
		if _, err := db.ExecContext(ctx, sqlText); err != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			return err
		}
	}
	return nil
}

func postgresShim(sql string) string {
	sql = strings.ReplaceAll(sql, "INSERT OR IGNORE", "INSERT")
	sql = strings.ReplaceAll(sql, "ON CONFLICT(site, key) DO UPDATE SET value=excluded.value",
		"ON CONFLICT(site, key) DO UPDATE SET value=EXCLUDED.value")
	sql = strings.ReplaceAll(sql, "ON CONFLICT(name) DO UPDATE SET deploy_id=excluded.deploy_id, deployed_by=excluded.deployed_by, deployed_at=excluded.deployed_at, size_bytes=excluded.size_bytes",
		"ON CONFLICT(name) DO UPDATE SET deploy_id=EXCLUDED.deploy_id, deployed_by=EXCLUDED.deployed_by, deployed_at=EXCLUDED.deployed_at, size_bytes=EXCLUDED.size_bytes")
	return sql
}
