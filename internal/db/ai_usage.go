package db

import (
	"context"
	"time"
)

type UsageSummary struct {
	UserEmail string `json:"user_email"`
	Site      string `json:"site"`
	Requests  int    `json:"requests"`
}

func (d *DB) CountAIUsageSince(ctx context.Context, email string, cutoff time.Time) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ai_usage WHERE user_email=? AND timestamp > ?`,
		email, cutoff).Scan(&n)
	return n, shimQuery(err, d.driver)
}

func (d *DB) ListAIUsageSummary(ctx context.Context, limit int) ([]UsageSummary, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT user_email, site, COUNT(*) as requests FROM ai_usage GROUP BY user_email, site ORDER BY requests DESC LIMIT ?`,
		limit)
	if err != nil {
		return nil, shimQuery(err, d.driver)
	}
	defer rows.Close()
	var out []UsageSummary
	for rows.Next() {
		var s UsageSummary
		if err := rows.Scan(&s.UserEmail, &s.Site, &s.Requests); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
