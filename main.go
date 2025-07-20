package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"competitive-programming-platform/internal/analytics"
	"competitive-programming-platform/internal/auth"
	"competitive-programming-platform/internal/contest"
	"competitive-programming-platform/internal/judge"
	"competitive-programming-platform/internal/metrics"
	"competitive-programming-platform/internal/problem"
	"competitive-programming-platform/internal/realtime"
	"competitive-programming-platform/internal/recommendation"
	"competitive-programming-platform/internal/tracing"
	"competitive-programming-platform/internal/user"
	"competitive-programming-platform/pkg/database"
	"competitive-programming-platform/pkg/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize OpenTelemetry tracing
	tracingConfig := tracing.DefaultConfig()
	tracingConfig.ServiceName = "api-server"
	tracingConfig.ServiceVersion = "1.0.0"
	tracingShutdown := tracing.InitTracing(tracingConfig)
	if tracingShutdown != nil {
		defer func() {
			if err := tracingShutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracing: %v", err)
			}
		}()
	}

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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

	// Initialize services
	authService := auth.NewService(db)
	userService := user.NewService(db)
	problemService := problem.NewService(db)
	contestService := contest.NewService(db)
	judgeService := judge.NewJudgeService(db.Pool, queueManager)
	judgeAPI := judge.NewAPIHandler(judgeService, db.Pool)
	
	// Initialize analytics services
	analyticsService := analytics.NewService(db.Pool)
	bayesianModel := analytics.NewBayesianSkillModel(nil)
	analyticsProcessor := analytics.NewAnalyticsProcessor(analyticsService, bayesianModel, nil)
	analyticsHandler := analytics.NewAnalyticsHandler(analyticsService, bayesianModel, analyticsProcessor)
	
	// Initialize real-time service
	realtimeService := realtime.NewService(db)
	submissionTracker := realtime.NewSubmissionTracker(db, realtimeService)
	
	// Initialize recommendation service
	recommendationService := recommendation.NewService(db)
	recommendationHandlers := recommendation.NewHandlers(recommendationService)
	
	// Start real-time hub
	realtimeService.StartHub(ctx)
	
	// Start submission tracker
	go submissionTracker.StartTracking(ctx)
	
	// Start analytics processor
	if err := analyticsProcessor.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start analytics processor: %v", err)
	}
	defer analyticsProcessor.Stop()
	
	// Initialize recommendation service
	if err := recommendationService.Initialize(ctx); err != nil {
		log.Printf("Warning: Failed to initialize recommendation service: %v", err)
	}
	defer recommendationService.Stop()
	
	// Connect judge service to problem service via adapter
	judgeAdapter := judge.NewJudgeAdapter(judgeService)
	problemService.SetJudgeService(judgeAdapter)

	// Initialize router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(tracing.HTTPMiddleware("api-server"))
	r.Use(metrics.HTTPMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:4321"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Metrics endpoint
	r.Handle("/metrics", metrics.MetricsHandler())

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Group(func(r chi.Router) {
			r.Post("/auth/login", authService.Login)
			r.Post("/auth/register", authService.Register)
			r.Get("/problems", problemService.GetProblems)
			r.Get("/problems/{id}", problemService.GetProblem)
			
			// Contest routes (public)
			r.Get("/contests", contestService.GetContests)
			r.Get("/contests/{id}", contestService.GetContest)
			r.Get("/contests/{id}/problems", contestService.GetContestProblems)
			r.Get("/contests/{id}/standings", contestService.GetContestStandings)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(authService))
			
			// User routes
			r.Get("/users/me", userService.GetCurrentUser)
			r.Put("/users/me", userService.UpdateCurrentUser)
			r.Get("/users/{id}", userService.GetUser)
			
			// Problem routes (authenticated)
			r.Post("/problems", problemService.CreateProblem)
			r.Put("/problems/{id}", problemService.UpdateProblem)
			r.Delete("/problems/{id}", problemService.DeleteProblem)
			
			// Contest routes (authenticated)
			r.Post("/contests", contestService.CreateContest)
			r.Put("/contests/{id}", contestService.UpdateContest)
			r.Delete("/contests/{id}", contestService.DeleteContest)
			r.Post("/contests/{id}/register", contestService.RegisterForContest)
			
			// Submission routes
			r.Post("/problems/{id}/submit", problemService.SubmitSolution)
			r.Get("/submissions", judgeAPI.GetSubmissions)
			r.Get("/submissions/{id}", judgeAPI.GetSubmission)
			
			// Judge routes
			r.Get("/judge/queue/stats", judgeAPI.GetQueueStats)
			
			// Real-time routes (authenticated)
			r.Get("/realtime/sse", realtimeService.HandleSSE)
			r.Get("/realtime/contests/{id}/sse", realtimeService.HandleContestSSE)
			r.Get("/realtime/stats", realtimeService.GetSSEStats)
			r.Get("/realtime/health", realtimeService.HealthCheck)
			
			// Analytics routes (authenticated)
			analyticsHandler.RegisterRoutes(r)
			
			// Recommendation routes (authenticated)
			recommendationHandlers.RegisterRoutes(r)
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	} else {
		log.Println("Server shutdown complete")
	}
}