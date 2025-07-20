package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

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

	ctx := context.Background()

	// Apply the recommendation schema
	log.Println("Applying recommendation system database schema...")
	
	// Read the schema file
	schemaPath := filepath.Join(".", "schema_recommendation_tables.sql")
	schemaSQL, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		log.Fatal("Failed to read schema file:", err)
	}

	// Execute the schema
	_, err = db.Pool.Exec(ctx, string(schemaSQL))
	if err != nil {
		log.Fatal("Failed to apply schema:", err)
	}

	log.Println("Recommendation system schema applied successfully!")

	// Populate user interactions from existing submissions
	log.Println("Populating user interactions from existing submissions...")
	
	query := `SELECT populate_user_interactions_from_submissions();`
	var count int
	err = db.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		log.Printf("Warning: Failed to populate user interactions: %v", err)
	} else {
		log.Printf("Populated %d user interactions from submissions", count)
	}

	// Get some basic statistics
	log.Println("Getting recommendation system statistics...")
	
	statsQuery := `SELECT * FROM get_recommendation_system_stats();`
	var totalInteractions, activeUsers, problemsWithFeatures, cachedRecs, trainedModels int64
	var avgUserInteractions *float64
	
	err = db.Pool.QueryRow(ctx, statsQuery).Scan(
		&totalInteractions, &activeUsers, &problemsWithFeatures, 
		&cachedRecs, &trainedModels, &avgUserInteractions,
	)
	if err != nil {
		log.Printf("Warning: Failed to get statistics: %v", err)
	} else {
		log.Printf("Recommendation System Statistics:")
		log.Printf("  Total interactions: %d", totalInteractions)
		log.Printf("  Active users (30 days): %d", activeUsers)
		log.Printf("  Problems with features: %d", problemsWithFeatures)
		log.Printf("  Cached recommendations: %d", cachedRecs)
		log.Printf("  Trained models: %d", trainedModels)
		if avgUserInteractions != nil {
			log.Printf("  Average user interactions: %.2f", *avgUserInteractions)
		}
	}

	log.Println("Migration completed successfully!")
}