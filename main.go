package main
import (
	"database/sql"
	"log"
	"time"
	_ "github.com/lib/pq"
)

func main() {
	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/db?sslmode=disable")
	for {
		// Acquire Postgres Advisory Lock (Leader Election)
		var locked bool
		db.QueryRow("SELECT pg_try_advisory_lock(12345)").Scan(&locked)
		if locked {
			log.Println("Acquired leader lock. Executing cron jobs...")
			// Do work here
			time.Sleep(10 * time.Second)
			db.Exec("SELECT pg_advisory_unlock(12345)")
		} else {
			log.Println("Standby mode. Another replica holds the lock.")
			time.Sleep(5 * time.Second)
		}
	}
}
