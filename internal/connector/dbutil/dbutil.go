// Package dbutil provides shared database query helpers for connectors.
package dbutil

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// QueryResult is the JS-facing query payload.
type QueryResult struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int             `json:"row_count"`
}

// VersionResult holds database server version text.
type VersionResult struct {
	Version string `json:"version"`
}

// OpenPing opens a sql.DB, pings it, and returns a closer-aware handle.
func OpenPing(ctx context.Context, driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Query runs a SELECT and materializes columns/rows.
func Query(ctx context.Context, db *sql.DB, query string, args []interface{}) (*QueryResult, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := &QueryResult{
		Columns: cols,
		Rows:    make([][]interface{}, 0),
	}
	for rows.Next() {
		raw := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]interface{}, len(cols))
		for i, v := range raw {
			row[i] = normalizeValue(v)
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.RowCount = len(out.Rows)
	return out, nil
}

// QueryVersion runs a version SQL and returns the first cell as string.
func QueryVersion(ctx context.Context, db *sql.DB, query string) (*VersionResult, error) {
	var ver string
	if err := db.QueryRowContext(ctx, query).Scan(&ver); err != nil {
		return nil, fmt.Errorf("version query: %w", err)
	}
	return &VersionResult{Version: ver}, nil
}

func normalizeValue(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return t
	}
}
