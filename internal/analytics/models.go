package analytics

import (
	"time"

	"github.com/google/uuid"
)

// SkillCategories defines the 15 key performance metrics categories
var SkillCategories = []string{
	// Problem-solving metrics (1-5)
	"problem_solving_speed",
	"debugging_efficiency", 
	"code_complexity_score",
	"pattern_recognition_accuracy",
	"algorithm_selection_accuracy",
	
	// Contest performance metrics (6-10)
	"contest_ranking_percentile",
	"time_pressure_performance",
	"multi_problem_efficiency",
	"contest_consistency",
	"penalty_optimization",
	
	// Learning and adaptation metrics (11-15)
	"learning_velocity",
	"knowledge_retention",
	"error_pattern_reduction",
	"adaptive_strategy_usage",
	"meta_cognitive_awareness",
}

// EventType constants for different performance events
const (
	EventTypeSubmission     = "submission"
	EventTypeContestJoin    = "contest_join"
	EventTypeProblemView    = "problem_view"
	EventTypeContestSubmit  = "contest_submission"
	EventTypeProblemSolved  = "problem_solved"
	EventTypeSessionStart   = "session_start"
	EventTypeSessionEnd     = "session_end"
	EventTypeDebugAttempt   = "debug_attempt"
	EventTypeHintUsed       = "hint_used"
)

// TimePeriod constants for time series aggregation
const (
	TimePeriodDaily   = "daily"
	TimePeriodWeekly  = "weekly"
	TimePeriodMonthly = "monthly"
)

// CacheKeys for analytics cache
const (
	CacheKeyUserSummary     = "user_summary"
	CacheKeySkillRadar      = "skill_radar"
	CacheKeyPerformanceTrend = "performance_trend"
	CacheKeyComparison      = "peer_comparison"
	CacheKeyRecommendations = "recommendations"
)

// Default cache durations
const (
	CacheDurationShort  = 15 * time.Minute
	CacheDurationMedium = 1 * time.Hour
	CacheDurationLong   = 24 * time.Hour
)

// SubmissionEventData represents data for submission events
type SubmissionEventData struct {
	SubmissionID     uuid.UUID `json:"submission_id"`
	ProblemID        uuid.UUID `json:"problem_id"`
	Status           string    `json:"status"`
	ExecutionTime    *int      `json:"execution_time,omitempty"`
	MemoryUsage      *int      `json:"memory_usage,omitempty"`
	Language         string    `json:"language"`
	TestCasesPassed  int       `json:"test_cases_passed"`
	TotalTestCases   int       `json:"total_test_cases"`
	SourceCodeLength int       `json:"source_code_length,omitempty"`
}

// ContestEventData represents data for contest events
type ContestEventData struct {
	ContestID       uuid.UUID `json:"contest_id"`
	Action          string    `json:"action"` // "join", "submit", "leave"
	ProblemID       *uuid.UUID `json:"problem_id,omitempty"`
	SubmissionID    *uuid.UUID `json:"submission_id,omitempty"`
	TimeFromStart   int       `json:"time_from_start"` // minutes from contest start
	RankAtTime      *int      `json:"rank_at_time,omitempty"`
}

// ProblemViewEventData represents data for problem view events
type ProblemViewEventData struct {
	ProblemID     uuid.UUID `json:"problem_id"`
	ViewDuration  int       `json:"view_duration"` // seconds
	ScrollDepth   float64   `json:"scroll_depth"`  // 0.0 to 1.0
	HintsViewed   int       `json:"hints_viewed"`
	SampleTested  bool      `json:"sample_tested"`
}

// SessionEventData represents data for session events
type SessionEventData struct {
	SessionID       uuid.UUID `json:"session_id"`
	Duration        int       `json:"duration"`        // seconds
	ProblemsViewed  int       `json:"problems_viewed"`
	SubmissionsMade int       `json:"submissions_made"`
	Language        string    `json:"language"`
}

// DebugEventData represents data for debug attempt events
type DebugEventData struct {
	SubmissionID    uuid.UUID `json:"submission_id"`
	ProblemID       uuid.UUID `json:"problem_id"`
	AttemptNumber   int       `json:"attempt_number"`
	ErrorType       string    `json:"error_type"`       // "compile", "runtime", "wrong_answer", "tle", "mle"
	FixAttempted    bool      `json:"fix_attempted"`
	FixSuccessful   bool      `json:"fix_successful"`
	TimeSpent       int       `json:"time_spent"`       // seconds
}

// UserPerformanceSummary represents a high-level performance summary
type UserPerformanceSummary struct {
	UserID                   uuid.UUID `json:"user_id"`
	OverallRating           float64   `json:"overall_rating"`
	PerformanceLevel        string    `json:"performance_level"` // "beginner", "intermediate", "advanced", "expert"
	StrongSkills            []string  `json:"strong_skills"`
	WeakSkills              []string  `json:"weak_skills"`
	RecentTrend             string    `json:"recent_trend"`      // "improving", "stable", "declining"
	TotalProblemsAttempted  int       `json:"total_problems_attempted"`
	TotalProblemsSolved     int       `json:"total_problems_solved"`
	ContestsParticipated    int       `json:"contests_participated"`
	LastActive              time.Time `json:"last_active"`
	StreakDays              int       `json:"streak_days"`
}

// SkillRadarData represents data for skill radar visualization
type SkillRadarData struct {
	UserID           uuid.UUID                `json:"user_id"`
	Skills           map[string]float64       `json:"skills"`
	ConfidenceRanges map[string][2]float64    `json:"confidence_ranges"`
	PeerComparison   map[string]float64       `json:"peer_comparison,omitempty"`
	LastUpdated      time.Time                `json:"last_updated"`
}

// PerformanceTrendData represents performance trend over time
type PerformanceTrendData struct {
	UserID     uuid.UUID                    `json:"user_id"`
	Period     string                       `json:"period"` // "daily", "weekly", "monthly"
	DataPoints []PerformanceTrendPoint      `json:"data_points"`
	Metrics    []string                     `json:"metrics"`
}

// PerformanceTrendPoint represents a single point in the trend
type PerformanceTrendPoint struct {
	Timestamp time.Time            `json:"timestamp"`
	Values    map[string]float64   `json:"values"`
}

// PeerComparisonData represents comparison with peer group
type PeerComparisonData struct {
	UserID          uuid.UUID            `json:"user_id"`
	UserMetrics     map[string]float64   `json:"user_metrics"`
	PeerAverages    map[string]float64   `json:"peer_averages"`
	Percentiles     map[string]float64   `json:"percentiles"`
	SimilarUsers    []uuid.UUID          `json:"similar_users"`
	ComparisonLevel string               `json:"comparison_level"` // "rating_band", "contest_frequency", "problem_difficulty"
}

// RecommendationData represents personalized recommendations
type RecommendationData struct {
	UserID           uuid.UUID          `json:"user_id"`
	SkillFocus       []string           `json:"skill_focus"`
	ProblemTypes     []string           `json:"problem_types"`
	DifficultyRange  [2]int             `json:"difficulty_range"`
	ContestStrategy  string             `json:"contest_strategy"`
	LearningPath     []LearningStep     `json:"learning_path"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

// LearningStep represents a step in the learning path
type LearningStep struct {
	StepNumber    int      `json:"step_number"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	SkillTargets  []string `json:"skill_targets"`
	Resources     []string `json:"resources"`
	EstimatedTime string   `json:"estimated_time"`
}

// AnalyticsConfig holds configuration for analytics processing
type AnalyticsConfig struct {
	ProcessingInterval     time.Duration `json:"processing_interval"`
	BatchSize              int           `json:"batch_size"`
	CacheCleanupInterval   time.Duration `json:"cache_cleanup_interval"`
	EnableRealtimeUpdates  bool          `json:"enable_realtime_updates"`
	SkillUpdateThreshold   int           `json:"skill_update_threshold"`
	TrendAnalysisWindow    time.Duration `json:"trend_analysis_window"`
}

// DefaultAnalyticsConfig returns default configuration
func DefaultAnalyticsConfig() *AnalyticsConfig {
	return &AnalyticsConfig{
		ProcessingInterval:     30 * time.Second,
		BatchSize:              1000,
		CacheCleanupInterval:   1 * time.Hour,
		EnableRealtimeUpdates:  true,
		SkillUpdateThreshold:   5,
		TrendAnalysisWindow:    30 * 24 * time.Hour, // 30 days
	}
}

// BayesianParameters represents parameters for Bayesian skill estimation
type BayesianParameters struct {
	PriorAlpha    float64 `json:"prior_alpha"`
	PriorBeta     float64 `json:"prior_beta"`
	LearningRate  float64 `json:"learning_rate"`
	DecayFactor   float64 `json:"decay_factor"`
	MinEvidence   int     `json:"min_evidence"`
}

// DefaultBayesianParameters returns default Bayesian parameters
func DefaultBayesianParameters() *BayesianParameters {
	return &BayesianParameters{
		PriorAlpha:   1.0,
		PriorBeta:    1.0,
		LearningRate: 0.1,
		DecayFactor:  0.95,
		MinEvidence:  3,
	}
}

// MetricWeights defines weights for different metrics in overall rating calculation
type MetricWeights struct {
	ProblemSolving   float64 `json:"problem_solving"`
	Contest          float64 `json:"contest"`
	Learning         float64 `json:"learning"`
	Consistency      float64 `json:"consistency"`
	Efficiency       float64 `json:"efficiency"`
}

// DefaultMetricWeights returns default weights for metrics
func DefaultMetricWeights() *MetricWeights {
	return &MetricWeights{
		ProblemSolving: 0.3,
		Contest:        0.25,
		Learning:       0.2,
		Consistency:    0.15,
		Efficiency:     0.1,
	}
}

// ValidationResult represents validation results for analytics data
type ValidationResult struct {
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	MetricsChecked int     `json:"metrics_checked"`
}

// AnalyticsHealth represents health status of analytics system
type AnalyticsHealth struct {
	Status                string    `json:"status"` // "healthy", "degraded", "unhealthy"
	EventProcessingLag    time.Duration `json:"event_processing_lag"`
	UnprocessedEvents     int       `json:"unprocessed_events"`
	CacheHitRate          float64   `json:"cache_hit_rate"`
	LastProcessingTime    time.Time `json:"last_processing_time"`
	Errors                []string  `json:"errors,omitempty"`
}