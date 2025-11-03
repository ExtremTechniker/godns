package db

import (
	"context"
	"fmt"

	"github.com/extremtechniker/godns/model"
	"github.com/extremtechniker/godns/util"
	"github.com/jackc/pgx/v5/pgxpool"
)

var PgPool *pgxpool.Pool

func InitPostgres(ctx context.Context) error {
	pgUrl := util.MustGetenv("PG_URL", "postgres://root:root@localhost:5432/postgres?sslmode=disable")
	pool, err := pgxpool.New(ctx, pgUrl)
	if err != nil {
		return fmt.Errorf("pgxpool.New: %w", err)
	}
	PgPool = pool
	return EnsureTables(ctx)
}

func ClosePostgres() {
	if PgPool != nil {
		PgPool.Close()
	}
}

// Creates tables if not exist
func EnsureTables(ctx context.Context) error {
	q1 := `CREATE TABLE IF NOT EXISTS dns_records (
		id SERIAL PRIMARY KEY,
		domain TEXT NOT NULL,
		qtype TEXT NOT NULL,
		ttl INT NOT NULL,
		value TEXT NOT NULL,
		UNIQUE(domain, qtype, value)
	);`
	q2 := `CREATE TABLE IF NOT EXISTS dns_metrics (
		domain TEXT NOT NULL,
		qtype TEXT NOT NULL,
		hits BIGINT NOT NULL DEFAULT 0,
		PRIMARY KEY(domain, qtype)
	);`

	if _, err := PgPool.Exec(ctx, q1); err != nil {
		return err
	}
	if _, err := PgPool.Exec(ctx, q2); err != nil {
		return err
	}
	return nil
}

func AddRecord(ctx context.Context, r model.Record) error {
	q := `INSERT INTO dns_records (domain, qtype, ttl, value) VALUES ($1,$2,$3,$4)
	ON CONFLICT (domain, qtype, value) DO UPDATE SET ttl = $3;`
	_, err := PgPool.Exec(ctx, q, r.Domain, r.QType, r.TTL, r.Value)
	return err
}

func FetchRecords(ctx context.Context, domain, qtype string) ([]model.Record, error) {
	q := `SELECT domain, qtype, ttl, value FROM dns_records WHERE domain = $1 AND qtype = $2`
	rows, err := PgPool.Query(ctx, q, domain, qtype)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Record
	for rows.Next() {
		var r model.Record
		if err := rows.Scan(&r.Domain, &r.QType, &r.TTL, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func FetchAllRecords(ctx context.Context) ([]model.Record, error) {
	q := `SELECT domain, qtype, ttl, value FROM dns_records`
	rows, err := PgPool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Record
	for rows.Next() {
		var r model.Record
		if err := rows.Scan(&r.Domain, &r.QType, &r.TTL, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
