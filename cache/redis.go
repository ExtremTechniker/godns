package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/extremtechniker/godns/model"
	"github.com/extremtechniker/godns/util"
	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client

func InitRedis(ctx context.Context) error {
	redisDb, _ := strconv.ParseInt(util.MustGetenv("REDIS_DB", "0"), 10, 32)
	Rdb = redis.NewClient(&redis.Options{
		Addr:     util.MustGetenv("REDIS_ADDR", "localhost:6379"),
		Password: util.MustGetenv("REDIS_PASS", ""),
		DB:       int(redisDb),
	})
	if err := Rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

func CacheKey(domain, qtype string) string {
	return fmt.Sprintf("dns:record:%s:%s", strings.ToLower(domain), strings.ToUpper(qtype))
}

// CacheRecord is used by CLI and metrics logic
func CacheRecord(ctx context.Context, domain, qtype string, records []model.Record) error {
	if len(records) == 0 {
		return fmt.Errorf("no records to cache")
	}
	b, _ := json.Marshal(records)
	return Rdb.Set(ctx, CacheKey(domain, qtype), b, time.Hour).Err()
}
