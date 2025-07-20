package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AnalyticsProcessor handles background processing of analytics data
type AnalyticsProcessor struct {
	service    *Service
	model      *BayesianSkillModel
	config     *AnalyticsConfig
	mu         sync.RWMutex
	running    bool
	stopChan   chan bool
	processors map[string]*processorState
}

// processorState tracks the state of individual user processors
type processorState struct {
	userID      uuid.UUID
	lastProcess time.Time
	processing  bool
}

// ProcessingJob represents a job for analytics processing
type ProcessingJob struct {
	ID        uuid.UUID              `json:"id"`
	UserID    uuid.UUID              `json:"user_id"`
	JobType   string                 `json:"job_type"`
	Priority  int                    `json:"priority"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	Status    string                 `json:"status"`
}

// Job types
const (
	JobTypeUserSkillUpdate    = "user_skill_update"
	JobTypeContestAnalysis    = "contest_analysis"
	JobTypeCacheRefresh       = "cache_refresh"
	JobTypePerformanceReport  = "performance_report"
	JobTypeTrendAnalysis      = "trend_analysis"
)

// Job statuses
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)

// NewAnalyticsProcessor creates a new analytics processor
func NewAnalyticsProcessor(service *Service, model *BayesianSkillModel, config *AnalyticsConfig) *AnalyticsProcessor {
	if config == nil {
		config = DefaultAnalyticsConfig()
	}
	
	return &AnalyticsProcessor{
		service:    service,
		model:      model,
		config:     config,
		stopChan:   make(chan bool),
		processors: make(map[string]*processorState),
	}
}

// Start begins the analytics processing service
func (ap *AnalyticsProcessor) Start(ctx context.Context) error {
	ap.mu.Lock()
	if ap.running {
		ap.mu.Unlock()
		return fmt.Errorf("analytics processor already running")
	}
	ap.running = true
	ap.mu.Unlock()

	log.Println("Starting analytics processor...")

	// Start the main processing loop
	go ap.processLoop(ctx)

	// Start the cache cleanup routine
	go ap.cacheCleanupLoop(ctx)

	// Start periodic job scheduling
	go ap.schedulePeriodicJobs(ctx)

	log.Println("Analytics processor started successfully")
	return nil
}

// Stop stops the analytics processing service
func (ap *AnalyticsProcessor) Stop() error {
	ap.mu.Lock()
	if !ap.running {
		ap.mu.Unlock()
		return fmt.Errorf("analytics processor not running")
	}
	ap.running = false
	ap.mu.Unlock()

	log.Println("Stopping analytics processor...")
	close(ap.stopChan)
	
	// Wait for processors to finish
	time.Sleep(time.Second)
	
	log.Println("Analytics processor stopped")
	return nil
}

// processLoop is the main processing loop
func (ap *AnalyticsProcessor) processLoop(ctx context.Context) {
	ticker := time.NewTicker(ap.config.ProcessingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ap.stopChan:
			return
		case <-ticker.C:
			if err := ap.processEventBatch(ctx); err != nil {
				log.Printf("Error processing event batch: %v", err)
			}
		}
	}
}

// processEventBatch processes a batch of performance events
func (ap *AnalyticsProcessor) processEventBatch(ctx context.Context) error {
	processed, err := ap.service.ProcessPerformanceEvents(ctx, ap.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to process performance events: %w", err)
	}

	if processed > 0 {
		log.Printf("Processed %d performance events", processed)
		
		// Schedule skill updates for affected users
		if err := ap.scheduleSkillUpdatesFromEvents(ctx); err != nil {
			log.Printf("Error scheduling skill updates: %v", err)
		}
	}

	return nil
}

// scheduleSkillUpdatesFromEvents schedules skill updates for users with new events
func (ap *AnalyticsProcessor) scheduleSkillUpdatesFromEvents(ctx context.Context) error {
	// Query for users with recent unprocessed events
	query := `
		SELECT DISTINCT user_id 
		FROM performance_events 
		WHERE processed = true 
		AND processed_at > NOW() - INTERVAL '1 hour'
		AND user_id NOT IN (
			SELECT user_id FROM user_skill_progression 
			WHERE last_updated > NOW() - INTERVAL '30 minutes'
		)`

	rows, err := ap.service.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query users for skill updates: %w", err)
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			log.Printf("Error scanning user ID: %v", err)
			continue
		}
		userIDs = append(userIDs, userID)
	}

	// Schedule skill update jobs for these users
	for _, userID := range userIDs {
		if err := ap.ScheduleJob(ctx, &ProcessingJob{
			ID:        uuid.New(),
			UserID:    userID,
			JobType:   JobTypeUserSkillUpdate,
			Priority:  1,
			Data:      map[string]interface{}{"trigger": "new_events"},
			CreatedAt: time.Now(),
			Status:    JobStatusPending,
		}); err != nil {
			log.Printf("Error scheduling skill update job for user %s: %v", userID, err)
		}
	}

	return nil
}

// ScheduleJob schedules a processing job
func (ap *AnalyticsProcessor) ScheduleJob(ctx context.Context, job *ProcessingJob) error {
	// In a real implementation, this would use a job queue like Redis/Asynq
	// For now, we'll process jobs immediately in a goroutine
	go func() {
		if err := ap.processJob(ctx, job); err != nil {
			log.Printf("Error processing job %s: %v", job.ID, err)
		}
	}()

	return nil
}

// processJob processes a single job
func (ap *AnalyticsProcessor) processJob(ctx context.Context, job *ProcessingJob) error {
	log.Printf("Processing job %s of type %s for user %s", job.ID, job.JobType, job.UserID)

	switch job.JobType {
	case JobTypeUserSkillUpdate:
		return ap.processUserSkillUpdate(ctx, job)
	case JobTypeContestAnalysis:
		return ap.processContestAnalysis(ctx, job)
	case JobTypeCacheRefresh:
		return ap.processCacheRefresh(ctx, job)
	case JobTypePerformanceReport:
		return ap.processPerformanceReport(ctx, job)
	case JobTypeTrendAnalysis:
		return ap.processTrendAnalysis(ctx, job)
	default:
		return fmt.Errorf("unknown job type: %s", job.JobType)
	}
}

// processUserSkillUpdate processes a user skill update job
func (ap *AnalyticsProcessor) processUserSkillUpdate(ctx context.Context, job *ProcessingJob) error {
	// Get recent performance events for the user
	query := `
		SELECT id, user_id, event_type, event_data, submission_id, contest_id, problem_id, recorded_at
		FROM performance_events
		WHERE user_id = $1 AND processed = true AND processed_at > NOW() - INTERVAL '24 hours'
		ORDER BY recorded_at DESC`

	rows, err := ap.service.db.QueryContext(ctx, query, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to query performance events: %w", err)
	}
	defer rows.Close()

	var events []*PerformanceEvent
	for rows.Next() {
		var event PerformanceEvent
		var eventDataJSON []byte

		err := rows.Scan(
			&event.ID, &event.UserID, &event.EventType, &eventDataJSON,
			&event.SubmissionID, &event.ContestID, &event.ProblemID, &event.RecordedAt,
		)
		if err != nil {
			log.Printf("Error scanning performance event: %v", err)
			continue
		}

		if err := json.Unmarshal(eventDataJSON, &event.EventData); err != nil {
			log.Printf("Error unmarshaling event data: %v", err)
			continue
		}

		events = append(events, &event)
	}

	if len(events) == 0 {
		return nil // No new events to process
	}

	// Get current skill progression
	currentSkills, err := ap.service.GetUserSkillProgression(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to get current skill progression: %w", err)
	}

	// Create a map for efficient lookup
	skillMap := make(map[string]*UserSkillProgression)
	for _, skill := range currentSkills {
		skillMap[skill.SkillCategory] = &skill
	}

	// Process each event and extract evidence
	for _, event := range events {
		var evidences []*SkillEvidence

		switch event.EventType {
		case EventTypeSubmission:
			submissionData, err := ap.parseSubmissionEventData(event.EventData)
			if err != nil {
				log.Printf("Error parsing submission event data: %v", err)
				continue
			}

			evidences, err = ap.model.ExtractEvidenceFromSubmission(
				ctx, submissionData, event.UserID, event.RecordedAt,
			)
			if err != nil {
				log.Printf("Error extracting evidence from submission: %v", err)
				continue
			}

		case EventTypeContestSubmit:
			contestData, err := ap.parseContestEventData(event.EventData)
			if err != nil {
				log.Printf("Error parsing contest event data: %v", err)
				continue
			}

			evidences, err = ap.model.ExtractEvidenceFromContest(
				ctx, contestData, event.UserID, event.RecordedAt,
			)
			if err != nil {
				log.Printf("Error extracting evidence from contest: %v", err)
				continue
			}
		}

		// Update skill estimates with new evidence
		for _, evidence := range evidences {
			currentEstimate := skillMap[evidence.SkillCategory]
			var skillEstimate *SkillEstimate

			if currentEstimate != nil {
				skillEstimate = &SkillEstimate{
					UserID:                  currentEstimate.UserID,
					SkillCategory:           currentEstimate.SkillCategory,
					Mean:                    float64(currentEstimate.SkillLevel),
					Variance:                0.1, // Default variance
					ConfidenceIntervalLower: func() float64 { if currentEstimate.ConfidenceIntervalLower != nil { return *currentEstimate.ConfidenceIntervalLower }; return 0.0 }(),
					ConfidenceIntervalUpper: func() float64 { if currentEstimate.ConfidenceIntervalUpper != nil { return *currentEstimate.ConfidenceIntervalUpper }; return 0.0 }(),
					Alpha:                   func() float64 { if currentEstimate.PriorAlpha != nil { return *currentEstimate.PriorAlpha }; return 1.0 }(),
					Beta:                    func() float64 { if currentEstimate.PriorBeta != nil { return *currentEstimate.PriorBeta }; return 1.0 }(),
					EvidenceCount:           currentEstimate.EvidenceCount,
					LastUpdated:             currentEstimate.LastUpdated,
				}
			}

			updatedEstimate, err := ap.model.UpdateSkillEstimate(ctx, skillEstimate, evidence)
			if err != nil {
				log.Printf("Error updating skill estimate: %v", err)
				continue
			}

			// Convert back to UserSkillProgression and save
			progression := &UserSkillProgression{
				UserID:                  updatedEstimate.UserID,
				SkillCategory:           updatedEstimate.SkillCategory,
				SkillLevel:              updatedEstimate.Mean,
				ConfidenceIntervalLower: &updatedEstimate.ConfidenceIntervalLower,
				ConfidenceIntervalUpper: &updatedEstimate.ConfidenceIntervalUpper,
				PriorAlpha:              &updatedEstimate.Alpha,
				PriorBeta:               &updatedEstimate.Beta,
				EvidenceCount:           updatedEstimate.EvidenceCount,
			}

			if err := ap.service.UpdateUserSkillProgression(ctx, progression); err != nil {
				log.Printf("Error saving skill progression: %v", err)
				continue
			}

			// Update our map for subsequent evidence
			skillMap[evidence.SkillCategory] = progression
		}
	}

	// Invalidate relevant caches
	if err := ap.invalidateUserCaches(ctx, job.UserID); err != nil {
		log.Printf("Error invalidating user caches: %v", err)
	}

	log.Printf("Completed skill update for user %s", job.UserID)
	return nil
}

// processContestAnalysis processes contest-specific analysis
func (ap *AnalyticsProcessor) processContestAnalysis(ctx context.Context, job *ProcessingJob) error {
	// Implementation for contest-specific analytics
	log.Printf("Processing contest analysis for user %s", job.UserID)
	return nil
}

// processCacheRefresh refreshes analytics caches
func (ap *AnalyticsProcessor) processCacheRefresh(ctx context.Context, job *ProcessingJob) error {
	// Implementation for cache refresh
	log.Printf("Processing cache refresh for user %s", job.UserID)
	return nil
}

// processPerformanceReport generates performance reports
func (ap *AnalyticsProcessor) processPerformanceReport(ctx context.Context, job *ProcessingJob) error {
	// Implementation for performance report generation
	log.Printf("Processing performance report for user %s", job.UserID)
	return nil
}

// processTrendAnalysis processes trend analysis
func (ap *AnalyticsProcessor) processTrendAnalysis(ctx context.Context, job *ProcessingJob) error {
	// Implementation for trend analysis
	log.Printf("Processing trend analysis for user %s", job.UserID)
	return nil
}

// parseSubmissionEventData parses submission event data
func (ap *AnalyticsProcessor) parseSubmissionEventData(data map[string]interface{}) (*SubmissionEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var submissionData SubmissionEventData
	if err := json.Unmarshal(jsonData, &submissionData); err != nil {
		return nil, err
	}

	return &submissionData, nil
}

// parseContestEventData parses contest event data
func (ap *AnalyticsProcessor) parseContestEventData(data map[string]interface{}) (*ContestEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var contestData ContestEventData
	if err := json.Unmarshal(jsonData, &contestData); err != nil {
		return nil, err
	}

	return &contestData, nil
}

// invalidateUserCaches invalidates caches for a specific user
func (ap *AnalyticsProcessor) invalidateUserCaches(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM performance_analytics_cache WHERE user_id = $1`
	
	_, err := ap.service.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to invalidate user caches: %w", err)
	}

	return nil
}

// cacheCleanupLoop periodically cleans up expired caches
func (ap *AnalyticsProcessor) cacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(ap.config.CacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ap.stopChan:
			return
		case <-ticker.C:
			if err := ap.service.CleanupExpiredCache(ctx); err != nil {
				log.Printf("Error during cache cleanup: %v", err)
			}
		}
	}
}

// schedulePeriodicJobs schedules periodic analytics jobs
func (ap *AnalyticsProcessor) schedulePeriodicJobs(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Schedule jobs every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ap.stopChan:
			return
		case <-ticker.C:
			if err := ap.schedulePeriodicJobsBatch(ctx); err != nil {
				log.Printf("Error scheduling periodic jobs: %v", err)
			}
		}
	}
}

// schedulePeriodicJobsBatch schedules a batch of periodic jobs
func (ap *AnalyticsProcessor) schedulePeriodicJobsBatch(ctx context.Context) error {
	// Schedule cache refresh for active users
	query := `
		SELECT DISTINCT user_id 
		FROM performance_events 
		WHERE recorded_at > NOW() - INTERVAL '24 hours'
		LIMIT 100`

	rows, err := ap.service.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query active users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			continue
		}

		// Schedule cache refresh job
		job := &ProcessingJob{
			ID:        uuid.New(),
			UserID:    userID,
			JobType:   JobTypeCacheRefresh,
			Priority:  3,
			Data:      map[string]interface{}{"trigger": "periodic"},
			CreatedAt: time.Now(),
			Status:    JobStatusPending,
		}

		if err := ap.ScheduleJob(ctx, job); err != nil {
			log.Printf("Error scheduling cache refresh job: %v", err)
		}
	}

	return nil
}

// GetProcessorHealth returns the health status of the analytics processor
func (ap *AnalyticsProcessor) GetProcessorHealth(ctx context.Context) (*AnalyticsHealth, error) {
	ap.mu.RLock()
	running := ap.running
	ap.mu.RUnlock()

	health := &AnalyticsHealth{
		Status:             "unhealthy",
		LastProcessingTime: time.Now(),
	}

	if !running {
		health.Errors = append(health.Errors, "processor not running")
		return health, nil
	}

	// Check unprocessed events
	query := `SELECT COUNT(*) FROM performance_events WHERE processed = false`
	err := ap.service.db.QueryRowContext(ctx, query).Scan(&health.UnprocessedEvents)
	if err != nil {
		health.Errors = append(health.Errors, fmt.Sprintf("failed to count unprocessed events: %v", err))
		return health, nil
	}

	// Check processing lag
	var oldestUnprocessed sql.NullTime
	query = `SELECT MIN(recorded_at) FROM performance_events WHERE processed = false`
	err = ap.service.db.QueryRowContext(ctx, query).Scan(&oldestUnprocessed)
	if err != nil {
		health.Errors = append(health.Errors, fmt.Sprintf("failed to get oldest unprocessed event: %v", err))
		return health, nil
	}

	if oldestUnprocessed.Valid {
		health.EventProcessingLag = time.Since(oldestUnprocessed.Time)
	}

	// Determine overall health status
	if health.UnprocessedEvents > 10000 {
		health.Status = "degraded"
	} else if health.EventProcessingLag > 1*time.Hour {
		health.Status = "degraded"
	} else {
		health.Status = "healthy"
	}

	return health, nil
}