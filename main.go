package main

import (
	"database/sql"
	"log"
	"time"
	"math/rand"

	_ "github.com/lib/pq"
)

type Scheduler struct {
	db *sql.DB
}

func NewScheduler(db *sql.DB) *Scheduler {
	return &Scheduler{db: db}
}

func (s *Scheduler) ExecuteJob(jobID int) {
	log.Printf("Executing job %d...\n", jobID)
	// Simulate work
	time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
	log.Printf("Finished job %d\n", jobID)
}

func (s *Scheduler) Run() (bool, error) {
	var locked bool
	// Try to acquire the leader lock
	err := s.db.QueryRow("SELECT pg_try_advisory_lock(12345)").Scan(&locked)
	if err != nil {
		return false, err
	}

	if locked {
		log.Println("Acquired leader lock. Checking for due jobs...")
		
		// In a real system we'd query the DB for jobs. 
		// We'll mock a fan-out to workers.
		for i := 1; i <= 3; i++ {
			go s.ExecuteJob(i)
		}
		
		// Unlock after scheduling
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
		time.Sleep(10 * time.Second)
	}
}
