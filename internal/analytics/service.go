package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// PerformanceMetrics represents the 15 key user performance metrics
type PerformanceMetrics struct {
	ID                         uuid.UUID  `json:"id" db:"id"`
	UserID                     uuid.UUID  `json:"user_id" db:"user_id"`
	RecordedAt                 time.Time  `json:"recorded_at" db:"recorded_at"`
	
	// Problem-solving metrics (1-5)
	ProblemSolvingSpeed        *float64   `json:"problem_solving_speed" db:"problem_solving_speed"`
	DebuggingEfficiency        *float64   `json:"debugging_efficiency" db:"debugging_efficiency"`
	CodeComplexityScore        *float64   `json:"code_complexity_score" db:"code_complexity_score"`
	PatternRecognitionAccuracy *float64   `json:"pattern_recognition_accuracy" db:"pattern_recognition_accuracy"`
	AlgorithmSelectionAccuracy *float64   `json:"algorithm_selection_accuracy" db:"algorithm_selection_accuracy"`
	
	// Contest performance metrics (6-10)
	ContestRankingPercentile   *float64   `json:"contest_ranking_percentile" db:"contest_ranking_percentile"`
	TimePressurePerformance    *float64   `json:"time_pressure_performance" db:"time_pressure_performance"`
	MultiProblemEfficiency     *float64   `json:"multi_problem_efficiency" db:"multi_problem_efficiency"`
	ContestConsistency         *float64   `json:"contest_consistency" db:"contest_consistency"`
	PenaltyOptimization        *float64   `json:"penalty_optimization" db:"penalty_optimization"`
	
	// Learning and adaptation metrics (11-15)
	LearningVelocity           *float64   `json:"learning_velocity" db:"learning_velocity"`
	KnowledgeRetention         *float64   `json:"knowledge_retention" db:"knowledge_retention"`
	ErrorPatternReduction      *float64   `json:"error_pattern_reduction" db:"error_pattern_reduction"`
	AdaptiveStrategyUsage      *float64   `json:"adaptive_strategy_usage" db:"adaptive_strategy_usage"`
	MetaCognitiveAwareness     *float64   `json:"meta_cognitive_awareness" db:"meta_cognitive_awareness"`
	
	// Supporting data
	TotalSubmissions           int        `json:"total_submissions" db:"total_submissions"`
	AcceptedSubmissions        int        `json:"accepted_submissions" db:"accepted_submissions"`
	ProblemsAttempted          int        `json:"problems_attempted" db:"problems_attempted"`
	ContestParticipations      int        `json:"contest_participations" db:"contest_participations"`
	
	CreatedAt                  time.Time  `json:"created_at" db:"created_at"`
}

// PerformanceEvent represents raw events for ingestion pipeline
type PerformanceEvent struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	UserID       uuid.UUID              `json:"user_id" db:"user_id"`
	EventType    string                 `json:"event_type" db:"event_type"`
	EventData    map[string]interface{} `json:"event_data" db:"event_data"`
	SubmissionID *uuid.UUID             `json:"submission_id" db:"submission_id"`
	ContestID    *uuid.UUID             `json:"contest_id" db:"contest_id"`
	ProblemID    *uuid.UUID             `json:"problem_id" db:"problem_id"`
	RecordedAt   time.Time              `json:"recorded_at" db:"recorded_at"`
	Processed    bool                   `json:"processed" db:"processed"`
	ProcessedAt  *time.Time             `json:"processed_at" db:"processed_at"`
}

// UserSkillProgression represents Bayesian skill tracking
type UserSkillProgression struct {
	ID                        uuid.UUID  `json:"id" db:"id"`
	UserID                    uuid.UUID  `json:"user_id" db:"user_id"`
	SkillCategory             string     `json:"skill_category" db:"skill_category"`
	SkillLevel                float64    `json:"skill_level" db:"skill_level"`
	ConfidenceIntervalLower   *float64   `json:"confidence_interval_lower" db:"confidence_interval_lower"`
	ConfidenceIntervalUpper   *float64   `json:"confidence_interval_upper" db:"confidence_interval_upper"`
	PriorAlpha                *float64   `json:"prior_alpha" db:"prior_alpha"`
	PriorBeta                 *float64   `json:"prior_beta" db:"prior_beta"`
	EvidenceCount             int        `json:"evidence_count" db:"evidence_count"`
	LastUpdated               time.Time  `json:"last_updated" db:"last_updated"`
}

// PerformanceAnalyticsCache represents cached analytics data
type PerformanceAnalyticsCache struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id"`
	CacheKey  string                 `json:"cache_key" db:"cache_key"`
	CacheData map[string]interface{} `json:"cache_data" db:"cache_data"`
	ValidUntil time.Time             `json:"valid_until" db:"valid_until"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

// PerformanceTimeSeries represents aggregated performance data
type PerformanceTimeSeries struct {
	ID                        uuid.UUID  `json:"id" db:"id"`
	UserID                    uuid.UUID  `json:"user_id" db:"user_id"`
	TimePeriod                string     `json:"time_period" db:"time_period"`
	PeriodStart               time.Time  `json:"period_start" db:"period_start"`
	PeriodEnd                 time.Time  `json:"period_end" db:"period_end"`
	AvgProblemSolvingSpeed    *float64   `json:"avg_problem_solving_speed" db:"avg_problem_solving_speed"`
	AvgDebuggingEfficiency    *float64   `json:"avg_debugging_efficiency" db:"avg_debugging_efficiency"`
	TotalSubmissions          int        `json:"total_submissions" db:"total_submissions"`
	SuccessRate               *float64   `json:"success_rate" db:"success_rate"`
	ImprovementTrend          *float64   `json:"improvement_trend" db:"improvement_trend"`
	CreatedAt                 time.Time  `json:"created_at" db:"created_at"`
}

// Service handles performance analytics operations
type Service struct {
	db *sql.DB
}

// NewService creates a new analytics service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// RecordPerformanceEvent records a raw performance event for processing
func (s *Service) RecordPerformanceEvent(ctx context.Context, event *PerformanceEvent) error {
	eventDataJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	query := `
		INSERT INTO performance_events (user_id, event_type, event_data, submission_id, contest_id, problem_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, recorded_at`
	
	err = s.db.QueryRowContext(ctx, query, 
		event.UserID, event.EventType, eventDataJSON, 
		event.SubmissionID, event.ContestID, event.ProblemID,
	).Scan(&event.ID, &event.RecordedAt)
	
	if err != nil {
		return fmt.Errorf("failed to record performance event: %w", err)
	}

	return nil
}

// ProcessPerformanceEvents processes unprocessed events in batches
func (s *Service) ProcessPerformanceEvents(ctx context.Context, batchSize int) (int, error) {
	var processedCount int
	
	err := s.db.QueryRowContext(ctx, "SELECT process_performance_events()").Scan(&processedCount)
	if err != nil {
		return 0, fmt.Errorf("failed to process performance events: %w", err)
	}

	return processedCount, nil
}

// GetUserPerformanceMetrics retrieves performance metrics for a user
func (s *Service) GetUserPerformanceMetrics(ctx context.Context, userID uuid.UUID, limit int) ([]PerformanceMetrics, error) {
	query := `
		SELECT id, user_id, recorded_at,
			   problem_solving_speed, debugging_efficiency, code_complexity_score,
			   pattern_recognition_accuracy, algorithm_selection_accuracy,
			   contest_ranking_percentile, time_pressure_performance,
			   multi_problem_efficiency, contest_consistency, penalty_optimization,
			   learning_velocity, knowledge_retention, error_pattern_reduction,
			   adaptive_strategy_usage, meta_cognitive_awareness,
			   total_submissions, accepted_submissions, problems_attempted,
			   contest_participations, created_at
		FROM user_performance_metrics
		WHERE user_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance metrics: %w", err)
	}
	defer rows.Close()

	var metrics []PerformanceMetrics
	for rows.Next() {
		var m PerformanceMetrics
		err := rows.Scan(
			&m.ID, &m.UserID, &m.RecordedAt,
			&m.ProblemSolvingSpeed, &m.DebuggingEfficiency, &m.CodeComplexityScore,
			&m.PatternRecognitionAccuracy, &m.AlgorithmSelectionAccuracy,
			&m.ContestRankingPercentile, &m.TimePressurePerformance,
			&m.MultiProblemEfficiency, &m.ContestConsistency, &m.PenaltyOptimization,
			&m.LearningVelocity, &m.KnowledgeRetention, &m.ErrorPatternReduction,
			&m.AdaptiveStrategyUsage, &m.MetaCognitiveAwareness,
			&m.TotalSubmissions, &m.AcceptedSubmissions, &m.ProblemsAttempted,
			&m.ContestParticipations, &m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan performance metrics: %w", err)
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// GetUserSkillProgression retrieves skill progression for a user
func (s *Service) GetUserSkillProgression(ctx context.Context, userID uuid.UUID) ([]UserSkillProgression, error) {
	query := `
		SELECT id, user_id, skill_category, skill_level,
			   confidence_interval_lower, confidence_interval_upper,
			   prior_alpha, prior_beta, evidence_count, last_updated
		FROM user_skill_progression
		WHERE user_id = $1
		ORDER BY skill_category`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query skill progression: %w", err)
	}
	defer rows.Close()

	var progressions []UserSkillProgression
	for rows.Next() {
		var p UserSkillProgression
		err := rows.Scan(
			&p.ID, &p.UserID, &p.SkillCategory, &p.SkillLevel,
			&p.ConfidenceIntervalLower, &p.ConfidenceIntervalUpper,
			&p.PriorAlpha, &p.PriorBeta, &p.EvidenceCount, &p.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill progression: %w", err)
		}
		progressions = append(progressions, p)
	}

	return progressions, nil
}

// UpdateUserSkillProgression updates or inserts skill progression data
func (s *Service) UpdateUserSkillProgression(ctx context.Context, progression *UserSkillProgression) error {
	query := `
		INSERT INTO user_skill_progression (
			user_id, skill_category, skill_level, confidence_interval_lower,
			confidence_interval_upper, prior_alpha, prior_beta, evidence_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, skill_category) 
		DO UPDATE SET
			skill_level = EXCLUDED.skill_level,
			confidence_interval_lower = EXCLUDED.confidence_interval_lower,
			confidence_interval_upper = EXCLUDED.confidence_interval_upper,
			prior_alpha = EXCLUDED.prior_alpha,
			prior_beta = EXCLUDED.prior_beta,
			evidence_count = EXCLUDED.evidence_count,
			last_updated = NOW()
		RETURNING id, last_updated`

	err := s.db.QueryRowContext(ctx, query,
		progression.UserID, progression.SkillCategory, progression.SkillLevel,
		progression.ConfidenceIntervalLower, progression.ConfidenceIntervalUpper,
		progression.PriorAlpha, progression.PriorBeta, progression.EvidenceCount,
	).Scan(&progression.ID, &progression.LastUpdated)

	if err != nil {
		return fmt.Errorf("failed to update skill progression: %w", err)
	}

	return nil
}

// GetAnalyticsCache retrieves cached analytics data
func (s *Service) GetAnalyticsCache(ctx context.Context, userID uuid.UUID, cacheKey string) (*PerformanceAnalyticsCache, error) {
	query := `
		SELECT id, user_id, cache_key, cache_data, valid_until, created_at
		FROM performance_analytics_cache
		WHERE user_id = $1 AND cache_key = $2 AND valid_until > NOW()`

	var cache PerformanceAnalyticsCache
	var cacheDataJSON []byte

	err := s.db.QueryRowContext(ctx, query, userID, cacheKey).Scan(
		&cache.ID, &cache.UserID, &cache.CacheKey, &cacheDataJSON,
		&cache.ValidUntil, &cache.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query analytics cache: %w", err)
	}

	err = json.Unmarshal(cacheDataJSON, &cache.CacheData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return &cache, nil
}

// SetAnalyticsCache stores analytics data in cache
func (s *Service) SetAnalyticsCache(ctx context.Context, cache *PerformanceAnalyticsCache) error {
	cacheDataJSON, err := json.Marshal(cache.CacheData)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	query := `
		INSERT INTO performance_analytics_cache (user_id, cache_key, cache_data, valid_until)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, cache_key)
		DO UPDATE SET
			cache_data = EXCLUDED.cache_data,
			valid_until = EXCLUDED.valid_until,
			created_at = NOW()
		RETURNING id, created_at`

	err = s.db.QueryRowContext(ctx, query,
		cache.UserID, cache.CacheKey, cacheDataJSON, cache.ValidUntil,
	).Scan(&cache.ID, &cache.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to set analytics cache: %w", err)
	}

	return nil
}

// GetPerformanceTimeSeries retrieves time series data for a user
func (s *Service) GetPerformanceTimeSeries(ctx context.Context, userID uuid.UUID, timePeriod string, limit int) ([]PerformanceTimeSeries, error) {
	query := `
		SELECT id, user_id, time_period, period_start, period_end,
			   avg_problem_solving_speed, avg_debugging_efficiency,
			   total_submissions, success_rate, improvement_trend, created_at
		FROM performance_time_series
		WHERE user_id = $1 AND time_period = $2
		ORDER BY period_start DESC
		LIMIT $3`

	rows, err := s.db.QueryContext(ctx, query, userID, timePeriod, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query time series data: %w", err)
	}
	defer rows.Close()

	var timeSeries []PerformanceTimeSeries
	for rows.Next() {
		var ts PerformanceTimeSeries
		err := rows.Scan(
			&ts.ID, &ts.UserID, &ts.TimePeriod, &ts.PeriodStart, &ts.PeriodEnd,
			&ts.AvgProblemSolvingSpeed, &ts.AvgDebuggingEfficiency,
			&ts.TotalSubmissions, &ts.SuccessRate, &ts.ImprovementTrend, &ts.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time series data: %w", err)
		}
		timeSeries = append(timeSeries, ts)
	}

	return timeSeries, nil
}

// CalculateProblemSolvingSpeed calculates problem solving speed for a user and problem
func (s *Service) CalculateProblemSolvingSpeed(ctx context.Context, userID, problemID uuid.UUID) (*float64, error) {
	query := `SELECT calculate_problem_solving_speed($1, $2)`
	
	var speed sql.NullFloat64
	err := s.db.QueryRowContext(ctx, query, userID, problemID).Scan(&speed)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate problem solving speed: %w", err)
	}
	
	if !speed.Valid {
		return nil, nil
	}
	
	return &speed.Float64, nil
}

// CalculateDebuggingEfficiency calculates debugging efficiency for a user and problem
func (s *Service) CalculateDebuggingEfficiency(ctx context.Context, userID, problemID uuid.UUID) (*float64, error) {
	query := `SELECT calculate_debugging_efficiency($1, $2)`
	
	var efficiency sql.NullFloat64
	err := s.db.QueryRowContext(ctx, query, userID, problemID).Scan(&efficiency)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate debugging efficiency: %w", err)
	}
	
	if !efficiency.Valid {
		return nil, nil
	}
	
	return &efficiency.Float64, nil
}

// StartEventProcessor starts a goroutine to process events periodically
func (s *Service) StartEventProcessor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processed, err := s.ProcessPerformanceEvents(ctx, 1000)
				if err != nil {
					log.Printf("Error processing performance events: %v", err)
				} else if processed > 0 {
					log.Printf("Processed %d performance events", processed)
				}
			}
		}
	}()
}

// CleanupExpiredCache removes expired cache entries
func (s *Service) CleanupExpiredCache(ctx context.Context) error {
	query := `DELETE FROM performance_analytics_cache WHERE valid_until < NOW()`
	
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired cache: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Cleaned up %d expired cache entries", rowsAffected)
	}

	return nil
}