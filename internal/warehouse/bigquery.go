package warehouse

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

type bigQueryQuerier struct {
	client *bigquery.Client
}

func newBigQueryQuerier(_ WarehouseConfig) (*bigQueryQuerier, error) {
	ctx := context.Background()
	project := os.Getenv("ARTIFACT_BIGQUERY_PROJECT")
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("bigquery client: %w — set GOOGLE_APPLICATION_CREDENTIALS", err)
	}
	return &bigQueryQuerier{client: client}, nil
}

func (b *bigQueryQuerier) Query(ctx context.Context, sqlText string, rowLimit int) ([]map[string]any, error) {
	if !strings.Contains(strings.ToUpper(sqlText), "LIMIT") {
		sqlText = fmt.Sprintf("%s LIMIT %d", sqlText, rowLimit)
	}
	q := b.client.Query(sqlText)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}
	var result []map[string]any
	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(result) >= rowLimit {
			break
		}
		m := make(map[string]any)
		for i, s := range it.Schema {
			if i < len(row) {
				m[s.Name] = row[i]
			}
		}
		result = append(result, m)
	}
	return result, nil
}

func (b *bigQueryQuerier) Close() error { return b.client.Close() }
