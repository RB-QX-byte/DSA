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

// ContentBasedFilter implements content-based filtering using embeddings
type ContentBasedFilter struct {
	db           *database.DB
	model        *ContentBasedModel
	embeddings   *EmbeddingModel
	mu           sync.RWMutex
	isTraining   bool
	lastTrained  time.Time
}

// EmbeddingModel represents the neural embedding model
type EmbeddingModel struct {
	EmbeddingDim     int                    `json:"embedding_dim"`
	UserEmbeddings   map[uuid.UUID][]float64 `json:"user_embeddings"`
	ProblemEmbeddings map[uuid.UUID][]float64 `json:"problem_embeddings"`
	TagEmbeddings    map[string][]float64   `json:"tag_embeddings"`
	SkillEmbeddings  map[string][]float64   `json:"skill_embeddings"`
	
	// Model parameters
	LearningRate     float64 `json:"learning_rate"`
	RegularizationL2 float64 `json:"regularization_l2"`
	BatchSize        int     `json:"batch_size"`
	Epochs           int     `json:"epochs"`
	
	// Training state
	TrainingLoss     []float64 `json:"training_loss"`
	ValidationLoss   []float64 `json:"validation_loss"`
	LastUpdated      time.Time `json:"last_updated"`
}

// NewContentBasedFilter creates a new content-based filter
func NewContentBasedFilter(db *database.DB) *ContentBasedFilter {
	return &ContentBasedFilter{
		db:         db,
		embeddings: NewEmbeddingModel(128), // 128-dimensional embeddings
	}
}

// NewEmbeddingModel creates a new embedding model
func NewEmbeddingModel(embeddingDim int) *EmbeddingModel {
	return &EmbeddingModel{
		EmbeddingDim:      embeddingDim,
		UserEmbeddings:    make(map[uuid.UUID][]float64),
		ProblemEmbeddings: make(map[uuid.UUID][]float64),
		TagEmbeddings:     make(map[string][]float64),
		SkillEmbeddings:   make(map[string][]float64),
		LearningRate:      0.01,
		RegularizationL2:  0.001,
		BatchSize:         32,
		Epochs:            100,
		TrainingLoss:      make([]float64, 0),
		ValidationLoss:    make([]float64, 0),
		LastUpdated:       time.Now(),
	}
}

// Train trains the content-based model using user interactions
func (cbf *ContentBasedFilter) Train(ctx context.Context, trainingData *TrainingData) error {
	cbf.mu.Lock()
	if cbf.isTraining {
		cbf.mu.Unlock()
		return fmt.Errorf("model is already training")
	}
	cbf.isTraining = true
	cbf.mu.Unlock()

	defer func() {
		cbf.mu.Lock()
		cbf.isTraining = false
		cbf.mu.Unlock()
	}()

	fmt.Printf("Training content-based model with %d interactions\n", len(trainingData.UserInteractions))

	// Initialize embeddings for all entities
	err := cbf.initializeEmbeddings(ctx, trainingData)
	if err != nil {
		return fmt.Errorf("failed to initialize embeddings: %w", err)
	}

	// Prepare training data
	trainingSamples, validationSamples, err := cbf.prepareTrainingData(trainingData)
	if err != nil {
		return fmt.Errorf("failed to prepare training data: %w", err)
	}

	fmt.Printf("Training samples: %d, Validation samples: %d\n", len(trainingSamples), len(validationSamples))

	// Training loop
	for epoch := 0; epoch < cbf.embeddings.Epochs; epoch++ {
		// Shuffle training data
		cbf.shuffleSamples(trainingSamples)

		trainLoss := 0.0
		batches := 0

		// Process in batches
		for i := 0; i < len(trainingSamples); i += cbf.embeddings.BatchSize {
			end := i + cbf.embeddings.BatchSize
			if end > len(trainingSamples) {
				end = len(trainingSamples)
			}

			batch := trainingSamples[i:end]
			batchLoss := cbf.trainBatch(batch)
			trainLoss += batchLoss
			batches++
		}

		avgTrainLoss := trainLoss / float64(batches)
		cbf.embeddings.TrainingLoss = append(cbf.embeddings.TrainingLoss, avgTrainLoss)

		// Validation
		if len(validationSamples) > 0 {
			valLoss := cbf.evaluateBatch(validationSamples)
			cbf.embeddings.ValidationLoss = append(cbf.embeddings.ValidationLoss, valLoss)
			
			if epoch%10 == 0 {
				fmt.Printf("Epoch %d: Train Loss: %.4f, Val Loss: %.4f\n", epoch, avgTrainLoss, valLoss)
			}
		}

		// Early stopping check
		if len(cbf.embeddings.ValidationLoss) > 10 {
			recent := cbf.embeddings.ValidationLoss[len(cbf.embeddings.ValidationLoss)-10:]
			if cbf.isConverged(recent) {
				fmt.Printf("Early stopping at epoch %d\n", epoch)
				break
			}
		}
	}

	// Update model metadata
	cbf.model = &ContentBasedModel{
		ModelID:         uuid.New(),
		ModelType:       "neural_embedding",
		Version:         "1.0",
		UserEmbeddings:  cbf.embeddings.UserEmbeddings,
		ProblemEmbeddings: cbf.embeddings.ProblemEmbeddings,
		EmbeddingDim:    cbf.embeddings.EmbeddingDim,
		TrainedAt:       time.Now(),
		Status:          ModelStatusReady,
	}

	cbf.lastTrained = time.Now()
	fmt.Println("Content-based model training completed")

	return nil
}

// GetRecommendations generates content-based recommendations for a user
func (cbf *ContentBasedFilter) GetRecommendations(ctx context.Context, userID uuid.UUID, count int, excludeProblems []uuid.UUID) ([]RecommendationResult, error) {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	if cbf.model == nil || cbf.model.Status != ModelStatusReady {
		return nil, fmt.Errorf("model is not ready")
	}

	// Get user embedding
	userEmbedding, exists := cbf.embeddings.UserEmbeddings[userID]
	if !exists {
		// Generate embedding for new user
		var err error
		userEmbedding, err = cbf.generateUserEmbedding(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to generate user embedding: %w", err)
		}
	}

	// Get all problem embeddings and calculate similarities
	var candidates []struct {
		problemID uuid.UUID
		score     float64
	}

	excludeSet := make(map[uuid.UUID]bool)
	for _, pid := range excludeProblems {
		excludeSet[pid] = true
	}

	for problemID, problemEmbedding := range cbf.embeddings.ProblemEmbeddings {
		if excludeSet[problemID] {
			continue
		}

		// Calculate cosine similarity
		similarity := cbf.cosineSimilarity(userEmbedding, problemEmbedding)
		candidates = append(candidates, struct {
			problemID uuid.UUID
			score     float64
		}{problemID, similarity})
	}

	// Sort by score and take top recommendations
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if count > len(candidates) {
		count = len(candidates)
	}

	var recommendations []RecommendationResult
	for i := 0; i < count; i++ {
		candidate := candidates[i]
		
		// Get additional metadata for the recommendation
		confidence := cbf.calculateConfidence(candidate.score)
		reasoningFactors := map[string]float64{
			"content_similarity": candidate.score,
			"embedding_quality":  0.8, // This could be calculated based on training metrics
		}

		recommendation := RecommendationResult{
			ProblemID:        candidate.problemID,
			Score:            candidate.score,
			Confidence:       confidence,
			ReasoningFactors: reasoningFactors,
		}

		// Enrich with problem-specific information
		err := cbf.enrichRecommendation(ctx, &recommendation, userID)
		if err != nil {
			continue // Skip if enrichment fails
		}

		recommendations = append(recommendations, recommendation)
	}

	return recommendations, nil
}

// initializeEmbeddings initializes embeddings for all entities
func (cbf *ContentBasedFilter) initializeEmbeddings(ctx context.Context, trainingData *TrainingData) error {
	// Collect all unique entities
	userSet := make(map[uuid.UUID]bool)
	problemSet := make(map[uuid.UUID]bool)
	tagSet := make(map[string]bool)
	skillSet := make(map[string]bool)

	for _, interaction := range trainingData.UserInteractions {
		userSet[interaction.UserID] = true
		problemSet[interaction.ProblemID] = true
	}

	for _, profile := range trainingData.UserProfiles {
		userSet[profile.UserID] = true
		for skill := range profile.SkillVector {
			skillSet[skill] = true
		}
		for _, tag := range profile.PreferredTags {
			tagSet[tag] = true
		}
	}

	for _, features := range trainingData.ProblemFeatures {
		problemSet[features.ProblemID] = true
		for _, tag := range features.Tags {
			tagSet[tag] = true
		}
	}

	// Initialize random embeddings
	for userID := range userSet {
		cbf.embeddings.UserEmbeddings[userID] = cbf.randomEmbedding()
	}

	for problemID := range problemSet {
		cbf.embeddings.ProblemEmbeddings[problemID] = cbf.randomEmbedding()
	}

	for tag := range tagSet {
		cbf.embeddings.TagEmbeddings[tag] = cbf.randomEmbedding()
	}

	for skill := range skillSet {
		cbf.embeddings.SkillEmbeddings[skill] = cbf.randomEmbedding()
	}

	return nil
}

// prepareTrainingData converts interactions to training samples
func (cbf *ContentBasedFilter) prepareTrainingData(trainingData *TrainingData) ([]TrainingSample, []TrainingSample, error) {
	var samples []TrainingSample

	for _, interaction := range trainingData.UserInteractions {
		sample := TrainingSample{
			UserID:    interaction.UserID,
			ProblemID: interaction.ProblemID,
			Rating:    cbf.convertInteractionToRating(interaction),
			Weight:    1.0,
		}

		// Add negative samples for non-solved problems
		if !interaction.Success {
			sample.Rating = 0.0
		}

		samples = append(samples, sample)
	}

	// Generate negative samples
	negativeSamples := cbf.generateNegativeSamples(trainingData.UserInteractions, len(samples)/4)
	samples = append(samples, negativeSamples...)

	// Shuffle and split
	cbf.shuffleSamples(samples)
	
	splitIndex := int(float64(len(samples)) * (1.0 - trainingData.ValidationSplit))
	trainingSamples := samples[:splitIndex]
	validationSamples := samples[splitIndex:]

	return trainingSamples, validationSamples, nil
}

// TrainingSample represents a single training sample
type TrainingSample struct {
	UserID    uuid.UUID
	ProblemID uuid.UUID
	Rating    float64
	Weight    float64
}

// trainBatch trains on a batch of samples
func (cbf *ContentBasedFilter) trainBatch(batch []TrainingSample) float64 {
	totalLoss := 0.0

	for _, sample := range batch {
		loss := cbf.trainSample(sample)
		totalLoss += loss
	}

	return totalLoss / float64(len(batch))
}

// trainSample trains on a single sample using gradient descent
func (cbf *ContentBasedFilter) trainSample(sample TrainingSample) float64 {
	userEmb := cbf.embeddings.UserEmbeddings[sample.UserID]
	problemEmb := cbf.embeddings.ProblemEmbeddings[sample.ProblemID]

	// Predict rating using dot product
	predicted := cbf.dotProduct(userEmb, problemEmb)
	
	// Calculate error
	error := sample.Rating - predicted
	loss := error * error

	// Calculate gradients
	userGrad := make([]float64, len(userEmb))
	problemGrad := make([]float64, len(problemEmb))

	for i := 0; i < len(userEmb); i++ {
		userGrad[i] = -2 * error * problemEmb[i] + cbf.embeddings.RegularizationL2*userEmb[i]
		problemGrad[i] = -2 * error * userEmb[i] + cbf.embeddings.RegularizationL2*problemEmb[i]
	}

	// Update embeddings
	for i := 0; i < len(userEmb); i++ {
		userEmb[i] -= cbf.embeddings.LearningRate * userGrad[i]
		problemEmb[i] -= cbf.embeddings.LearningRate * problemGrad[i]
	}

	cbf.embeddings.UserEmbeddings[sample.UserID] = userEmb
	cbf.embeddings.ProblemEmbeddings[sample.ProblemID] = problemEmb

	return loss
}

// evaluateBatch evaluates a batch without updating parameters
func (cbf *ContentBasedFilter) evaluateBatch(batch []TrainingSample) float64 {
	totalLoss := 0.0

	for _, sample := range batch {
		userEmb := cbf.embeddings.UserEmbeddings[sample.UserID]
		problemEmb := cbf.embeddings.ProblemEmbeddings[sample.ProblemID]
		
		predicted := cbf.dotProduct(userEmb, problemEmb)
		error := sample.Rating - predicted
		loss := error * error
		totalLoss += loss
	}

	return totalLoss / float64(len(batch))
}

// Helper functions
func (cbf *ContentBasedFilter) randomEmbedding() []float64 {
	embedding := make([]float64, cbf.embeddings.EmbeddingDim)
	for i := range embedding {
		embedding[i] = rand.NormFloat64() * 0.1 // Small random values
	}
	return embedding
}

func (cbf *ContentBasedFilter) dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	
	sum := 0.0
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func (cbf *ContentBasedFilter) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	dotProd := 0.0
	normA := 0.0
	normB := 0.0

	for i := 0; i < len(a); i++ {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProd / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (cbf *ContentBasedFilter) convertInteractionToRating(interaction UserInteraction) float64 {
	if interaction.Success {
		// Positive rating based on solution quality
		if interaction.SolutionQuality > 0 {
			return interaction.SolutionQuality
		}
		return 1.0
	}
	
	// Partial rating for attempts
	return 0.2
}

func (cbf *ContentBasedFilter) generateNegativeSamples(interactions []UserInteraction, count int) []TrainingSample {
	var negativeSamples []TrainingSample
	
	// Collect user-problem pairs that actually interacted
	interacted := make(map[string]bool)
	var users []uuid.UUID
	var problems []uuid.UUID
	
	userSet := make(map[uuid.UUID]bool)
	problemSet := make(map[uuid.UUID]bool)
	
	for _, interaction := range interactions {
		key := fmt.Sprintf("%s-%s", interaction.UserID, interaction.ProblemID)
		interacted[key] = true
		
		if !userSet[interaction.UserID] {
			users = append(users, interaction.UserID)
			userSet[interaction.UserID] = true
		}
		
		if !problemSet[interaction.ProblemID] {
			problems = append(problems, interaction.ProblemID)
			problemSet[interaction.ProblemID] = true
		}
	}
	
	// Generate negative samples
	for len(negativeSamples) < count && len(users) > 0 && len(problems) > 0 {
		userID := users[rand.Intn(len(users))]
		problemID := problems[rand.Intn(len(problems))]
		
		key := fmt.Sprintf("%s-%s", userID, problemID)
		if !interacted[key] {
			negativeSamples = append(negativeSamples, TrainingSample{
				UserID:    userID,
				ProblemID: problemID,
				Rating:    0.0,
				Weight:    0.5, // Lower weight for negative samples
			})
		}
	}
	
	return negativeSamples
}

func (cbf *ContentBasedFilter) shuffleSamples(samples []TrainingSample) {
	for i := len(samples) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		samples[i], samples[j] = samples[j], samples[i]
	}
}

func (cbf *ContentBasedFilter) isConverged(losses []float64) bool {
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

func (cbf *ContentBasedFilter) generateUserEmbedding(ctx context.Context, userID uuid.UUID) ([]float64, error) {
	// For new users, generate embedding based on their profile
	// This is a simplified approach - in practice, you might use user features
	
	// Get user's solved problems and preferences
	query := `
		SELECT skill_vector, preferred_tags
		FROM user_profiles
		WHERE user_id = $1
	`
	
	var skillVector map[string]float64
	var preferredTags []string
	
	err := cbf.db.Pool.QueryRow(ctx, query, userID).Scan(&skillVector, &preferredTags)
	if err != nil {
		// Return random embedding for completely new users
		embedding := cbf.randomEmbedding()
		cbf.embeddings.UserEmbeddings[userID] = embedding
		return embedding, nil
	}
	
	// Generate embedding based on skills and preferences
	embedding := make([]float64, cbf.embeddings.EmbeddingDim)
	
	// Use skill vector to influence embedding
	skillIndex := 0
	for _, skillValue := range skillVector {
		if skillIndex < len(embedding) {
			embedding[skillIndex] = skillValue
			skillIndex++
		}
	}
	
	// Use tag embeddings to influence user embedding
	for _, tag := range preferredTags {
		if tagEmb, exists := cbf.embeddings.TagEmbeddings[tag]; exists {
			for i := 0; i < len(embedding) && i < len(tagEmb); i++ {
				embedding[i] += tagEmb[i] * 0.1 // Small influence
			}
		}
	}
	
	// Normalize embedding
	norm := 0.0
	for _, val := range embedding {
		norm += val * val
	}
	norm = math.Sqrt(norm)
	
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	
	cbf.embeddings.UserEmbeddings[userID] = embedding
	return embedding, nil
}

func (cbf *ContentBasedFilter) calculateConfidence(score float64) float64 {
	// Simple confidence calculation based on score
	// Higher scores get higher confidence
	return math.Min(score*2, 1.0)
}

func (cbf *ContentBasedFilter) enrichRecommendation(ctx context.Context, rec *RecommendationResult, userID uuid.UUID) error {
	// Get problem features to enrich the recommendation
	query := `
		SELECT difficulty, average_solve_time, complexity_score
		FROM problem_features
		WHERE problem_id = $1
	`
	
	var difficulty int
	var avgSolveTime, complexityScore float64
	
	err := cbf.db.Pool.QueryRow(ctx, query, rec.ProblemID).Scan(&difficulty, &avgSolveTime, &complexityScore)
	if err != nil {
		return err
	}
	
	// Set estimated time
	rec.EstimatedTime = int(avgSolveTime)
	
	// Calculate difficulty match (this should use user's preferred difficulty)
	// Simplified approach
	rec.DifficultyMatch = 1.0 - math.Abs(float64(difficulty-1500))/3500.0 // Assume 1500 as average preference
	
	// Set topic relevance and learning value
	rec.TopicRelevance = rec.Score // Use content similarity as topic relevance
	rec.LearningValue = complexityScore // Use complexity as learning value
	
	return nil
}

// GetModelInfo returns information about the trained model
func (cbf *ContentBasedFilter) GetModelInfo() map[string]interface{} {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()
	
	info := map[string]interface{}{
		"model_type":     "content_based_neural_embedding",
		"embedding_dim":  cbf.embeddings.EmbeddingDim,
		"is_training":    cbf.isTraining,
		"last_trained":   cbf.lastTrained,
	}
	
	if cbf.model != nil {
		info["model_id"] = cbf.model.ModelID
		info["status"] = cbf.model.Status
		info["version"] = cbf.model.Version
		info["trained_at"] = cbf.model.TrainedAt
		info["user_count"] = len(cbf.embeddings.UserEmbeddings)
		info["problem_count"] = len(cbf.embeddings.ProblemEmbeddings)
	}
	
	if len(cbf.embeddings.TrainingLoss) > 0 {
		info["final_training_loss"] = cbf.embeddings.TrainingLoss[len(cbf.embeddings.TrainingLoss)-1]
	}
	
	if len(cbf.embeddings.ValidationLoss) > 0 {
		info["final_validation_loss"] = cbf.embeddings.ValidationLoss[len(cbf.embeddings.ValidationLoss)-1]
	}
	
	return info
}