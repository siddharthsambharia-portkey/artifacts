package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type DB struct {
	*sql.DB
	driver string
}

type Document struct {
	ID         string          `json:"id"`
	Site       string          `json:"site"`
	Collection string          `json:"collection"`
	Data       json.RawMessage `json:"data"`
	CreatedBy  string          `json:"created_by"`
	UpdatedBy  string          `json:"updated_by"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type KVEntry struct {
	Site  string `json:"site"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SiteRecord struct {
	Name             string    `json:"name"`
	Owner            string    `json:"owner,omitempty"`
	DeployID         string    `json:"deploy_id"`
	DeployedBy       string    `json:"deployed_by"`
	DeployedAt       time.Time `json:"deployed_at"`
	SizeBytes        int64     `json:"size_bytes"`
	Visibility       string    `json:"visibility"`
	VisibilityGroups []string  `json:"visibility_groups"`
}

type AuditEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	UserEmail string    `json:"user_email"`
	Site      string    `json:"site"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
}

type AIUsage struct {
	UserEmail string    `json:"user_email"`
	Site      string    `json:"site"`
	Timestamp time.Time `json:"timestamp"`
}

func Open(cfg *config.Config) (*DB, error) {
	switch cfg.Database.Driver {
	case "sqlite":
		url := cfg.Database.URL
		if url == "" {
			url = cfg.DataDir + "/artifact.db"
		}
		return openSQLite(url)
	case "postgres":
		url := cfg.Database.URL
		if url == "" && cfg.Database.URLEnv != "" {
			url = os.Getenv(cfg.Database.URLEnv)
		}
		if url == "" {
			return nil, fmt.Errorf("postgres driver requires database.url or %s env var", cfg.Database.URLEnv)
		}
		return openPostgres(url)
	default:
		return nil, fmt.Errorf("unknown database driver %q: use sqlite or postgres", cfg.Database.Driver)
	}
}

func (d *DB) Driver() string { return d.driver }

func (d *DB) Migrate(ctx context.Context) error {
	return migrate(ctx, d.DB, d.driver)
}

func (d *DB) CreateDocument(ctx context.Context, doc *Document) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO documents (id, site, collection, data, created_by, updated_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Site, doc.Collection, doc.Data, doc.CreatedBy, doc.UpdatedBy, doc.CreatedAt, doc.UpdatedAt)
	return shimExec(err, d.driver)
}

func (d *DB) UpdateDocument(ctx context.Context, doc *Document) error {
	_, err := d.ExecContext(ctx,
		`UPDATE documents SET data=?, updated_by=?, updated_at=? WHERE id=? AND site=?`,
		doc.Data, doc.UpdatedBy, doc.UpdatedAt, doc.ID, doc.Site)
	return shimExec(err, d.driver)
}

func (d *DB) DeleteDocument(ctx context.Context, site, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM documents WHERE id=? AND site=?`, id, site)
	return shimExec(err, d.driver)
}

func (d *DB) GetDocument(ctx context.Context, site, id string) (*Document, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, site, collection, data, created_by, updated_by, created_at, updated_at FROM documents WHERE id=? AND site=?`,
		id, site)
	return scanDocument(row)
}

func (d *DB) ListDocuments(ctx context.Context, site, collection string, limit int, orderDesc bool) ([]Document, error) {
	order := "created_at ASC"
	if orderDesc {
		order = "created_at DESC"
	}
	q := fmt.Sprintf(`SELECT id, site, collection, data, created_by, updated_by, created_at, updated_at
		FROM documents WHERE site=? AND collection=? ORDER BY %s LIMIT ?`, order)
	rows, err := d.QueryContext(ctx, q, site, collection, limit)
	if err != nil {
		return nil, shimQuery(err, d.driver)
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Site, &doc.Collection, &doc.Data, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (d *DB) CountDocuments(ctx context.Context, site string) (int, error) {
	var n int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE site=?`, site).Scan(&n)
	return n, shimQuery(err, d.driver)
}

func (d *DB) SetKV(ctx context.Context, site, key, value string) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO kv (site, key, value) VALUES (?, ?, ?) ON CONFLICT(site, key) DO UPDATE SET value=excluded.value`,
		site, key, value)
	return shimExec(err, d.driver)
}

func (d *DB) GetKV(ctx context.Context, site, key string) (string, error) {
	var v string
	err := d.QueryRowContext(ctx, `SELECT value FROM kv WHERE site=? AND key=?`, site, key).Scan(&v)
	return v, shimQuery(err, d.driver)
}

func (d *DB) UpsertSite(ctx context.Context, s *SiteRecord) error {
	groupsJSON := marshalVisibilityGroups(s.VisibilityGroups)
	_, err := d.ExecContext(ctx,
		`INSERT INTO sites (name, owner, deploy_id, deployed_by, deployed_at, size_bytes, visibility, visibility_groups)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET deploy_id=excluded.deploy_id, deployed_by=excluded.deployed_by,
		 deployed_at=excluded.deployed_at, size_bytes=excluded.size_bytes`,
		s.Name, s.Owner, s.DeployID, s.DeployedBy, s.DeployedAt, s.SizeBytes, s.Visibility, groupsJSON)
	return shimExec(err, d.driver)
}

func (d *DB) UpdateSiteVisibility(ctx context.Context, name, visibility string, groups []string) (bool, error) {
	groupsJSON := "[]"
	if visibility == "group" {
		groupsJSON = marshalVisibilityGroups(groups)
	}
	res, err := d.ExecContext(ctx,
		`UPDATE sites SET visibility=?, visibility_groups=? WHERE name=?`,
		visibility, groupsJSON, name)
	if err != nil {
		return false, shimExec(err, d.driver)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *DB) GetSite(ctx context.Context, name string) (*SiteRecord, error) {
	row := d.QueryRowContext(ctx,
		`SELECT name, owner, deploy_id, deployed_by, deployed_at, size_bytes, visibility, visibility_groups FROM sites WHERE name=?`, name)
	s, err := scanSite(row)
	if err != nil {
		return nil, shimQuery(err, d.driver)
	}
	return s, nil
}

func (d *DB) ListSites(ctx context.Context) ([]SiteRecord, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT name, owner, deploy_id, deployed_by, deployed_at, size_bytes, visibility, visibility_groups FROM sites ORDER BY deployed_at DESC`)
	if err != nil {
		return nil, shimQuery(err, d.driver)
	}
	defer rows.Close()
	var sites []SiteRecord
	for rows.Next() {
		s, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, *s)
	}
	return sites, nil
}

func (d *DB) InsertAudit(ctx context.Context, e *AuditEntry) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO audit_log (timestamp, user_email, site, action, detail) VALUES (?, ?, ?, ?, ?)`,
		e.Timestamp, e.UserEmail, e.Site, e.Action, e.Detail)
	return shimExec(err, d.driver)
}

func (d *DB) SearchAudit(ctx context.Context, site, user string, limit int) ([]AuditEntry, error) {
	q := `SELECT id, timestamp, user_email, site, action, detail FROM audit_log WHERE 1=1`
	args := []any{}
	if site != "" {
		q += ` AND site=?`
		args = append(args, site)
	}
	if user != "" {
		q += ` AND user_email=?`
		args = append(args, user)
	}
	q += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, shimQuery(err, d.driver)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.UserEmail, &e.Site, &e.Action, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (d *DB) InsertAIUsage(ctx context.Context, u *AIUsage) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO ai_usage (user_email, site, timestamp) VALUES (?, ?, ?)`,
		u.UserEmail, u.Site, u.Timestamp)
	return shimExec(err, d.driver)
}

func scanDocument(row *sql.Row) (*Document, error) {
	var doc Document
	err := row.Scan(&doc.ID, &doc.Site, &doc.Collection, &doc.Data, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &doc, err
}

func marshalVisibilityGroups(groups []string) string {
	if groups == nil {
		groups = []string{}
	}
	b, err := json.Marshal(groups)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalVisibilityGroups(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var groups []string
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		return []string{}
	}
	return groups
}

type scannable interface {
	Scan(dest ...any) error
}

func scanSite(row scannable) (*SiteRecord, error) {
	var s SiteRecord
	var groupsJSON string
	err := row.Scan(&s.Name, &s.Owner, &s.DeployID, &s.DeployedBy, &s.DeployedAt, &s.SizeBytes, &s.Visibility, &groupsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.VisibilityGroups = unmarshalVisibilityGroups(groupsJSON)
	return &s, nil
}
