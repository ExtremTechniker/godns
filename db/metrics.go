package db

import "context"

func IncrementMetric(ctx context.Context, domain, qtype string) error {
	q := `INSERT INTO dns_metrics (domain, qtype, hits) VALUES ($1,$2,1)
	ON CONFLICT (domain, qtype) DO UPDATE SET hits = dns_metrics.hits + 1`
	_, err := PgPool.Exec(ctx, q, domain, qtype)
	return err
}

func GetDomainHits(ctx context.Context, domain, qtype string) (int64, error) {
	q := `SELECT hits FROM dns_metrics WHERE domain = $1 AND qtype = $2`
	row := PgPool.QueryRow(ctx, q, domain, qtype)
	var hits int64
	err := row.Scan(&hits)
	return hits, err
}
