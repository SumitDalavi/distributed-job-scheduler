package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SumitDalavi/distributed-job-scheduler/config"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/api"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/scheduler"
	"github.com/gorilla/mux"
)

var quit = make(chan os.Signal, 1)

func run() int {
	cfg := config.Load()
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Printf("cannot connect to database: %v", err)
		return 1
	}
	return runWithDB(database, cfg)
}

func runWithDB(database *db.DB, cfg *config.Config) int {
	if err := database.Migrate(); err != nil {
		log.Printf("migration failed: %v", err)
		return 1
	}

	// ── Leader election ───────────────────────────────────────────────────────
	leaseTTL := time.Duration(cfg.LeaseTTLSeconds) * time.Second
	elector := scheduler.NewLeaderElector(database, cfg.NodeID, leaseTTL)

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched := scheduler.New(database, elector, cfg.NodeID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx, leaseTTL)

	// ── HTTP API ──────────────────────────────────────────────────────────────
	router := mux.NewRouter()
	handler := api.New(database, elector, cfg.NodeID)
	handler.RegisterRoutes(router)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("[main] node %s starting on port %s", cfg.NodeID, cfg.Port)

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("[main] shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	sched.Stop(shutdownCtx)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[main] server shutdown error: %v", err)
	}
	log.Println("[main] stopped")
	return 0
}

func main() {
	os.Exit(run())
}
