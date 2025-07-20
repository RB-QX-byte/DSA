package recommendation

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"competitive-programming-platform/pkg/database"

	"github.com/google/uuid"
)

// CollaborativeFilter implements collaborative filtering using matrix factorization
type CollaborativeFilter struct {
	db           *database.DB
	model        *CollaborativeModel
	mu           sync.RWMutex
	isTraining   bool
	lastTrained  time.Time
}

// MatrixFactorization represents the matrix factorization model
type MatrixFactorization struct {
	FactorDim        int                     `json:"factor_dim"`
	UserFactors      map[uuid.UUID][]float64 `json:"user_factors"`
	ItemFactors      map[uuid.UUID][]float64 `json:"item_factors"`
	UserBiases       map[uuid.UUID]float64   `json:"user_biases"`
	ItemBiases       map[uuid.UUID]float64   `json:"item_biases"`
	GlobalBias       float64                 `json:"global_bias"`
	
	// Training parameters
	LearningRate     float64 `json:"learning_rate"`
	RegularizationL2 float64 `json:"regularization_l2"`
	BatchSize        int     `json:"batch_size"`
	Epochs           int     `json:"epochs"`
	
	// Training state
	TrainingLoss     []float64 `json:"training_loss"`
	ValidationLoss   []float64 `json:"validation_loss"`
	LastUpdated      time.Time `json:"last_updated"`
}

// UserItemInteraction represents a user-item interaction for training
type UserItemInteraction struct {
	UserID    uuid.UUID `json:"user_id"`
	ProblemID uuid.UUID `json:"problem_id"`
	Rating    float64   `json:"rating"`
	Timestamp time.Time `json:"timestamp"`
	Weight    float64   `json:"weight"`
}

// NewCollaborativeFilter creates a new collaborative filter
func NewCollaborativeFilter(db *database.DB) *CollaborativeFilter {
	return &CollaborativeFilter{
		db: db,
	}
}

// NewMatrixFactorization creates a new matrix factorization model
func NewMatrixFactorization(factorDim int) *MatrixFactorization {
	return &MatrixFactorization{
		FactorDim:        factorDim,
		UserFactors:      make(map[uuid.UUID][]float64),
		ItemFactors:      make(map[uuid.UUID][]float64),
		UserBiases:       make(map[uuid.UUID]float64),
		ItemBiases:       make(map[uuid.UUID]float64),
		GlobalBias:       0.0,
		LearningRate:     0.01,
		RegularizationL2: 0.01,
		BatchSize:        256,
		Epochs:           100,
		TrainingLoss:     make([]float64, 0),
		ValidationLoss:   make([]float64, 0),
		LastUpdated:      time.Now(),
	}
}

// Train trains the collaborative filtering model
func (cf *CollaborativeFilter) Train(ctx context.Context, trainingData *TrainingData) error {
	cf.mu.Lock()
	if cf.isTraining {
		cf.mu.Unlock()
		return fmt.Errorf("model is already training")
	}
	cf.isTraining = true
	cf.mu.Unlock()

	defer func() {
		cf.mu.Lock()
		cf.isTraining = false
		cf.mu.Unlock()
	}()

	fmt.Printf("Training collaborative filtering model with %d interactions\n", len(trainingData.UserInteractions))

	// Initialize matrix factorization model
	mf := NewMatrixFactorization(50) // 50 latent factors

	// Convert interactions to training format
	interactions, err := cf.prepareInteractionMatrix(ctx, trainingData)
	if err != nil {
		return fmt.Errorf("failed to prepare interaction matrix: %w", err)
	}

	fmt.Printf("Prepared %d user-item interactions\n", len(interactions))

	// Initialize factors and biases
	err = cf.initializeFactors(mf, interactions)
	if err != nil {
		return fmt.Errorf("failed to initialize factors: %w", err)
	}

	// Split into training and validation sets
	trainSet, valSet := cf.splitInteractions(interactions, trainingData.ValidationSplit)

	fmt.Printf("Training set: %d, Validation set: %d\n", len(trainSet), len(valSet))

	// Training loop
	for epoch := 0; epoch < mf.Epochs; epoch++ {
		// Shuffle training data
		cf.shuffleInteractions(trainSet)

		trainLoss := 0.0
		batches := 0

		// Process in batches
		for i := 0; i < len(trainSet); i += mf.BatchSize {
			end := i + mf.BatchSize
			if end > len(trainSet) {
				end = len(trainSet)
			}

			batch := trainSet[i:end]
			batchLoss := cf.trainBatch(mf, batch)
			trainLoss += batchLoss
			batches++
		}

		avgTrainLoss := trainLoss / float64(batches)
		mf.TrainingLoss = append(mf.TrainingLoss, avgTrainLoss)

		// Validation
		if len(valSet) > 0 {
			valLoss := cf.evaluateBatch(mf, valSet)
			mf.ValidationLoss = append(mf.ValidationLoss, valLoss)

			if epoch%10 == 0 {
				fmt.Printf("Epoch %d: Train Loss: %.4f, Val Loss: %.4f\n", epoch, avgTrainLoss, valLoss)
			}
		}

		// Early stopping check
		if len(mf.ValidationLoss) > 10 {
			recent := mf.ValidationLoss[len(mf.ValidationLoss)-10:]
			if cf.isConverged(recent) {
				fmt.Printf("Early stopping at epoch %d\n", epoch)
				break
			}
		}
	}

	// Update model
	cf.model = &CollaborativeModel{
		ModelID:          uuid.New(),
		ModelType:        "matrix_factorization",
		Version:          "1.0",
		UserFactors:      mf.UserFactors,
		ItemFactors:      mf.ItemFactors,
		FactorDim:        mf.FactorDim,
		UserBiases:       mf.UserBiases,
		ItemBiases:       mf.ItemBiases,
		GlobalBias:       mf.GlobalBias,
		RegularizationL2: mf.RegularizationL2,
		LearningRate:     mf.LearningRate,
		TrainedAt:        time.Now(),
		Status:           ModelStatusReady,
	}

	if len(mf.ValidationLoss) > 0 {
		cf.model.RMSE = mf.ValidationLoss[len(mf.ValidationLoss)-1]
	}

	cf.lastTrained = time.Now()
	fmt.Println("Collaborative filtering model training completed")

	return nil
}

// GetRecommendations generates collaborative filtering recommendations for a user
func (cf *CollaborativeFilter) GetRecommendations(ctx context.Context, userID uuid.UUID, count int, excludeProblems []uuid.UUID) ([]RecommendationResult, error) {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	if cf.model == nil || cf.model.Status != ModelStatusReady {
		return nil, fmt.Errorf("model is not ready")
	}

	// Get user factors
	userFactors, userExists := cf.model.UserFactors[userID]
	userBias := cf.model.UserBiases[userID]

	if !userExists {
		// Generate recommendations for new user using item popularity
		return cf.getPopularityBasedRecommendations(ctx, count, excludeProblems)
	}

	// Get all item factors and calculate predictions
	var candidates []struct {
		problemID uuid.UUID
		score     float64
	}

	excludeSet := make(map[uuid.UUID]bool)
	for _, pid := range excludeProblems {
		excludeSet[pid] = true
	}

	for problemID, itemFactors := range cf.model.ItemFactors {
		if excludeSet[problemID] {
			continue
		}

		// Calculate predicted rating using dot product + biases
		itemBias := cf.model.ItemBiases[problemID]
		prediction := cf.model.GlobalBias + userBias + itemBias + cf.dotProduct(userFactors, itemFactors)

		candidates = append(candidates, struct {
			problemID uuid.UUID
			score     float64
		}{problemID, prediction})
	}

	// Sort by predicted score and take top recommendations
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if count > len(candidates) {
		count = len(candidates)
	}

	var recommendations []RecommendationResult
	for i := 0; i < count; i++ {
		candidate := candidates[i]

		// Calculate confidence based on prediction score and user factors magnitude
		confidence := cf.calculateConfidence(candidate.score, userFactors)
		
		reasoningFactors := map[string]float64{
			"collaborative_score": candidate.score,
			"user_bias":          userBias,
			"item_bias":          cf.model.ItemBiases[candidate.problemID],
			"global_bias":        cf.model.GlobalBias,
		}

		recommendation := RecommendationResult{
			ProblemID:        candidate.problemID,
			Score:            candidate.score,
			Confidence:       confidence,
			ReasoningFactors: reasoningFactors,
		}

		// Enrich with additional information
		err := cf.enrichRecommendation(ctx, &recommendation, userID)
		if err != nil {
			continue // Skip if enrichment fails
		}

		recommendations = append(recommendations, recommendation)
	}

	return recommendations, nil
}

// prepareInteractionMatrix converts user interactions to rating matrix format
func (cf *CollaborativeFilter) prepareInteractionMatrix(ctx context.Context, trainingData *TrainingData) ([]UserItemInteraction, error) {
	var interactions []UserItemInteraction

	// Convert user interactions to ratings
	for _, interaction := range trainingData.UserInteractions {
		rating := cf.convertInteractionToRating(interaction)
		
		// Apply time decay to older interactions
		timeDiff := time.Since(interaction.Timestamp)
		timeWeight := math.Exp(-timeDiff.Hours() / (24 * 30)) // Decay over 30 days

		interactions = append(interactions, UserItemInteraction{
			UserID:    interaction.UserID,
			ProblemID: interaction.ProblemID,
			Rating:    rating,
			Timestamp: interaction.Timestamp,
			Weight:    timeWeight,
		})
	}

	// Generate implicit feedback from submissions
	implicitInteractions, err := cf.generateImplicitFeedback(ctx)
	if err != nil {
		return interactions, nil // Continue without implicit feedback if it fails
	}

	interactions = append(interactions, implicitInteractions...)

	return interactions, nil
}

// generateImplicitFeedback creates implicit ratings from submission data
func (cf *CollaborativeFilter) generateImplicitFeedback(ctx context.Context) ([]UserItemInteraction, error) {
	query := `
		SELECT user_id, problem_id, status, created_at
		FROM submissions 
		WHERE created_at >= $1
		ORDER BY created_at DESC
	`

	cutoffTime := time.Now().Add(-90 * 24 * time.Hour) // Last 90 days
	rows, err := cf.db.Pool.Query(ctx, query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactions []UserItemInteraction
	for rows.Next() {
		var userID, problemID uuid.UUID
		var status string
		var timestamp time.Time

		err := rows.Scan(&userID, &problemID, &status, &timestamp)
		if err != nil {
			continue
		}

		// Convert submission to implicit rating
		var rating float64
		switch status {
		case "AC":
			rating = 5.0 // Solved successfully
		case "WA", "TLE", "MLE":
			rating = 2.0 // Attempted but failed
		default:
			rating = 1.0 // Other attempts
		}

		// Time decay
		timeDiff := time.Since(timestamp)
		timeWeight := math.Exp(-timeDiff.Hours() / (24 * 60)) // Decay over 60 days

		interactions = append(interactions, UserItemInteraction{
			UserID:    userID,
			ProblemID: problemID,
			Rating:    rating,
			Timestamp: timestamp,
			Weight:    timeWeight * 0.5, // Lower weight for implicit feedback
		})
	}

	return interactions, nil
}

// initializeFactors initializes user and item factors randomly
func (cf *CollaborativeFilter) initializeFactors(mf *MatrixFactorization, interactions []UserItemInteraction) error {
	// Collect unique users and items
	userSet := make(map[uuid.UUID]bool)
	itemSet := make(map[uuid.UUID]bool)
	totalRating := 0.0
	ratingCount := 0

	for _, interaction := range interactions {
		userSet[interaction.UserID] = true
		itemSet[interaction.ProblemID] = true
		totalRating += interaction.Rating
		ratingCount++
	}

	// Set global bias as mean rating
	if ratingCount > 0 {
		mf.GlobalBias = totalRating / float64(ratingCount)
	}

	// Initialize user factors and biases
	for userID := range userSet {
		mf.UserFactors[userID] = cf.randomVector(mf.FactorDim)
		mf.UserBiases[userID] = (rand.Float64() - 0.5) * 0.1
	}

	// Initialize item factors and biases
	for itemID := range itemSet {
		mf.ItemFactors[itemID] = cf.randomVector(mf.FactorDim)
		mf.ItemBiases[itemID] = (rand.Float64() - 0.5) * 0.1
	}

	return nil
}

// splitInteractions splits interactions into training and validation sets
func (cf *CollaborativeFilter) splitInteractions(interactions []UserItemInteraction, validationSplit float64) ([]UserItemInteraction, []UserItemInteraction) {
	cf.shuffleInteractions(interactions)
	
	splitIndex := int(float64(len(interactions)) * (1.0 - validationSplit))
	return interactions[:splitIndex], interactions[splitIndex:]
}

// trainBatch trains on a batch of interactions
func (cf *CollaborativeFilter) trainBatch(mf *MatrixFactorization, batch []UserItemInteraction) float64 {
	totalLoss := 0.0

	for _, interaction := range batch {
		loss := cf.trainSingleInteraction(mf, interaction)
		totalLoss += loss * interaction.Weight
	}

	return totalLoss / float64(len(batch))
}

// trainSingleInteraction trains on a single user-item interaction
func (cf *CollaborativeFilter) trainSingleInteraction(mf *MatrixFactorization, interaction UserItemInteraction) float64 {
	userFactors := mf.UserFactors[interaction.UserID]
	itemFactors := mf.ItemFactors[interaction.ProblemID]
	userBias := mf.UserBiases[interaction.UserID]
	itemBias := mf.ItemBiases[interaction.ProblemID]

	// Predict rating
	prediction := mf.GlobalBias + userBias + itemBias + cf.dotProduct(userFactors, itemFactors)
	
	// Calculate error
	error := interaction.Rating - prediction
	loss := error * error

	// Calculate gradients
	userGrad := make([]float64, len(userFactors))
	itemGrad := make([]float64, len(itemFactors))

	for i := 0; i < len(userFactors); i++ {
		userGrad[i] = -2*error*itemFactors[i] + mf.RegularizationL2*userFactors[i]
		itemGrad[i] = -2*error*userFactors[i] + mf.RegularizationL2*itemFactors[i]
	}

	userBiasGrad := -2*error + mf.RegularizationL2*userBias
	itemBiasGrad := -2*error + mf.RegularizationL2*itemBias

	// Update parameters
	for i := 0; i < len(userFactors); i++ {
		userFactors[i] -= mf.LearningRate * userGrad[i]
		itemFactors[i] -= mf.LearningRate * itemGrad[i]
	}

	mf.UserBiases[interaction.UserID] = userBias - mf.LearningRate*userBiasGrad
	mf.ItemBiases[interaction.ProblemID] = itemBias - mf.LearningRate*itemBiasGrad

	return loss
}

// evaluateBatch evaluates a batch without updating parameters
func (cf *CollaborativeFilter) evaluateBatch(mf *MatrixFactorization, batch []UserItemInteraction) float64 {
	totalLoss := 0.0
	totalWeight := 0.0

	for _, interaction := range batch {
		userFactors := mf.UserFactors[interaction.UserID]
		itemFactors := mf.ItemFactors[interaction.ProblemID]
		userBias := mf.UserBiases[interaction.UserID]
		itemBias := mf.ItemBiases[interaction.ProblemID]

		prediction := mf.GlobalBias + userBias + itemBias + cf.dotProduct(userFactors, itemFactors)
		error := interaction.Rating - prediction
		loss := error * error

		totalLoss += loss * interaction.Weight
		totalWeight += interaction.Weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return math.Sqrt(totalLoss / totalWeight) // RMSE
}

// getPopularityBasedRecommendations provides fallback recommendations for new users
func (cf *CollaborativeFilter) getPopularityBasedRecommendations(ctx context.Context, count int, excludeProblems []uuid.UUID) ([]RecommendationResult, error) {
	excludeSet := make(map[uuid.UUID]bool)
	for _, pid := range excludeProblems {
		excludeSet[pid] = true
	}

	// Get popular problems based on positive interactions
	query := `
		SELECT p.id, p.title, p.difficulty, COUNT(*) as interaction_count,
		       AVG(CASE WHEN ui.success THEN 1.0 ELSE 0.0 END) as success_rate
		FROM problems p
		LEFT JOIN user_interactions ui ON p.id = ui.problem_id
		WHERE ui.timestamp >= $1
		GROUP BY p.id, p.title, p.difficulty
		HAVING COUNT(*) >= 5
		ORDER BY interaction_count DESC, success_rate DESC
		LIMIT $2
	`

	cutoffTime := time.Now().Add(-30 * 24 * time.Hour)
	rows, err := cf.db.Pool.Query(ctx, query, cutoffTime, count*2) // Get more than needed for filtering
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recommendations []RecommendationResult
	for rows.Next() && len(recommendations) < count {
		var problemID uuid.UUID
		var title string
		var difficulty int
		var interactionCount int
		var successRate float64

		err := rows.Scan(&problemID, &title, &difficulty, &interactionCount, &successRate)
		if err != nil {
			continue
		}

		if excludeSet[problemID] {
			continue
		}

		// Score based on popularity and success rate
		score := math.Log(float64(interactionCount)) * successRate

		recommendation := RecommendationResult{
			ProblemID:  problemID,
			Score:      score,
			Confidence: 0.5, // Lower confidence for popularity-based
			ReasoningFactors: map[string]float64{
				"popularity_score": score,
				"interaction_count": float64(interactionCount),
				"success_rate": successRate,
			},
		}

		recommendations = append(recommendations, recommendation)
	}

	return recommendations, nil
}

// Helper functions
func (cf *CollaborativeFilter) randomVector(dim int) []float64 {
	vector := make([]float64, dim)
	for i := range vector {
		vector[i] = (rand.Float64() - 0.5) * 0.1 // Small random values
	}
	return vector
}

func (cf *CollaborativeFilter) dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	
	sum := 0.0
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func (cf *CollaborativeFilter) convertInteractionToRating(interaction UserInteraction) float64 {
	baseRating := 1.0 // Base rating for any interaction

	switch interaction.InteractionType {
	case InteractionView:
		return baseRating
	case InteractionAttempt:
		if interaction.Success {
			// Factor in solution quality if available
			if interaction.SolutionQuality > 0 {
				return 3.0 + 2.0*interaction.SolutionQuality // 3.0 to 5.0 based on quality
			}
			return 4.0 // Standard success rating
		}
		return 2.0 // Failed attempt
	case InteractionSolve:
		return 5.0 // Maximum rating for solving
	case InteractionHint:
		return 1.5 // Slight positive for seeking help
	default:
		return baseRating
	}
}

func (cf *CollaborativeFilter) shuffleInteractions(interactions []UserItemInteraction) {
	for i := len(interactions) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		interactions[i], interactions[j] = interactions[j], interactions[i]
	}
}

func (cf *CollaborativeFilter) isConverged(losses []float64) bool {
	if len(losses) < 5 {
		return false
	}
	
	// Check if loss hasn't improved in last 5 epochs
	threshold := 0.001
	recent := losses[len(losses)-5:]
	
	for i := 1; i < len(recent); i++ {
		if recent[i] < recent[i-1]-threshold {
			return false // Still improving
		}
	}
	
	return true // Converged
}

func (cf *CollaborativeFilter) calculateConfidence(score float64, userFactors []float64) float64 {
	// Base confidence from score
	scoreConfidence := math.Min(math.Abs(score)/5.0, 1.0)
	
	// Factor confidence from user factor magnitude (higher magnitude = more established user)
	factorMagnitude := 0.0
	for _, factor := range userFactors {
		factorMagnitude += factor * factor
	}
	factorMagnitude = math.Sqrt(factorMagnitude)
	factorConfidence := math.Min(factorMagnitude/2.0, 1.0)
	
	return (scoreConfidence + factorConfidence) / 2.0
}

func (cf *CollaborativeFilter) enrichRecommendation(ctx context.Context, rec *RecommendationResult, userID uuid.UUID) error {
	// Get problem information
	query := `
		SELECT p.difficulty, pf.average_solve_time, pf.complexity_score
		FROM problems p
		LEFT JOIN problem_features pf ON p.id = pf.problem_id
		WHERE p.id = $1
	`
	
	var difficulty int
	var avgSolveTime, complexityScore *float64
	
	err := cf.db.Pool.QueryRow(ctx, query, rec.ProblemID).Scan(&difficulty, &avgSolveTime, &complexityScore)
	if err != nil {
		return err
	}
	
	// Set estimated time
	if avgSolveTime != nil {
		rec.EstimatedTime = int(*avgSolveTime)
	} else {
		rec.EstimatedTime = 30 // Default 30 minutes
	}
	
	// Calculate difficulty match (simplified)
	rec.DifficultyMatch = 1.0 - math.Abs(float64(difficulty-1500))/3500.0
	
	// Set learning value
	if complexityScore != nil {
		rec.LearningValue = *complexityScore
	} else {
		rec.LearningValue = 0.5
	}
	
	// Topic relevance is set based on collaborative score
	rec.TopicRelevance = math.Min(rec.Score/5.0, 1.0)
	
	return nil
}

// GetModelInfo returns information about the trained model
func (cf *CollaborativeFilter) GetModelInfo() map[string]interface{} {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	
	info := map[string]interface{}{
		"model_type":     "collaborative_filtering_matrix_factorization",
		"is_training":    cf.isTraining,
		"last_trained":   cf.lastTrained,
	}
	
	if cf.model != nil {
		info["model_id"] = cf.model.ModelID
		info["status"] = cf.model.Status
		info["version"] = cf.model.Version
		info["trained_at"] = cf.model.TrainedAt
		info["factor_dim"] = cf.model.FactorDim
		info["user_count"] = len(cf.model.UserFactors)
		info["item_count"] = len(cf.model.ItemFactors)
		info["global_bias"] = cf.model.GlobalBias
		info["rmse"] = cf.model.RMSE
	}
	
	return info
}