package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Scheduler represents the job scheduler
type Scheduler struct {
	db *sql.DB
}

// NewScheduler creates a new scheduler
func NewScheduler(db *sql.DB) *Scheduler {
	return &Scheduler{db: db}
}

// Run executes the scheduler loop once (for testing)
func (s *Scheduler) Run() (bool, error) {
	var locked bool
	err := s.db.QueryRow("SELECT pg_try_advisory_lock(12345)").Scan(&locked)
	if err != nil {
		return false, err
	}

	if locked {
		log.Println("Acquired leader lock. Executing cron jobs...")
		// Do work here (mocked for speed in tests)
		
		// Unlock
		_, err = s.db.Exec("SELECT pg_advisory_unlock(12345)")
		if err != nil {
			return true, err
		}
		return true, nil
	}
	
	log.Println("Standby mode. Another replica holds the lock.")
	return false, nil
}

func main() {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost/db?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	
	scheduler := NewScheduler(db)
	for {
		_, err := scheduler.Run()
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
		time.Sleep(5 * time.Second)
	}
}
