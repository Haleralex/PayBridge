// Package main - Entry point for PayBridge API Server.
//
// Пример запуска:
//
//	# Development (defaults)
//	go run cmd/api/main.go
//
//	# With config file
//	go run cmd/api/main.go -config ./configs
//
//	# With environment variables
//	PAYBRIDGE_DATABASE_HOST=localhost \
//	PAYBRIDGE_SERVER_PORT=3000 \
//	go run cmd/api/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Haleralex/wallethub/internal/config"
	"github.com/Haleralex/wallethub/internal/container"
)

// Build-time variables (заполняются при сборке)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "./configs", "Path to config directory")
	configName := flag.String("config-name", "config", "Config file name (without extension)")
	envOnly := flag.Bool("env-only", false, "Load config only from environment variables")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	// Version flag
	if *showVersion {
		fmt.Printf("PayBridge API Server\n")
		fmt.Printf("Version:    %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *envOnly {
		cfg, err = config.LoadFromEnv()
	} else {
		cfg, err = config.Load(*configPath, *configName)
	}

	if err != nil {
		// Fallback to development config
		log.Printf("Warning: Failed to load config: %v", err)
		log.Printf("Using development defaults...")
		cfg = config.Development()
	}

	// Set build info
	cfg.App.Version = version
	cfg.App.BuildTime = buildTime
	cfg.App.GitCommit = gitCommit

	// Create container
	c := container.New(cfg)

	// Initialize with timeout
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	if err := c.Initialize(initCtx); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Setup shutdown handler
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run server in goroutine
	errChan := make(chan error, 1)
	go func() {
		c.Logger().Info("Starting server",
			"address", cfg.Server.Address(),
			"environment", cfg.App.Environment,
			"version", cfg.App.Version,
		)
		errChan <- c.HTTPServer().Start()
	}()

	// Print startup banner
	printBanner(cfg)

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		if err != nil {
			c.Logger().Error("Server error", "error", err)
		}
	case sig := <-quit:
		c.Logger().Info("Received shutdown signal", "signal", sig.String())
	}

	// Graceful shutdown
	c.Logger().Info("Initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := c.Shutdown(shutdownCtx); err != nil {
		c.Logger().Error("Shutdown error", "error", err)
		os.Exit(1)
	}

	c.Logger().Info("Server stopped gracefully")
}

func printBanner(cfg *config.Config) {
	banner := `
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║     ██████╗  █████╗ ██╗   ██╗██████╗ ██████╗ ██╗██████╗       ║
║     ██╔══██╗██╔══██╗╚██╗ ██╔╝██╔══██╗██╔══██╗██║██╔══██╗      ║
║     ██████╔╝███████║ ╚████╔╝ ██████╔╝██████╔╝██║██║  ██║      ║
║     ██╔═══╝ ██╔══██║  ╚██╔╝  ██╔══██╗██╔══██╗██║██║  ██║      ║
║     ██║     ██║  ██║   ██║   ██████╔╝██║  ██║██║██████╔╝      ║
║     ╚═╝     ╚═╝  ╚═╝   ╚═╝   ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝       ║
║                                                               ║
║                   Payment Gateway Service                     ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
	fmt.Printf("  Version:     %s\n", cfg.App.Version)
	fmt.Printf("  Environment: %s\n", cfg.App.Environment)
	fmt.Printf("  Address:     http://%s\n", cfg.Server.Address())
	fmt.Printf("  Health:      http://%s/health\n", cfg.Server.Address())
	fmt.Printf("  API Docs:    http://%s/api/v1\n", cfg.Server.Address())
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()
}
