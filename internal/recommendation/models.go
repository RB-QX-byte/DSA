package recommendation

import (
	"time"

	"github.com/google/uuid"
)

// UserInteraction represents a user's interaction with a problem
type UserInteraction struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	ProblemID        uuid.UUID `json:"problem_id"`
	InteractionType  string    `json:"interaction_type"` // "view", "attempt", "solve", "hint_used"
	Duration         int       `json:"duration"`         // seconds spent
	Success          bool      `json:"success"`          // whether problem was solved
	AttemptCount     int       `json:"attempt_count"`
	LanguageUsed     string    `json:"language_used"`
	SolutionQuality  float64   `json:"solution_quality"` // 0.0 to 1.0
	DifficultyRating float64   `json:"difficulty_rating"` // user's perceived difficulty
	Timestamp        time.Time `json:"timestamp"`
}

// UserProfile represents a user's skill profile for recommendations
type UserProfile struct {
	UserID              uuid.UUID              `json:"user_id"`
	SkillVector         map[string]float64     `json:"skill_vector"`         // skill category -> proficiency
	PreferredDifficulty [2]int                 `json:"preferred_difficulty"` // [min, max] difficulty range
	PreferredTags       []string               `json:"preferred_tags"`
	PreferredLanguages  []string               `json:"preferred_languages"`
	SolvedProblems      []uuid.UUID            `json:"solved_problems"`
	AttemptedProblems   []uuid.UUID            `json:"attempted_problems"`
	WeakAreas           []string               `json:"weak_areas"`
	LearningGoals       []string               `json:"learning_goals"`
	ActivityPattern     map[string]float64     `json:"activity_pattern"`     // time of day -> activity score
	LastActive          time.Time              `json:"last_active"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// ProblemFeatures represents feature vector for a problem
type ProblemFeatures struct {
	ProblemID        uuid.UUID          `json:"problem_id"`
	Title            string             `json:"title"`
	Difficulty       int                `json:"difficulty"`
	Tags             []string           `json:"tags"`
	AcceptanceRate   float64            `json:"acceptance_rate"`
	AverageAttempts  float64            `json:"average_attempts"`
	AverageSolveTime float64            `json:"average_solve_time"` // in minutes
	TopicVector      map[string]float64 `json:"topic_vector"`       // topic -> weight
	ComplexityScore  float64            `json:"complexity_score"`
	PopularityScore  float64            `json:"popularity_score"`
	SimilarProblems  []uuid.UUID        `json:"similar_problems"`
	Prerequisites    []uuid.UUID        `json:"prerequisites"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// UserSimilarity represents similarity between users
type UserSimilarity struct {
	UserID1        uuid.UUID `json:"user_id_1"`
	UserID2        uuid.UUID `json:"user_id_2"`
	SimilarityType string    `json:"similarity_type"` // "cosine", "pearson", "jaccard"
	Score          float64   `json:"score"`           // 0.0 to 1.0
	SharedProblems int       `json:"shared_problems"`
	ComputedAt     time.Time `json:"computed_at"`
}

// ProblemSimilarity represents similarity between problems
type ProblemSimilarity struct {
	ProblemID1     uuid.UUID `json:"problem_id_1"`
	ProblemID2     uuid.UUID `json:"problem_id_2"`
	SimilarityType string    `json:"similarity_type"` // "content", "collaborative", "difficulty"
	Score          float64   `json:"score"`           // 0.0 to 1.0
	CommonTags     []string  `json:"common_tags"`
	ComputedAt     time.Time `json:"computed_at"`
}

// RecommendationResult represents a single recommendation
type RecommendationResult struct {
	ProblemID        uuid.UUID          `json:"problem_id"`
	Score            float64            `json:"score"`            // overall recommendation score
	Confidence       float64            `json:"confidence"`       // confidence in recommendation
	ReasoningFactors map[string]float64 `json:"reasoning_factors"` // factor -> contribution
	EstimatedTime    int                `json:"estimated_time"`   // estimated solve time in minutes
	DifficultyMatch  float64            `json:"difficulty_match"` // how well difficulty matches user
	TopicRelevance   float64            `json:"topic_relevance"`  // relevance to user's interests
	LearningValue    float64            `json:"learning_value"`   // educational value for user
}

// RecommendationRequest represents a request for recommendations
type RecommendationRequest struct {
	UserID           uuid.UUID `json:"user_id"`
	Count            int       `json:"count"`              // number of recommendations
	MaxDifficulty    *int      `json:"max_difficulty"`     // optional difficulty filter
	MinDifficulty    *int      `json:"min_difficulty"`     // optional difficulty filter
	RequiredTags     []string  `json:"required_tags"`      // optional tag filter
	ExcludeTags      []string  `json:"exclude_tags"`       // optional tag exclusion
	FocusAreas       []string  `json:"focus_areas"`        // skills to focus on
	TimeLimit        *int      `json:"time_limit"`         // max estimated solve time
	IncludeSolved    bool      `json:"include_solved"`     // include already solved problems
	RecommendationType string  `json:"recommendation_type"` // "skill_building", "challenge", "practice", "contest_prep"
}

// RecommendationResponse represents the response with recommendations
type RecommendationResponse struct {
	UserID           uuid.UUID              `json:"user_id"`
	Recommendations  []RecommendationResult `json:"recommendations"`
	TotalCount       int                    `json:"total_count"`
	ModelVersion     string                 `json:"model_version"`
	GeneratedAt      time.Time              `json:"generated_at"`
	RefreshIn        time.Duration          `json:"refresh_in"` // when to refresh recommendations
}

// ContentBasedModel represents content-based filtering model
type ContentBasedModel struct {
	ModelID         uuid.UUID              `json:"model_id"`
	ModelType       string                 `json:"model_type"` // "neural_embedding", "tfidf", "hybrid"
	Version         string                 `json:"version"`
	TopicWeights    map[string]float64     `json:"topic_weights"`
	UserEmbeddings  map[uuid.UUID][]float64 `json:"user_embeddings"`
	ProblemEmbeddings map[uuid.UUID][]float64 `json:"problem_embeddings"`
	EmbeddingDim    int                    `json:"embedding_dim"`
	TrainedAt       time.Time              `json:"trained_at"`
	Accuracy        float64                `json:"accuracy"`
	Status          string                 `json:"status"` // "training", "ready", "updating"
}

// CollaborativeModel represents collaborative filtering model
type CollaborativeModel struct {
	ModelID          uuid.UUID                `json:"model_id"`
	ModelType        string                   `json:"model_type"` // "matrix_factorization", "neural_cf"
	Version          string                   `json:"version"`
	UserFactors      map[uuid.UUID][]float64  `json:"user_factors"`
	ItemFactors      map[uuid.UUID][]float64  `json:"item_factors"`
	FactorDim        int                      `json:"factor_dim"`
	UserBiases       map[uuid.UUID]float64    `json:"user_biases"`
	ItemBiases       map[uuid.UUID]float64    `json:"item_biases"`
	GlobalBias       float64                  `json:"global_bias"`
	RegularizationL2 float64                  `json:"regularization_l2"`
	LearningRate     float64                  `json:"learning_rate"`
	TrainedAt        time.Time                `json:"trained_at"`
	RMSE             float64                  `json:"rmse"`
	Status           string                   `json:"status"`
}

// HybridModel represents the hybrid recommendation model
type HybridModel struct {
	ModelID           uuid.UUID          `json:"model_id"`
	Version           string             `json:"version"`
	ContentWeight     float64            `json:"content_weight"`
	CollaborativeWeight float64          `json:"collaborative_weight"`
	PopularityWeight  float64            `json:"popularity_weight"`
	ContentModelID    uuid.UUID          `json:"content_model_id"`
	CollaborativeModelID uuid.UUID       `json:"collaborative_model_id"`
	CombinationStrategy string           `json:"combination_strategy"` // "weighted_sum", "rank_fusion", "neural_combination"
	TrainedAt         time.Time          `json:"trained_at"`
	ValidationScore   float64            `json:"validation_score"`
	Status            string             `json:"status"`
}

// TrainingData represents data used for model training
type TrainingData struct {
	UserInteractions   []UserInteraction   `json:"user_interactions"`
	UserProfiles       []UserProfile       `json:"user_profiles"`
	ProblemFeatures    []ProblemFeatures   `json:"problem_features"`
	StartDate          time.Time           `json:"start_date"`
	EndDate            time.Time           `json:"end_date"`
	ValidationSplit    float64             `json:"validation_split"`
	TestSplit          float64             `json:"test_split"`
}

// ModelPerformanceMetrics represents performance metrics for recommendation models
type ModelPerformanceMetrics struct {
	ModelID            uuid.UUID `json:"model_id"`
	ModelType          string    `json:"model_type"`
	Precision          float64   `json:"precision"`
	Recall             float64   `json:"recall"`
	F1Score            float64   `json:"f1_score"`
	MAP                float64   `json:"map"`                // Mean Average Precision
	NDCG               float64   `json:"ndcg"`               // Normalized Discounted Cumulative Gain
	Coverage           float64   `json:"coverage"`           // catalog coverage
	Diversity          float64   `json:"diversity"`          // recommendation diversity
	Novelty            float64   `json:"novelty"`            // recommendation novelty
	UserSatisfaction   float64   `json:"user_satisfaction"`  // implicit feedback score
	ClickThroughRate   float64   `json:"click_through_rate"`
	ConversionRate     float64   `json:"conversion_rate"`    // solve rate for recommended problems
	EvaluatedAt        time.Time `json:"evaluated_at"`
}

// RecommendationCache represents cached recommendations
type RecommendationCache struct {
	UserID         uuid.UUID              `json:"user_id"`
	CacheKey       string                 `json:"cache_key"`
	Recommendations []RecommendationResult `json:"recommendations"`
	ModelVersion   string                 `json:"model_version"`
	GeneratedAt    time.Time              `json:"generated_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
	HitCount       int                    `json:"hit_count"`
}

// FeatureEngineering constants and types
const (
	// Interaction types
	InteractionView    = "view"
	InteractionAttempt = "attempt"
	InteractionSolve   = "solve"
	InteractionHint    = "hint_used"
	
	// Recommendation types
	RecommendationSkillBuilding = "skill_building"
	RecommendationChallenge     = "challenge"
	RecommendationPractice      = "practice"
	RecommendationContestPrep   = "contest_prep"
	
	// Model statuses
	ModelStatusTraining = "training"
	ModelStatusReady    = "ready"
	ModelStatusUpdating = "updating"
	ModelStatusFailed   = "failed"
	
	// Similarity types
	SimilarityTypeCosine   = "cosine"
	SimilarityTypePearson  = "pearson"
	SimilarityTypeJaccard  = "jaccard"
	SimilarityTypeContent  = "content"
	SimilarityTypeCollaborative = "collaborative"
	SimilarityTypeDifficulty = "difficulty"
)

// FeatureVector represents a feature vector for machine learning
type FeatureVector struct {
	UserID    uuid.UUID          `json:"user_id"`
	ProblemID uuid.UUID          `json:"problem_id"`
	Features  map[string]float64 `json:"features"`
	Label     float64            `json:"label"` // target variable (rating, success, etc.)
	Weight    float64            `json:"weight"` // sample weight
}

// PipelineConfig represents configuration for the data pipeline
type PipelineConfig struct {
	BatchSize               int           `json:"batch_size"`
	ProcessingInterval      time.Duration `json:"processing_interval"`
	FeatureWindow          time.Duration `json:"feature_window"`         // how far back to look for features
	MinInteractionsPerUser int           `json:"min_interactions_per_user"`
	MinUsersPerProblem     int           `json:"min_users_per_problem"`
	EnableRealtimeUpdates  bool          `json:"enable_realtime_updates"`
	CacheExpiration        time.Duration `json:"cache_expiration"`
	ModelRetrainingInterval time.Duration `json:"model_retraining_interval"`
}

// DefaultPipelineConfig returns default configuration for the pipeline
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		BatchSize:               1000,
		ProcessingInterval:      1 * time.Hour,
		FeatureWindow:          90 * 24 * time.Hour, // 90 days
		MinInteractionsPerUser: 5,
		MinUsersPerProblem:     3,
		EnableRealtimeUpdates:  true,
		CacheExpiration:        1 * time.Hour,
		ModelRetrainingInterval: 24 * time.Hour,
	}
}