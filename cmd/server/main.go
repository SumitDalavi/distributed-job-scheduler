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

func main() {
	cfg := config.Load()

	// ── Database ──────────────────────────────────────────────────────────────
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("migration failed: %v", err)
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
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
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
}
