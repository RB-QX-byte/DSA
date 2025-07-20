package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"competitive-programming-platform/internal/judge"
	"competitive-programming-platform/internal/metrics"
	"competitive-programming-platform/internal/tracing"
	"competitive-programming-platform/pkg/database"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize OpenTelemetry tracing
	tracingConfig := tracing.DefaultConfig()
	tracingConfig.ServiceName = "judge-worker"
	tracingConfig.ServiceVersion = "1.0.0"
	tracingShutdown := tracing.InitTracing(tracingConfig)
	if tracingShutdown != nil {
		defer func() {
			if err := tracingShutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracing: %v", err)
			}
		}()
	}

	// Initialize database connection
	db, err := database.NewConnection()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize queue manager
	queueManager, err := judge.NewQueueManager()
	if err != nil {
		log.Fatal("Failed to initialize queue manager:", err)
	}
	defer queueManager.Close()

	// Initialize judge service
	judgeService := judge.NewJudgeService(db, queueManager)

	// Register task handlers
	queueManager.RegisterHandlers(judgeService)

	// Start metrics server
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "8082"
	}
	
	http.Handle("/metrics", metrics.MetricsHandler())
	go func() {
		log.Printf("Metrics server starting on port %s", metricsPort)
		if err := http.ListenAndServe(":"+metricsPort, nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	log.Println("Judge worker started successfully")
	log.Println("Press Ctrl+C to stop the worker")

	// Wait for interrupt signal to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down judge worker...")
	
	// Gracefully stop the server
	queueManager.Server.Stop()
	log.Println("Judge worker stopped")
}