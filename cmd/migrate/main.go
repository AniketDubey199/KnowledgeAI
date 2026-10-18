package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"knowledgebase-ai/internal/config"
)

func main() {
	var migrationsPath string
	flag.StringVar(&migrationsPath, "path", "./migrations", "Path to migration files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Usage: migrate [-path ./migrations] <up|down|version>")
	}

	cmd := args[0]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)
	m, err := migrate.New(sourceURL, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Migration initialization failed: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("Source close error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("Database close error: %v", dbErr)
		}
	}()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully!")
	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Reverted 1 migration step successfully!")
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get migration version: %v", err)
		}
		log.Printf("Current migration version: %d (dirty: %v)\n", v, dirty)
	default:
		log.Fatalf("Unknown migration command: %s. Expected 'up', 'down', or 'version'", cmd)
		os.Exit(1)
	}
}
