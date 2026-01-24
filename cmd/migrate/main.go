package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		migrationsPath string
		databaseURL    string
		command        string
		steps          int
	)

	flag.StringVar(&migrationsPath, "path", "./migrations", "Path to migrations directory")
	flag.StringVar(&databaseURL, "database-url", "", "Database connection URL")
	flag.StringVar(&command, "command", "up", "Migration command: up, down, force, version, drop")
	flag.IntVar(&steps, "steps", 0, "Number of steps for up/down (0 = all)")
	flag.Parse()

	// Try to get database URL from environment if not provided
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		// Build from individual env vars
		host := getEnvOrDefault("PAYBRIDGE_DATABASE_HOST", "localhost")
		port := getEnvOrDefault("PAYBRIDGE_DATABASE_PORT", "5432")
		user := getEnvOrDefault("PAYBRIDGE_DATABASE_USER", "postgres")
		password := getEnvOrDefault("PAYBRIDGE_DATABASE_PASSWORD", "postgres")
		dbname := getEnvOrDefault("PAYBRIDGE_DATABASE_NAME", "paybridge")
		sslmode := getEnvOrDefault("PAYBRIDGE_DATABASE_SSLMODE", "disable")

		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, dbname, sslmode)
	}

	if databaseURL == "" {
		log.Fatal("database URL is required: use -database-url flag or set DATABASE_URL environment variable")
	}

	// Handle positional arguments
	args := flag.Args()
	if len(args) > 0 {
		command = args[0]
	}
	if len(args) > 1 {
		var err error
		steps, err = strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("invalid steps argument: %v", err)
		}
	}

	sourceURL := "file://" + migrationsPath

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}
	defer m.Close()

	// Enable verbose logging
	m.Log = &migrationLogger{}

	switch command {
	case "up":
		if steps > 0 {
			err = m.Steps(steps)
		} else {
			err = m.Up()
		}
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migration up failed: %v", err)
		}
		fmt.Println("Migrations applied successfully")

	case "down":
		if steps > 0 {
			err = m.Steps(-steps)
		} else {
			err = m.Down()
		}
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migration down failed: %v", err)
		}
		fmt.Println("Migrations rolled back successfully")

	case "force":
		if len(args) < 2 {
			log.Fatal("force requires a version argument")
		}
		version, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("invalid version: %v", err)
		}
		if err := m.Force(version); err != nil {
			log.Fatalf("force failed: %v", err)
		}
		fmt.Printf("Forced version to %d\n", version)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("No migrations applied yet")
			} else {
				log.Fatalf("failed to get version: %v", err)
			}
		} else {
			fmt.Printf("Current version: %d (dirty: %v)\n", version, dirty)
		}

	case "drop":
		if err := m.Drop(); err != nil {
			log.Fatalf("drop failed: %v", err)
		}
		fmt.Println("All tables dropped successfully")

	case "create":
		if len(args) < 2 {
			log.Fatal("create requires a migration name")
		}
		name := args[1]
		fmt.Printf("Creating migration: %s\n", name)
		fmt.Println("Please create files manually:")
		fmt.Printf("  migrations/XXXXXX_%s.up.sql\n", name)
		fmt.Printf("  migrations/XXXXXX_%s.down.sql\n", name)

	default:
		log.Fatalf("unknown command: %s\nAvailable commands: up, down, force, version, drop, create", command)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// migrationLogger implements migrate.Logger interface
type migrationLogger struct{}

func (l *migrationLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *migrationLogger) Verbose() bool {
	return true
}
