package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/engine"
)

type API struct {
	Engine *engine.Engine
}

func NewAPI(eng *engine.Engine) *API {
	return &API{Engine: eng}
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"engine":  "lsm-kv-storage-engine",
		"version": "1.0.0",
		"moat":    "LSM-Tree with Concurrent SkipList, CRC32 WAL, Bloom Filter Sparse Index & Compactor",
	})
}

func (a *API) PutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "key and value required"})
		return
	}

	if err := a.Engine.Put([]byte(req.Key), []byte(req.Value)); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "STORED",
		"key":    req.Key,
	})
}

func (a *API) GetHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ?key= parameter"})
		return
	}

	val, found, err := a.Engine.Get([]byte(key))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "KEY_NOT_FOUND"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":   key,
		"value": string(val),
		"found": true,
	})
}

func (a *API) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ?key= parameter"})
		return
	}

	if err := a.Engine.Delete([]byte(key)); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "TOMBSTONE_WRITTEN",
		"key":    key,
	})
}

func (a *API) FlushHandler(w http.ResponseWriter, r *http.Request) {
	if err := a.Engine.Flush(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "MEMTABLE_FLUSHED_TO_LEVEL0"})
}

func (a *API) CompactHandler(w http.ResponseWriter, r *http.Request) {
	if err := a.Engine.TriggerCompaction(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "COMPACTION_EXECUTED"})
}

func (a *API) StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := a.Engine.Stats()
	respondJSON(w, http.StatusOK, stats)
}

func (a *API) SeedBatchHandler(w http.ResponseWriter, r *http.Request) {
	// Seed 200 ordered records to observe MemTable growth and auto-flushing
	count := 200
	for i := 1; i <= count; i++ {
		k := fmt.Sprintf("sensor:node_%04d:telemetry", i)
		v := fmt.Sprintf(`{"pressure_psi":%.2f,"temp_c":%.2f,"status":"NOMINAL"}`, 14.7+float64(i)*0.1, 22.4+float64(i%10))
		_ = a.Engine.Put([]byte(k), []byte(v))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "BATCH_SEEDED",
		"records_stored": count,
	})
}

func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

