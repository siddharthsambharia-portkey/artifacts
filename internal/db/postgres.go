package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func openPostgres(url string) (*DB, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open postgres database: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	return &DB{DB: sqlDB, driver: "postgres"}, nil
}
