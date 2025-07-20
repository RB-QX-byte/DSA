package main

import (
	"context"
	"log"
	"time"

	"competitive-programming-platform/internal/judge"
	"competitive-programming-platform/pkg/database"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
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

	ctx := context.Background()

	// Test submission
	payload := &judge.SubmissionPayload{
		UserID:      "test-user-id",
		ProblemID:   "test-problem-id",
		Language:    "cpp",
		SourceCode:  "#include <iostream>\nint main() { std::cout << \"Hello World\" << std::endl; return 0; }",
		TimeLimit:   1000,
		MemoryLimit: 256,
	}

	log.Println("Testing submission integration...")
	
	// Test the submission process
	err = judgeService.SubmitForJudging(ctx, payload)
	if err != nil {
		log.Printf("Submission failed: %v", err)
	} else {
		log.Println("Submission successful!")
	}

	// Wait a bit for processing
	time.Sleep(3 * time.Second)
	
	// Test queue stats
	stats, err := queueManager.GetQueueStats(ctx)
	if err != nil {
		log.Printf("Failed to get queue stats: %v", err)
	} else {
		log.Printf("Queue stats: %+v", stats)
	}

	log.Println("Integration test completed!")
}