package warehouse

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresQuerier struct {
	db *sql.DB
}

func newPostgresQuerier(url string) (*postgresQuerier, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	return &postgresQuerier{db: db}, nil
}

func (p *postgresQuerier) Query(ctx context.Context, sqlText string, rowLimit int) ([]map[string]any, error) {
	if !strings.Contains(strings.ToUpper(sqlText), "LIMIT") {
		sqlText = fmt.Sprintf("%s LIMIT %d", sqlText, rowLimit)
	}
	rows, err := p.db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows, rowLimit)
}

func (p *postgresQuerier) Close() error { return p.db.Close() }

func scanRows(rows *sql.Rows, rowLimit int) ([]map[string]any, error) {
	cols, _ := rows.Columns()
	var result []map[string]any
	for rows.Next() {
		if len(result) >= rowLimit {
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any)
		for i, c := range cols {
			row[c] = vals[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
