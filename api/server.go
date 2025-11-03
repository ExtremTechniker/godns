package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/extremtechniker/godns/cache"
	"github.com/extremtechniker/godns/db"
	"github.com/extremtechniker/godns/logger"
	"github.com/extremtechniker/godns/model"
	"github.com/extremtechniker/godns/util"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type Server struct {
	Addr string
	Ctx  context.Context
}

var jwtSecret = []byte(util.GetJwtSecret()) // from env in production

func NewServer(addr string, ctx context.Context) *Server {
	return &Server{Addr: addr, Ctx: ctx}
}

func (s *Server) Run() error {
	r := mux.NewRouter()

	// Middleware applied to all routes
	r.Use(s.jwtMiddleware)

	// Record CRUD
	r.HandleFunc("/records", s.CreateRecord).Methods("POST")
	r.HandleFunc("/records", s.ListRecords).Methods("GET")
	r.HandleFunc("/records/{domain}/{qtype}", s.UpdateRecordTTL).Methods("PUT")
	r.HandleFunc("/records/{domain}/{qtype}", s.DeleteRecord).Methods("DELETE")

	// Cache management
	r.HandleFunc("/cache/{domain}/{qtype}", s.AddToCache).Methods("POST")
	r.HandleFunc("/cache/{domain}/{qtype}", s.RemoveFromCache).Methods("DELETE")

	logger.Logger.Infof("HTTP API listening on %s", s.Addr)
	return http.ListenAndServe(s.Addr, r)
}

// ---------------- JWT Middleware ----------------
func (s *Server) jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if !strings.HasPrefix(tokenStr, "Bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if token.Valid {
			next.ServeHTTP(w, r)
			return
		}
		logger.Logger.Debugf("Invalid token: XXX %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
	})
}

func (s *Server) CreateRecord(w http.ResponseWriter, r *http.Request) {
	var rec model.Record
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// If no ttl is specified use 300
	if rec.TTL == 0 {
		rec.TTL = 300
	}

	if err := db.AddRecord(s.Ctx, rec); err != nil {
		http.Error(w, "failed to add record", http.StatusInternalServerError)
		return
	}

	// Update cache only if exists
	key := cache.CacheKey(rec.Domain, rec.QType)
	if exists, _ := cache.Rdb.Exists(s.Ctx, key).Result(); exists > 0 {
		if err := cache.CacheRecord(s.Ctx, rec.Domain, rec.QType, []model.Record{rec}); err != nil {
			logger.Logger.Errorf("failed to update cache: %v", err)
		}
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) ListRecords(w http.ResponseWriter, r *http.Request) {
	records, err := db.FetchAllRecords(s.Ctx) // fetch all
	if err != nil {
		http.Error(w, "failed to fetch records", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(records)
}

func (s *Server) UpdateRecordTTL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	qtype := vars["qtype"]

	var input struct {
		TTL int `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	records, err := db.FetchRecords(s.Ctx, domain, qtype)
	if err != nil || len(records) == 0 {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	for _, rec := range records {
		rec.TTL = input.TTL
		if err := db.AddRecord(s.Ctx, rec); err != nil {
			http.Error(w, "failed to update record", http.StatusInternalServerError)
			return
		}
	}

	key := cache.CacheKey(domain, qtype)
	if exists, _ := cache.Rdb.Exists(s.Ctx, key).Result(); exists > 0 {
		if err := cache.CacheRecord(s.Ctx, domain, qtype, records); err != nil {
			logger.Logger.Errorf("failed to update cache: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	qtype := vars["qtype"]

	_, err := db.PgPool.Exec(s.Ctx, "DELETE FROM dns_records WHERE domain=$1 AND qtype=$2", domain, qtype)
	if err != nil {
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}

	cache.Rdb.Del(s.Ctx, cache.CacheKey(domain, qtype))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) AddToCache(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	qtype := vars["qtype"]

	recs, err := db.FetchRecords(s.Ctx, domain, qtype)
	if err != nil || len(recs) == 0 {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	if err := cache.CacheRecord(s.Ctx, domain, qtype, recs); err != nil {
		http.Error(w, "failed to cache", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) RemoveFromCache(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	qtype := vars["qtype"]

	cache.Rdb.Del(s.Ctx, cache.CacheKey(domain, qtype))
	w.WriteHeader(http.StatusOK)
}

func StartServer(ctx context.Context) error {
	srv := NewServer(util.MustGetenv("HTTP_SERVE", ":8080"), ctx)
	return srv.Run()
}
