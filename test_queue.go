package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"competitive-programming-platform/internal/judge"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize queue manager
	queueManager, err := judge.NewQueueManager()
	if err != nil {
		log.Fatal("Failed to initialize queue manager:", err)
	}
	defer queueManager.Close()

	ctx := context.Background()

	// Test ping task
	fmt.Println("Testing ping task...")
	err = queueManager.EnqueuePing(ctx, "Hello from test!")
	if err != nil {
		log.Fatal("Failed to enqueue ping task:", err)
	}

	// Test submission task
	fmt.Println("Testing submission task...")
	payload := &judge.SubmissionPayload{
		SubmissionID: "test-123",
		UserID:       "user-123",
		ProblemID:    "problem-123",
		Language:     "cpp",
		SourceCode:   "#include <iostream>\nint main() { std::cout << \"Hello World\" << std::endl; return 0; }",
		TimeLimit:    1000,
		MemoryLimit:  256,
	}
	
	err = queueManager.EnqueueSubmission(ctx, payload)
	if err != nil {
		log.Fatal("Failed to enqueue submission task:", err)
	}

	// Get queue stats
	stats, err := queueManager.GetQueueStats(ctx)
	if err != nil {
		log.Fatal("Failed to get queue stats:", err)
	}

	fmt.Printf("Queue stats: %+v\n", stats)
	fmt.Println("Tasks enqueued successfully!")
	fmt.Println("Run the judge worker to process these tasks.")
}