package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/engine"
	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	eng, err := engine.Open(engine.Config{
		DataDir:           dataDir,
		MemTableThreshold: 64 * 1024, // 64KB threshold for demonstration
		MaxLevel0SSTables: 3,
	})
	if err != nil {
		log.Fatalf("Failed to initialize LSM-KV engine: %v", err)
	}
	defer eng.Close()

	api := handlers.NewAPI(eng)
	mux := http.NewServeMux()

	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/api/v1/put", api.PutHandler)
	mux.HandleFunc("/api/v1/get", api.GetHandler)
	mux.HandleFunc("/api/v1/delete", api.DeleteHandler)
	mux.HandleFunc("/api/v1/flush", api.FlushHandler)
	mux.HandleFunc("/api/v1/compact", api.CompactHandler)
	mux.HandleFunc("/api/v1/stats", api.StatsHandler)
	mux.HandleFunc("/api/v1/seed", api.SeedBatchHandler)

	handler := handlers.EnableCORS(mux)

	serverAddr := ":" + port
	fmt.Printf("⚡ LSM-KV Storage Engine running on http://localhost%s (DataDir: %s)\n", serverAddr, dataDir)
	if err := http.ListenAndServe(serverAddr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

