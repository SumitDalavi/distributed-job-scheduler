package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set some environment variables
	os.Setenv("DATABASE_URL", "test-db")
	os.Setenv("PORT", "9090")
	os.Setenv("NODE_ID", "test-node")

	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("NODE_ID")

	cfg := Load()

	if cfg.DatabaseURL != "test-db" {
		t.Errorf("expected test-db, got %s", cfg.DatabaseURL)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected 9090, got %s", cfg.Port)
	}
	if cfg.NodeID != "test-node" {
		t.Errorf("expected test-node, got %s", cfg.NodeID)
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("NODE_ID")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default 8080, got %s", cfg.Port)
	}

	// NODE_ID defaults to hostname, just ensure it's not empty
	if cfg.NodeID == "" {
		t.Errorf("expected fallback NodeID")
	}
}
