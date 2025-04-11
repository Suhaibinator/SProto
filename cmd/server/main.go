package main

import (
	"log"
	"net/http"

	"github.com/Suhaibinator/SProto/internal/api"
	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/Suhaibinator/SProto/internal/db"
	"github.com/Suhaibinator/SProto/internal/storage"
	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Database (Postgres or SQLite)
	_, err = db.Init(cfg) // Pass the whole config struct
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Storage (Minio or Local)
	_, err = storage.InitStorage(cfg) // Use the new unified storage init
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err) // Updated error message
	}

	// Initialize Router
	router := mux.NewRouter()

	// Register API routes
	api.RegisterRoutes(router, cfg.AuthToken) // Pass the router and auth token

	// Start Server
	listenAddr := ":" + cfg.ServerPort
	log.Printf("Starting server on %s", listenAddr)
	err = http.ListenAndServe(listenAddr, router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
