// main.bak.go
// Go DNS server with Postgres persistence, Redis caching (per-record), and a CLI.
// Added structured logging with zap and updated Postgres defaults to root/root.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Record struct {
	Domain string `json:"domain"`
	QType  string `json:"qtype"`
	TTL    int    `json:"ttl"`
	Value  string `json:"value"`
}

var (
	pgPool   *pgxpool.Pool
	rdb      *redis.Client
	ctx      = context.Background()
	logger   *zap.SugaredLogger
	logLevel string
)

func main() {
	root := &cobra.Command{
		Use:   "godns",
		Short: "Go DNS server with Postgres and Redis",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initLogger(logLevel)
		},
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", mustGetenv("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")

	root.AddCommand(cmdDaemon())
	root.AddCommand(cmdAddRecord())
	root.AddCommand(cmdCacheRecord())

	if err := root.Execute(); err != nil {
		logger.Fatalf("error: %v", err)
	}
}

func initLogger(level string) {
	var cfg zap.Config
	if os.Getenv("LOG_FORMAT") == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	switch strings.ToLower(level) {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn", "warning":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	l, err := cfg.Build()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	logger = l.Sugar()
	logger.Infof("Logger initialized with level: %s", level)
}

func mustGetenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func initDeps() error {
	pgUrl := mustGetenv("PG_URL", "postgres://root:root@localhost:5432/postgres?sslmode=disable")
	pool, err := pgxpool.New(ctx, pgUrl)
	if err != nil {
		return fmt.Errorf("pgxpool.New: %w", err)
	}
	pgPool = pool

	rdb = redis.NewClient(&redis.Options{
		Addr:     mustGetenv("REDIS_ADDR", "localhost:6379"),
		Password: mustGetenv("REDIS_PASS", ""),
		DB:       0,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		if strings.Contains(err.Error(), "maint_notifications") {
			logger.Warn("Redis maint_notifications unsupported, continuing.")
		} else {
			return fmt.Errorf("redis ping: %w", err)
		}
	}

	if err := ensureTables(ctx); err != nil {
		return err
	}

	return nil
}

func ensureTables(ctx context.Context) error {
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

	if _, err := pgPool.Exec(ctx, q1); err != nil {
		return err
	}
	if _, err := pgPool.Exec(ctx, q2); err != nil {
		return err
	}
	return nil
}

func cmdDaemon() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run DNS server daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDeps(); err != nil {
				return err
			}
			listen := mustGetenv("DNS_LISTEN", ":1053")
			return runDaemon(listen)
		},
	}
	return cmd
}

func cmdAddRecord() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-record <domain> <type> <value> [ttl]",
		Short: "Add a DNS record to Postgres",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDeps(); err != nil {
				return err
			}
			domain := args[0]
			qtype := strings.ToUpper(args[1])
			value := args[2]
			ttl := 300
			if len(args) == 4 {
				fmt.Sscanf(args[3], "%d", &ttl)
			}
			rec := Record{Domain: domain, QType: qtype, TTL: ttl, Value: value}
			if err := addRecord(ctx, rec); err != nil {
				return err
			}
			logger.Infof("Record added: %s %s %s", domain, qtype, value)
			return nil
		},
	}
	return cmd
}

func cmdCacheRecord() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache-record <domain> <type>",
		Short: "Pre-warm Redis cache for a specific record (domain + qtype)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDeps(); err != nil {
				return err
			}
			domain := args[0]
			qtype := strings.ToUpper(args[1])
			if err := cacheRecord(ctx, domain, qtype); err != nil {
				return err
			}
			logger.Infof("Cached record: %s %s", domain, qtype)
			return nil
		},
	}
	return cmd
}

func addRecord(ctx context.Context, r Record) error {
	q := `INSERT INTO dns_records (domain, qtype, ttl, value) VALUES ($1,$2,$3,$4)
	ON CONFLICT (domain, qtype, value) DO UPDATE SET ttl = EXCLUDED.ttl`
	_, err := pgPool.Exec(ctx, q, r.Domain, r.QType, r.TTL, r.Value)
	return err
}

func fetchRecordsFromDB(ctx context.Context, domain, qtype string) ([]Record, error) {
	q := `SELECT domain, qtype, ttl, value FROM dns_records WHERE domain = $1 AND qtype = $2`
	rows, err := pgPool.Query(ctx, q, domain, qtype)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Domain, &r.QType, &r.TTL, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func cacheRecord(ctx context.Context, domain, qtype string) error {
	recs, err := fetchRecordsFromDB(ctx, domain, qtype)
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		return fmt.Errorf("no %s records for domain %s", qtype, domain)
	}
	b, _ := json.Marshal(recs)
	key := cacheKey(domain, qtype)
	if err := rdb.Set(ctx, key, b, time.Hour).Err(); err != nil {
		return err
	}
	return nil
}

func cacheKey(domain, qtype string) string {
	return fmt.Sprintf("dns:record:%s:%s", strings.ToLower(domain), strings.ToUpper(qtype))
}

func runDaemon(listen string) error {
	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: listen, Net: "udp"}
	serverTCP := &dns.Server{Addr: listen, Net: "tcp"}

	go func() {
		logger.Infof("DNS server listening on %s (udp)", listen)
		if err := server.ListenAndServe(); err != nil {
			logger.Fatalf("failed to start udp server: %v", err)
		}
	}()
	go func() {
		logger.Infof("DNS server listening on %s (tcp)", listen)
		if err := serverTCP.ListenAndServe(); err != nil {
			logger.Fatalf("failed to start tcp server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutting down")
	_ = server.Shutdown()
	_ = serverTCP.Shutdown()
	return nil
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(m)
		return
	}
	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")
	qtype := dns.TypeToString[q.Qtype]

	key := cacheKey(domain, qtype)
	var recs []Record
	if s, err := rdb.Get(ctx, key).Result(); err == nil {
		if err := json.Unmarshal([]byte(s), &recs); err == nil {
			logger.Debugf("cache hit: %s %s", domain, qtype)
			respondWithRecords(w, r, recs, q)
			go updateMetricServedFromCache(domain, qtype)

			return
		}
	}

	recs, err := fetchRecordsFromDB(ctx, domain, qtype)
	if err != nil {
		logger.Errorf("db fetch error: %v", err)
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}
	if len(recs) == 0 {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeNameError)
		_ = w.WriteMsg(m)
		logger.Debugf("no %s records for domain %s", qtype, domain)
		return
	}

	logger.Debugf("serving record from db: %s %s", qtype, domain)
	respondWithRecords(w, r, recs, q)

	go updateMetricServedNotFromCache(domain, qtype)
}

func updateMetricServedFromCache(domain, qtype string) {
	updateMetric(domain, qtype, true)
}

func updateMetricServedNotFromCache(domain, qtype string) {
	updateMetric(domain, qtype, false)
}

func updateMetric(domain, qtype string, servedFromCache bool) {
	logger.Debugf("incrementing hits for record from db: %s %s", qtype, domain)
	_ = incrementMetric(ctx, domain, qtype)
	hits, err := getDomainHits(ctx, domain, qtype)
	if err != nil {
		logger.Errorf("db fetch error: %v", err)
		return
	}
	minHits, _ := strconv.ParseInt(mustGetenv("MIN_HITS_FOR_CACHE", "5"), 10, 64)
	if hits < minHits {
		logger.Debugf("min hits for cache not reached: %d hits", hits)
		return
	}
	if servedFromCache {
		logger.Debugf("%s %s already in cache, skipping insertion", qtype, domain)
		return
	}
	logger.Debugf("insering %s %s into cache with hits of %d", domain, qtype, hits)
	recs, err := fetchRecordsFromDB(ctx, domain, qtype)
	if err != nil {
		return
	}
	if len(recs) == 0 {
		return
	}
	key := cacheKey(domain, qtype)
	b, _ := json.Marshal(recs)

	_ = rdb.Set(ctx, key, b, time.Hour).Err()
}

func respondWithRecords(w dns.ResponseWriter, req *dns.Msg, recs []Record, q dns.Question) {
	m := new(dns.Msg)
	m.SetReply(req)
	for _, r := range recs {
		if strings.EqualFold(r.QType, dns.TypeToString[q.Qtype]) || q.Qtype == dns.TypeANY {
			switch strings.ToUpper(r.QType) {
			case "A":
				rr := &dns.A{Hdr: dns.RR_Header{Name: dns.Fqdn(r.Domain), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(r.TTL)}}
				rr.A = net.ParseIP(r.Value).To4()
				if rr.A != nil {
					m.Answer = append(m.Answer, rr)
				}
			case "AAAA":
				rr := &dns.AAAA{Hdr: dns.RR_Header{Name: dns.Fqdn(r.Domain), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(r.TTL)}}
				rr.AAAA = net.ParseIP(r.Value)
				if rr.AAAA != nil {
					m.Answer = append(m.Answer, rr)
				}
			case "CNAME":
				rr := &dns.CNAME{Hdr: dns.RR_Header{Name: dns.Fqdn(r.Domain), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: uint32(r.TTL)}, Target: dns.Fqdn(r.Value)}
				m.Answer = append(m.Answer, rr)
			case "TXT":
				rr := &dns.TXT{Hdr: dns.RR_Header{Name: dns.Fqdn(r.Domain), Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: uint32(r.TTL)}, Txt: []string{r.Value}}
				m.Answer = append(m.Answer, rr)
			}
		}
	}
	_ = w.WriteMsg(m)
}

func getDomainHits(ctx context.Context, domain, qtype string) (int64, error) {
	q := `SELECT hits FROM dns_metrics WHERE domain = $1 AND qtype = $2`
	row := pgPool.QueryRow(ctx, q, domain, qtype)
	var hits int64
	err := row.Scan(&hits)
	if err != nil {
		return 0, err
	}
	return hits, nil
}

func incrementMetric(ctx context.Context, domain, qtype string) error {
	q := `INSERT INTO dns_metrics (domain, qtype, hits) VALUES ($1,$2,1)
	ON CONFLICT (domain, qtype) DO UPDATE SET hits = dns_metrics.hits + 1`
	_, err := pgPool.Exec(ctx, q, domain, qtype)
	return err
}

func closeDeps() {
	if pgPool != nil {
		pgPool.Close()
	}
	if rdb != nil {
		_ = rdb.Close()
	}
}
