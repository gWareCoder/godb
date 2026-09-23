package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"godb/internal/api"
	"godb/internal/database"
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api") {
			log.Printf("[%s] %s - %v", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

func main() {
	port := flag.Int("port", 8080, "Port to run web server on")
	dataDir := flag.String("data", "./data", "Directory to store SQLite databases")
	flag.Parse()

	log.Printf("Starting GoDB Server on port %d...", *port)

	// Resolve absolute path for data dir
	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("Invalid data directory: %v", err)
	}

	dbMgr, err := database.NewManager(absDataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database manager: %v", err)
	}
	defer dbMgr.Close()

	// Seed sample database if default has no tables
	db, activeName := dbMgr.GetDB()
	if db != nil {
		schema, err := database.GetSchema(db)
		if err == nil && len(schema.Tables) == 0 {
			log.Printf("Initializing sample e-commerce schema in '%s'...", activeName)
			if err := database.LoadSampleEcommerceDB(db); err != nil {
				log.Printf("Warning: failed to seed sample schema: %v", err)
			} else {
				log.Printf("Sample e-commerce schema initialized successfully.")
			}
		}
	}

	mux := http.NewServeMux()

	// Register API routes
	server := api.NewServer(dbMgr)
	server.RegisterRoutes(mux)

	// Static files handler
	webDir, _ := filepath.Abs("./web")
	fs := http.FileServer(http.Dir(webDir))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(webDir, r.URL.Path)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA routes
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("GoDB Web App is ready and listening at http://localhost:%d", *port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Server stopped.")
}
