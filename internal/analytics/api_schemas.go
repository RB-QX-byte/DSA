package analytics

import (
	"time"
	"github.com/google/uuid"
)

// VisualizationDataStructures optimized for Chart.js and D3.js consumption
// These structures minimize client-side data transformation

// SkillRadarVisualization represents data for radar/spider charts
type SkillRadarVisualization struct {
	UserID uuid.UUID `json:"user_id"`
	Data   []SkillRadarPoint `json:"data"`
	Meta   SkillRadarMeta    `json:"meta"`
}

// SkillRadarPoint represents a single skill point for radar chart
type SkillRadarPoint struct {
	Skill       string  `json:"skill"`        // Human-readable skill name
	Score       float64 `json:"score"`        // 0-100 scale for easier visualization
	Confidence  float64 `json:"confidence"`   // 0-100 scale
	Category    string  `json:"category"`     // "problem_solving", "contest", "learning"
	Description string  `json:"description"`  // Tooltip description
}

// SkillRadarMeta contains metadata for the radar chart
type SkillRadarMeta struct {
	MaxScore       float64   `json:"max_score"`
	LastUpdated    time.Time `json:"last_updated"`
	TotalSkills    int       `json:"total_skills"`
	AverageScore   float64   `json:"average_score"`
	StrongestSkill string    `json:"strongest_skill"`
	WeakestSkill   string    `json:"weakest_skill"`
}

// PerformanceTrendVisualization represents data for line/area charts
type PerformanceTrendVisualization struct {
	UserID   uuid.UUID              `json:"user_id"`
	Period   string                 `json:"period"` // "daily", "weekly", "monthly"
	Datasets []TrendDataset         `json:"datasets"`
	Meta     PerformanceTrendMeta   `json:"meta"`
}

// TrendDataset represents a single metric's trend data
type TrendDataset struct {
	Label       string      `json:"label"`        // "Problem Solving Speed", "Success Rate"
	MetricKey   string      `json:"metric_key"`   // "problem_solving_speed", "success_rate"
	Data        []DataPoint `json:"data"`         // Array of [timestamp, value] pairs
	Color       string      `json:"color"`        // Hex color for chart
	Unit        string      `json:"unit"`         // "minutes", "percentage", "score"
	Description string      `json:"description"`  // Tooltip description
}

// DataPoint represents a single data point as [timestamp, value]
type DataPoint [2]interface{} // [timestamp (ISO string), value (number)]

// PerformanceTrendMeta contains metadata for trend charts
type PerformanceTrendMeta struct {
	DateRange    DateRange `json:"date_range"`
	TotalPoints  int       `json:"total_points"`
	TrendSummary map[string]TrendDirection `json:"trend_summary"`
}

// DateRange represents the date range of the data
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TrendDirection represents the direction of a trend
type TrendDirection struct {
	Direction string  `json:"direction"` // "up", "down", "stable"
	Change    float64 `json:"change"`    // Percentage change
	Period    string  `json:"period"`    // "last_week", "last_month"
}

// PerformanceSummaryVisualization represents summary data for dashboard cards
type PerformanceSummaryVisualization struct {
	UserID  uuid.UUID          `json:"user_id"`
	Summary SummaryMetrics     `json:"summary"`
	Badges  []AchievementBadge `json:"badges"`
	Stats   []StatCard         `json:"stats"`
}

// SummaryMetrics contains key performance indicators
type SummaryMetrics struct {
	OverallRating     float64 `json:"overall_rating"`      // 0-100 scale
	PerformanceLevel  string  `json:"performance_level"`   // "Beginner", "Intermediate", etc.
	Rank              int     `json:"rank"`                // Global rank (if available)
	TotalUsers        int     `json:"total_users"`         // Total users for context
	RecentTrend       string  `json:"recent_trend"`        // "improving", "stable", "declining"
	TrendPercentage   float64 `json:"trend_percentage"`    // Percentage change
	LastActive        time.Time `json:"last_active"`
}

// AchievementBadge represents earned badges/achievements
type AchievementBadge struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`       // Icon name or URL
	Color       string    `json:"color"`      // Badge color
	EarnedAt    time.Time `json:"earned_at"`
	Rarity      string    `json:"rarity"`     // "common", "rare", "epic", "legendary"
}

// StatCard represents a dashboard stat card
type StatCard struct {
	Title       string      `json:"title"`
	Value       interface{} `json:"value"`       // Can be number or string
	Unit        string      `json:"unit"`        // "problems", "%", "minutes"
	Change      float64     `json:"change"`      // Percentage change
	ChangeType  string      `json:"change_type"` // "positive", "negative", "neutral"
	Icon        string      `json:"icon"`
	Description string      `json:"description"`
}

// PeerComparisonVisualization represents peer comparison data
type PeerComparisonVisualization struct {
	UserID     uuid.UUID         `json:"user_id"`
	Categories []ComparisonCategory `json:"categories"`
	Meta       ComparisonMeta    `json:"meta"`
}

// ComparisonCategory represents comparison in a skill category
type ComparisonCategory struct {
	Category     string  `json:"category"`
	UserScore    float64 `json:"user_score"`
	PeerAverage  float64 `json:"peer_average"`
	Percentile   float64 `json:"percentile"`   // 0-100
	Rank         int     `json:"rank"`         // Rank within peer group
	TotalPeers   int     `json:"total_peers"`
	Status       string  `json:"status"`      // "above_average", "below_average", "average"
}

// ComparisonMeta contains metadata for peer comparison
type ComparisonMeta struct {
	PeerGroupSize   int    `json:"peer_group_size"`
	ComparisonBasis string `json:"comparison_basis"` // "rating_band", "experience_level"
	LastUpdated     time.Time `json:"last_updated"`
}

// PredictionVisualization represents performance predictions
type PredictionVisualization struct {
	UserID      uuid.UUID            `json:"user_id"`
	Scenarios   []PredictionScenario `json:"scenarios"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// PredictionScenario represents a prediction for a specific scenario
type PredictionScenario struct {
	Scenario      string  `json:"scenario"`      // "easy_problem", "contest"
	Prediction    float64 `json:"prediction"`    // 0-100 probability of success
	Confidence    float64 `json:"confidence"`    // 0-100 confidence in prediction
	Description   string  `json:"description"`
	Recommendation string `json:"recommendation"`
	Color         string  `json:"color"`         // Color for visualization
}

// RecommendationVisualization represents personalized recommendations
type RecommendationVisualization struct {
	UserID          uuid.UUID           `json:"user_id"`
	SkillFocus      []SkillRecommendation `json:"skill_focus"`
	ProblemTypes    []ProblemRecommendation `json:"problem_types"`
	LearningPath    []LearningStepCard  `json:"learning_path"`
	DifficultyRange DifficultyRange     `json:"difficulty_range"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// SkillRecommendation represents a skill improvement recommendation
type SkillRecommendation struct {
	Skill       string  `json:"skill"`
	CurrentScore float64 `json:"current_score"`
	TargetScore  float64 `json:"target_score"`
	Priority     string  `json:"priority"`     // "high", "medium", "low"
	Reason       string  `json:"reason"`
	EstimatedTime string `json:"estimated_time"`
}

// ProblemRecommendation represents a problem type recommendation
type ProblemRecommendation struct {
	Type         string   `json:"type"`         // "dynamic-programming", "graphs"
	Difficulty   int      `json:"difficulty"`   // Rating difficulty
	Count        int      `json:"count"`        // Number of problems to solve
	Priority     string   `json:"priority"`
	Tags         []string `json:"tags"`         // Related tags
	Description  string   `json:"description"`
}

// LearningStepCard represents a learning path step card
type LearningStepCard struct {
	StepNumber    int      `json:"step_number"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Skills        []string `json:"skills"`         // Target skills
	Resources     []Resource `json:"resources"`
	EstimatedTime string   `json:"estimated_time"`
	Status        string   `json:"status"`         // "pending", "in_progress", "completed"
	Progress      float64  `json:"progress"`       // 0-100
}

// Resource represents a learning resource
type Resource struct {
	Type        string `json:"type"`        // "article", "video", "practice"
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Duration    string `json:"duration"`    // "15 minutes", "2 hours"
}

// DifficultyRange represents recommended difficulty range
type DifficultyRange struct {
	Min         int    `json:"min"`
	Max         int    `json:"max"`
	Current     int    `json:"current"`      // User's current level
	Recommended int    `json:"recommended"`  // Recommended next level
	Description string `json:"description"`
}

// SystemMetricsVisualization represents system-wide analytics
type SystemMetricsVisualization struct {
	Metrics     []SystemMetric `json:"metrics"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Status      string         `json:"status"`      // "healthy", "degraded", "unhealthy"
}

// SystemMetric represents a system-wide metric
type SystemMetric struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Unit        string      `json:"unit"`
	Description string      `json:"description"`
	Status      string      `json:"status"`      // "good", "warning", "critical"
	Threshold   float64     `json:"threshold"`
}

// HealthStatusVisualization represents health status
type HealthStatusVisualization struct {
	Status               string        `json:"status"`
	Components           []HealthComponent `json:"components"`
	EventProcessingLag   string        `json:"event_processing_lag"`   // Human readable
	UnprocessedEvents    int           `json:"unprocessed_events"`
	CacheHitRate         float64       `json:"cache_hit_rate"`         // 0-100
	LastProcessingTime   time.Time     `json:"last_processing_time"`
	Uptime               string        `json:"uptime"`                 // Human readable
}

// HealthComponent represents a system component's health
type HealthComponent struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	LastCheck   time.Time `json:"last_check"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// API Response Wrappers

// APIResponse is the standard API response wrapper
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *APIMeta    `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// APIMeta contains metadata about the response
type APIMeta struct {
	RequestID   string `json:"request_id,omitempty"`
	CacheHit    bool   `json:"cache_hit,omitempty"`
	ProcessTime string `json:"process_time,omitempty"`
	Version     string `json:"version,omitempty"`
}

// Chart.js specific formats

// ChartJSDataset represents a Chart.js compatible dataset
type ChartJSDataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BackgroundColor string    `json:"backgroundColor,omitempty"`
	BorderColor     string    `json:"borderColor,omitempty"`
	BorderWidth     int       `json:"borderWidth,omitempty"`
	Fill            bool      `json:"fill,omitempty"`
	Tension         float64   `json:"tension,omitempty"`
}

// ChartJSData represents Chart.js compatible data structure
type ChartJSData struct {
	Labels   []string         `json:"labels"`
	Datasets []ChartJSDataset `json:"datasets"`
}

// ChartJSRadarData represents Chart.js radar chart data
type ChartJSRadarData struct {
	Labels   []string         `json:"labels"`    // Skill names
	Datasets []ChartJSDataset `json:"datasets"`  // User data, peer average, etc.
}

// D3.js specific formats

// D3Node represents a node in D3.js network/tree visualizations
type D3Node struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Value    float64                `json:"value"`
	Group    int                    `json:"group"`
	Children []D3Node               `json:"children,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// D3Link represents a link in D3.js network visualizations
type D3Link struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Value  float64 `json:"value"`
}

// D3NetworkData represents D3.js network data
type D3NetworkData struct {
	Nodes []D3Node `json:"nodes"`
	Links []D3Link `json:"links"`
}

// Utility functions for data transformation

// SkillCategoryMapping maps internal skill names to display names
var SkillCategoryMapping = map[string]string{
	"problem_solving_speed":        "Problem Solving Speed",
	"debugging_efficiency":         "Debugging Efficiency",
	"code_complexity_score":        "Code Quality",
	"pattern_recognition_accuracy": "Pattern Recognition",
	"algorithm_selection_accuracy": "Algorithm Selection",
	"contest_ranking_percentile":   "Contest Ranking",
	"time_pressure_performance":    "Time Pressure",
	"multi_problem_efficiency":     "Multi-Problem Handling",
	"contest_consistency":          "Contest Consistency",
	"penalty_optimization":         "Penalty Optimization",
	"learning_velocity":            "Learning Speed",
	"knowledge_retention":          "Knowledge Retention",
	"error_pattern_reduction":      "Error Reduction",
	"adaptive_strategy_usage":      "Strategy Adaptation",
	"meta_cognitive_awareness":     "Self-Assessment",
}

// SkillCategoryColors maps skill categories to colors for visualization
var SkillCategoryColors = map[string]string{
	"problem_solving_speed":        "#FF6384",
	"debugging_efficiency":         "#36A2EB",
	"code_complexity_score":        "#FFCE56",
	"pattern_recognition_accuracy": "#4BC0C0",
	"algorithm_selection_accuracy": "#9966FF",
	"contest_ranking_percentile":   "#FF9F40",
	"time_pressure_performance":    "#FF6384",
	"multi_problem_efficiency":     "#C9CBCF",
	"contest_consistency":          "#4BC0C0",
	"penalty_optimization":         "#36A2EB",
	"learning_velocity":            "#FFCE56",
	"knowledge_retention":          "#9966FF",
	"error_pattern_reduction":      "#FF9F40",
	"adaptive_strategy_usage":      "#FF6384",
	"meta_cognitive_awareness":     "#36A2EB",
}

// GetDisplayName returns the human-readable display name for a skill category
func GetDisplayName(skillCategory string) string {
	if displayName, exists := SkillCategoryMapping[skillCategory]; exists {
		return displayName
	}
	return skillCategory
}

// GetSkillColor returns the color for a skill category
func GetSkillColor(skillCategory string) string {
	if color, exists := SkillCategoryColors[skillCategory]; exists {
		return color
	}
	return "#999999" // Default gray
}

// ConvertToChartJS converts generic data to Chart.js format
func ConvertToChartJS(data interface{}) interface{} {
	// Implementation would depend on the specific data type
	// This is a placeholder for the conversion logic
	return data
}

// ConvertToD3 converts generic data to D3.js format
func ConvertToD3(data interface{}) interface{} {
	// Implementation would depend on the specific data type
	// This is a placeholder for the conversion logic
	return data
}