package config

import (
	"log"
	"os"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL     string
	Port            string
	NodeID          string
	LeaseTTLSeconds int
	TickIntervalMs  int
}

// Load reads config from environment with sane defaults.
func Load() *Config {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal("cannot determine node ID:", err)
		}
		nodeID = hostname
	}
	return &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://scheduler:scheduler@localhost:5432/scheduler?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		NodeID:          nodeID,
		LeaseTTLSeconds: 30,
		TickIntervalMs:  1000,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
