package dns

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/extremtechniker/godns/cache"
	"github.com/extremtechniker/godns/db"
	"github.com/extremtechniker/godns/logger"
	"github.com/extremtechniker/godns/model"
	"github.com/extremtechniker/godns/util"
	"github.com/miekg/dns"
)

// Ctx is the global context used by the handler
var Ctx context.Context

func HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")
	qtype := dns.TypeToString[q.Qtype]

	// 1️⃣ Try Redis cache first
	var recs []model.Record
	if s, err := cache.Rdb.Get(Ctx, cache.CacheKey(domain, qtype)).Result(); err == nil {
		if err := json.Unmarshal([]byte(s), &recs); err == nil {
			logger.Logger.Debugf("cache hit: %s %s", domain, qtype)
			RespondWithRecords(w, r, recs, q)
			go updateMetricServedFromCache(domain, qtype)
			return
		}
	}

	// 2️⃣ Fetch from Postgres if not in cache
	recs, err := db.FetchRecords(Ctx, domain, qtype)
	if err != nil {
		logger.Logger.Errorf("db fetch error: %v", err)
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}

	if len(recs) == 0 {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeNameError)
		_ = w.WriteMsg(m)
		logger.Logger.Debugf("no %s records for domain %s", qtype, domain)
		return
	}

	// 3️⃣ Serve the records
	logger.Logger.Debugf("serving record from db: %s %s", qtype, domain)
	RespondWithRecords(w, r, recs, q)

	// 4️⃣ Update metrics and optionally populate Redis
	go updateMetricServedNotFromCache(domain, qtype)
}

// ---------------- Metric helpers ----------------

func updateMetricServedFromCache(domain, qtype string) {
	updateMetric(domain, qtype, true)
}

func updateMetricServedNotFromCache(domain, qtype string) {
	updateMetric(domain, qtype, false)
}

func updateMetric(domain, qtype string, servedFromCache bool) {
	logger.Logger.Debugf("incrementing hits for record: %s %s", qtype, domain)

	_ = db.IncrementMetric(Ctx, domain, qtype)
	hits, err := db.GetDomainHits(Ctx, domain, qtype)
	if err != nil {
		logger.Logger.Errorf("db fetch error: %v", err)
		return
	}

	minHits, _ := strconv.ParseInt(util.MustGetenv("MIN_HITS_FOR_CACHE", "5"), 10, 64)
	if hits < minHits {
		logger.Logger.Debugf("min hits for cache not reached: %d hits", hits)
		return
	}

	if servedFromCache {
		logger.Logger.Debugf("%s %s already in cache, skipping insertion: %d hits", qtype, domain, hits)
		return
	}

	logger.Logger.Debugf("iserting %s %s into cache: %d hits", qtype, domain, hits)

	// Fetch again from DB before caching
	recs, err := db.FetchRecords(Ctx, domain, qtype)
	if err != nil || len(recs) == 0 {
		return
	}

	if err := cache.CacheRecord(Ctx, domain, qtype, recs); err != nil {
		logger.Logger.Errorf("failed to cache record: %v", err)
	}
}
