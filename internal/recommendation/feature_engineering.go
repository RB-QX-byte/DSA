package recommendation

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"competitive-programming-platform/internal/analytics"
	"competitive-programming-platform/pkg/database"

	"github.com/google/uuid"
)

// FeatureEngineer handles feature extraction and engineering for recommendation models
type FeatureEngineer struct {
	db     *database.DB
	config *PipelineConfig
}

// NewFeatureEngineer creates a new feature engineer
func NewFeatureEngineer(db *database.DB, config *PipelineConfig) *FeatureEngineer {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	return &FeatureEngineer{
		db:     db,
		config: config,
	}
}

// ExtractUserProfile extracts and builds a comprehensive user profile
func (fe *FeatureEngineer) ExtractUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	profile := &UserProfile{
		UserID:         userID,
		SkillVector:    make(map[string]float64),
		ActivityPattern: make(map[string]float64),
		UpdatedAt:      time.Now(),
	}

	// Extract user interactions
	interactions, err := fe.getUserInteractions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user interactions: %w", err)
	}

	// Extract solved and attempted problems
	profile.SolvedProblems, profile.AttemptedProblems = fe.extractUserProblemHistory(interactions)

	// Extract skill vector from analytics data
	skillVector, err := fe.extractSkillVectorFromAnalytics(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract skill vector: %w", err)
	}
	profile.SkillVector = skillVector

	// Extract preferences
	profile.PreferredDifficulty = fe.extractPreferredDifficulty(interactions)
	profile.PreferredTags = fe.extractPreferredTags(ctx, profile.SolvedProblems)
	profile.PreferredLanguages = fe.extractPreferredLanguages(interactions)

	// Extract weak areas
	profile.WeakAreas = fe.identifyWeakAreas(profile.SkillVector)

	// Extract activity patterns
	profile.ActivityPattern = fe.extractActivityPattern(interactions)

	// Set last active time
	if len(interactions) > 0 {
		profile.LastActive = interactions[0].Timestamp
	}

	return profile, nil
}

// ExtractProblemFeatures extracts comprehensive features for a problem
func (fe *FeatureEngineer) ExtractProblemFeatures(ctx context.Context, problemID uuid.UUID) (*ProblemFeatures, error) {
	features := &ProblemFeatures{
		ProblemID:   problemID,
		TopicVector: make(map[string]float64),
		UpdatedAt:   time.Now(),
	}

	// Get basic problem information
	err := fe.extractBasicProblemInfo(ctx, features)
	if err != nil {
		return nil, fmt.Errorf("failed to extract basic problem info: %w", err)
	}

	// Calculate problem statistics
	err = fe.calculateProblemStatistics(ctx, features)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate problem statistics: %w", err)
	}

	// Extract topic vector from tags and description
	features.TopicVector = fe.extractTopicVector(features.Tags, features.Title)

	// Calculate complexity and popularity scores
	features.ComplexityScore = fe.calculateComplexityScore(features)
	features.PopularityScore = fe.calculatePopularityScore(features)

	// Find similar problems and prerequisites
	features.SimilarProblems, err = fe.findSimilarProblems(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar problems: %w", err)
	}

	features.Prerequisites, err = fe.identifyPrerequisites(ctx, features)
	if err != nil {
		return nil, fmt.Errorf("failed to identify prerequisites: %w", err)
	}

	return features, nil
}

// CreateFeatureVectors creates feature vectors for training
func (fe *FeatureEngineer) CreateFeatureVectors(ctx context.Context, startDate, endDate time.Time) ([]FeatureVector, error) {
	var vectors []FeatureVector

	// Get all user interactions in the time window
	query := `
		SELECT user_id, problem_id, interaction_type, duration, success, 
		       attempt_count, language_used, solution_quality, difficulty_rating, timestamp
		FROM user_interactions 
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp
	`

	rows, err := fe.db.Pool.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query interactions: %w", err)
	}
	defer rows.Close()

	userProfiles := make(map[uuid.UUID]*UserProfile)
	problemFeatures := make(map[uuid.UUID]*ProblemFeatures)

	for rows.Next() {
		var interaction UserInteraction
		err := rows.Scan(
			&interaction.UserID, &interaction.ProblemID, &interaction.InteractionType,
			&interaction.Duration, &interaction.Success, &interaction.AttemptCount,
			&interaction.LanguageUsed, &interaction.SolutionQuality,
			&interaction.DifficultyRating, &interaction.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan interaction: %w", err)
		}

		// Get or create user profile
		if _, exists := userProfiles[interaction.UserID]; !exists {
			profile, err := fe.ExtractUserProfile(ctx, interaction.UserID)
			if err != nil {
				continue // Skip if profile extraction fails
			}
			userProfiles[interaction.UserID] = profile
		}

		// Get or create problem features
		if _, exists := problemFeatures[interaction.ProblemID]; !exists {
			features, err := fe.ExtractProblemFeatures(ctx, interaction.ProblemID)
			if err != nil {
				continue // Skip if feature extraction fails
			}
			problemFeatures[interaction.ProblemID] = features
		}

		// Create feature vector
		vector := fe.createFeatureVector(interaction, userProfiles[interaction.UserID], problemFeatures[interaction.ProblemID])
		vectors = append(vectors, vector)
	}

	return vectors, nil
}

// getUserInteractions retrieves user interactions from the database
func (fe *FeatureEngineer) getUserInteractions(ctx context.Context, userID uuid.UUID) ([]UserInteraction, error) {
	cutoffTime := time.Now().Add(-fe.config.FeatureWindow)
	query := `
		SELECT id, user_id, problem_id, interaction_type, duration, success,
		       attempt_count, language_used, solution_quality, difficulty_rating, timestamp
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp >= $2
		ORDER BY timestamp DESC
	`

	rows, err := fe.db.Pool.Query(ctx, query, userID, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactions []UserInteraction
	for rows.Next() {
		var interaction UserInteraction
		err := rows.Scan(
			&interaction.ID, &interaction.UserID, &interaction.ProblemID,
			&interaction.InteractionType, &interaction.Duration, &interaction.Success,
			&interaction.AttemptCount, &interaction.LanguageUsed,
			&interaction.SolutionQuality, &interaction.DifficultyRating, &interaction.Timestamp,
		)
		if err != nil {
			return nil, err
		}
		interactions = append(interactions, interaction)
	}

	return interactions, nil
}

// extractUserProblemHistory extracts solved and attempted problems
func (fe *FeatureEngineer) extractUserProblemHistory(interactions []UserInteraction) ([]uuid.UUID, []uuid.UUID) {
	solvedMap := make(map[uuid.UUID]bool)
	attemptedMap := make(map[uuid.UUID]bool)

	for _, interaction := range interactions {
		attemptedMap[interaction.ProblemID] = true
		if interaction.Success {
			solvedMap[interaction.ProblemID] = true
		}
	}

	var solved, attempted []uuid.UUID
	for problemID := range solvedMap {
		solved = append(solved, problemID)
	}
	for problemID := range attemptedMap {
		attempted = append(attempted, problemID)
	}

	return solved, attempted
}

// extractSkillVectorFromAnalytics extracts skill vector from analytics data
func (fe *FeatureEngineer) extractSkillVectorFromAnalytics(ctx context.Context, userID uuid.UUID) (map[string]float64, error) {
	skillVector := make(map[string]float64)

	// Initialize with default values
	for _, skill := range analytics.SkillCategories {
		skillVector[skill] = 0.5 // Default neutral skill level
	}

	// Query analytics skill data
	query := `
		SELECT skill_category, skill_value, confidence
		FROM user_skill_assessments 
		WHERE user_id = $1 AND updated_at >= $2
		ORDER BY updated_at DESC
	`

	cutoffTime := time.Now().Add(-30 * 24 * time.Hour) // Last 30 days
	rows, err := fe.db.Pool.Query(ctx, query, userID, cutoffTime)
	if err != nil {
		return skillVector, nil // Return defaults if query fails
	}
	defer rows.Close()

	for rows.Next() {
		var skill string
		var value, confidence float64
		if err := rows.Scan(&skill, &value, &confidence); err != nil {
			continue
		}
		// Weight by confidence
		skillVector[skill] = value * confidence
	}

	return skillVector, nil
}

// extractPreferredDifficulty calculates user's preferred difficulty range
func (fe *FeatureEngineer) extractPreferredDifficulty(interactions []UserInteraction) [2]int {
	if len(interactions) == 0 {
		return [2]int{800, 1200} // Default range for beginners
	}

	var difficulties []float64
	for _, interaction := range interactions {
		if interaction.Success {
			difficulties = append(difficulties, interaction.DifficultyRating)
		}
	}

	if len(difficulties) == 0 {
		return [2]int{800, 1200}
	}

	sort.Float64s(difficulties)
	p25 := difficulties[int(float64(len(difficulties))*0.25)]
	p75 := difficulties[int(float64(len(difficulties))*0.75)]

	return [2]int{int(p25), int(p75)}
}

// extractPreferredTags identifies user's preferred problem tags
func (fe *FeatureEngineer) extractPreferredTags(ctx context.Context, solvedProblems []uuid.UUID) []string {
	if len(solvedProblems) == 0 {
		return []string{}
	}

	tagCounts := make(map[string]int)
	
	// Build placeholder string for IN clause
	placeholders := make([]string, len(solvedProblems))
	args := make([]interface{}, len(solvedProblems))
	for i, id := range solvedProblems {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT unnest(tags) as tag, COUNT(*) as count
		FROM problems 
		WHERE id IN (%s)
		GROUP BY tag
		ORDER BY count DESC
		LIMIT 10
	`, fmt.Sprintf("%s", placeholders))

	rows, err := fe.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var preferredTags []string
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			continue
		}
		preferredTags = append(preferredTags, tag)
	}

	return preferredTags
}

// extractPreferredLanguages identifies user's preferred programming languages
func (fe *FeatureEngineer) extractPreferredLanguages(interactions []UserInteraction) []string {
	langCounts := make(map[string]int)
	for _, interaction := range interactions {
		if interaction.LanguageUsed != "" {
			langCounts[interaction.LanguageUsed]++
		}
	}

	type langCount struct {
		lang  string
		count int
	}
	var langPairs []langCount
	for lang, count := range langCounts {
		langPairs = append(langPairs, langCount{lang, count})
	}

	sort.Slice(langPairs, func(i, j int) bool {
		return langPairs[i].count > langPairs[j].count
	})

	var preferred []string
	for i, pair := range langPairs {
		if i >= 3 { // Top 3 languages
			break
		}
		preferred = append(preferred, pair.lang)
	}

	return preferred
}

// identifyWeakAreas identifies areas where the user needs improvement
func (fe *FeatureEngineer) identifyWeakAreas(skillVector map[string]float64) []string {
	type skillPair struct {
		skill string
		value float64
	}

	var skills []skillPair
	for skill, value := range skillVector {
		skills = append(skills, skillPair{skill, value})
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].value < skills[j].value
	})

	var weakAreas []string
	threshold := 0.3 // Skills below this threshold are considered weak
	for _, skill := range skills {
		if skill.value < threshold {
			weakAreas = append(weakAreas, skill.skill)
		}
		if len(weakAreas) >= 5 { // Limit to top 5 weak areas
			break
		}
	}

	return weakAreas
}

// extractActivityPattern analyzes user's activity patterns
func (fe *FeatureEngineer) extractActivityPattern(interactions []UserInteraction) map[string]float64 {
	pattern := make(map[string]float64)
	
	// Initialize hourly buckets
	for i := 0; i < 24; i++ {
		pattern[fmt.Sprintf("hour_%d", i)] = 0.0
	}

	// Initialize day of week buckets
	days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for _, day := range days {
		pattern[day] = 0.0
	}

	if len(interactions) == 0 {
		return pattern
	}

	// Count interactions by hour and day
	for _, interaction := range interactions {
		hour := interaction.Timestamp.Hour()
		pattern[fmt.Sprintf("hour_%d", hour)]++
		
		dayOfWeek := interaction.Timestamp.Weekday()
		pattern[days[int(dayOfWeek)]]++
	}

	// Normalize to probabilities
	total := float64(len(interactions))
	for key, count := range pattern {
		pattern[key] = count / total
	}

	return pattern
}

// extractBasicProblemInfo extracts basic problem information
func (fe *FeatureEngineer) extractBasicProblemInfo(ctx context.Context, features *ProblemFeatures) error {
	query := `
		SELECT title, difficulty, tags, acceptance_rate, total_submissions, accepted_submissions
		FROM problems 
		WHERE id = $1
	`

	var totalSubs, acceptedSubs int
	err := fe.db.Pool.QueryRow(ctx, query, features.ProblemID).Scan(
		&features.Title, &features.Difficulty, &features.Tags,
		&features.AcceptanceRate, &totalSubs, &acceptedSubs,
	)
	if err != nil {
		return err
	}

	// Calculate acceptance rate if not already set
	if totalSubs > 0 && features.AcceptanceRate == 0 {
		features.AcceptanceRate = float64(acceptedSubs) / float64(totalSubs)
	}

	return nil
}

// calculateProblemStatistics calculates problem statistics
func (fe *FeatureEngineer) calculateProblemStatistics(ctx context.Context, features *ProblemFeatures) error {
	// Calculate average attempts and solve time
	query := `
		SELECT 
			AVG(attempt_count) as avg_attempts,
			AVG(duration) as avg_solve_time
		FROM user_interactions 
		WHERE problem_id = $1 AND success = true
	`

	err := fe.db.Pool.QueryRow(ctx, query, features.ProblemID).Scan(
		&features.AverageAttempts, &features.AverageSolveTime,
	)
	if err != nil {
		// Set defaults if no data
		features.AverageAttempts = 2.0
		features.AverageSolveTime = 30.0 // 30 minutes default
	} else {
		// Convert seconds to minutes
		features.AverageSolveTime = features.AverageSolveTime / 60.0
	}

	return nil
}

// extractTopicVector creates a topic vector from tags and title
func (fe *FeatureEngineer) extractTopicVector(tags []string, title string) map[string]float64 {
	vector := make(map[string]float64)

	// Weight tags highly
	for _, tag := range tags {
		vector[tag] = 1.0
	}

	// Add algorithm/data structure keywords from title (simplified approach)
	keywords := []string{
		"array", "string", "tree", "graph", "sort", "search", "dynamic",
		"greedy", "binary", "hash", "stack", "queue", "heap", "math",
		"geometry", "number", "combinatorics", "probability", "game",
	}

	titleLower := fmt.Sprintf("%s %s", title, fmt.Sprintf("%v", tags))
	for _, keyword := range keywords {
		if contains(titleLower, keyword) {
			vector[keyword] = 0.5
		}
	}

	return vector
}

// calculateComplexityScore calculates a complexity score for the problem
func (fe *FeatureEngineer) calculateComplexityScore(features *ProblemFeatures) float64 {
	// Base complexity from difficulty
	baseScore := float64(features.Difficulty) / 3500.0

	// Adjust based on acceptance rate (lower acceptance = higher complexity)
	acceptanceAdjustment := 1.0 - features.AcceptanceRate
	
	// Adjust based on average attempts
	attemptsAdjustment := math.Min(features.AverageAttempts/10.0, 1.0)

	// Combine factors
	complexity := (baseScore*0.5 + acceptanceAdjustment*0.3 + attemptsAdjustment*0.2)
	return math.Max(0.0, math.Min(1.0, complexity))
}

// calculatePopularityScore calculates a popularity score for the problem
func (fe *FeatureEngineer) calculatePopularityScore(features *ProblemFeatures) float64 {
	// Simple popularity based on acceptance rate and submission count
	// In a real system, this could include views, bookmarks, etc.
	submissionScore := math.Log(float64(features.TotalSubmissions + 1)) / 10.0
	acceptanceScore := features.AcceptanceRate
	
	popularity := (submissionScore*0.6 + acceptanceScore*0.4)
	return math.Max(0.0, math.Min(1.0, popularity))
}

// findSimilarProblems finds problems similar to the given problem
func (fe *FeatureEngineer) findSimilarProblems(ctx context.Context, problemID uuid.UUID) ([]uuid.UUID, error) {
	// This is a simplified implementation
	// In practice, you'd use more sophisticated similarity measures
	query := `
		SELECT id 
		FROM problems 
		WHERE id != $1 
		AND difficulty BETWEEN (
			SELECT difficulty - 200 FROM problems WHERE id = $1
		) AND (
			SELECT difficulty + 200 FROM problems WHERE id = $1
		)
		ORDER BY RANDOM()
		LIMIT 5
	`

	rows, err := fe.db.Pool.Query(ctx, query, problemID)
	if err != nil {
		return []uuid.UUID{}, nil // Return empty if query fails
	}
	defer rows.Close()

	var similar []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		similar = append(similar, id)
	}

	return similar, nil
}

// identifyPrerequisites identifies prerequisite problems
func (fe *FeatureEngineer) identifyPrerequisites(ctx context.Context, features *ProblemFeatures) ([]uuid.UUID, error) {
	// Simplified: find easier problems with similar tags
	if len(features.Tags) == 0 {
		return []uuid.UUID{}, nil
	}

	query := `
		SELECT id 
		FROM problems 
		WHERE difficulty < $1 
		AND tags && $2 
		ORDER BY difficulty DESC
		LIMIT 3
	`

	rows, err := fe.db.Pool.Query(ctx, query, features.Difficulty, features.Tags)
	if err != nil {
		return []uuid.UUID{}, nil
	}
	defer rows.Close()

	var prerequisites []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		prerequisites = append(prerequisites, id)
	}

	return prerequisites, nil
}

// createFeatureVector creates a feature vector for training
func (fe *FeatureEngineer) createFeatureVector(interaction UserInteraction, userProfile *UserProfile, problemFeatures *ProblemFeatures) FeatureVector {
	features := make(map[string]float64)

	// User features
	for skill, value := range userProfile.SkillVector {
		features[fmt.Sprintf("user_skill_%s", skill)] = value
	}

	// Problem features
	features["problem_difficulty"] = float64(problemFeatures.Difficulty) / 3500.0
	features["problem_acceptance_rate"] = problemFeatures.AcceptanceRate
	features["problem_complexity"] = problemFeatures.ComplexityScore
	features["problem_popularity"] = problemFeatures.PopularityScore

	// Interaction features
	features["duration_normalized"] = math.Min(float64(interaction.Duration)/3600.0, 2.0) // Cap at 2 hours
	features["attempt_count"] = math.Min(float64(interaction.AttemptCount), 10.0) // Cap at 10

	// Difficulty match feature
	userPrefDiff := float64(userProfile.PreferredDifficulty[0]+userProfile.PreferredDifficulty[1]) / 2.0
	difficultyMatch := 1.0 - math.Abs(float64(problemFeatures.Difficulty)-userPrefDiff)/3500.0
	features["difficulty_match"] = math.Max(0.0, difficultyMatch)

	// Tag overlap feature
	tagOverlap := fe.calculateTagOverlap(userProfile.PreferredTags, problemFeatures.Tags)
	features["tag_overlap"] = tagOverlap

	// Create target label
	var label float64
	if interaction.Success {
		label = 1.0
	} else {
		label = 0.0
	}

	// Adjust label based on solution quality if available
	if interaction.SolutionQuality > 0 {
		label = interaction.SolutionQuality
	}

	return FeatureVector{
		UserID:    interaction.UserID,
		ProblemID: interaction.ProblemID,
		Features:  features,
		Label:     label,
		Weight:    1.0, // Default weight
	}
}

// calculateTagOverlap calculates overlap between user preferred tags and problem tags
func (fe *FeatureEngineer) calculateTagOverlap(userTags, problemTags []string) float64 {
	if len(userTags) == 0 || len(problemTags) == 0 {
		return 0.0
	}

	userTagSet := make(map[string]bool)
	for _, tag := range userTags {
		userTagSet[tag] = true
	}

	overlap := 0
	for _, tag := range problemTags {
		if userTagSet[tag] {
			overlap++
		}
	}

	return float64(overlap) / float64(len(userTags))
}

// contains checks if a string contains a substring (case-insensitive)
func contains(text, substr string) bool {
	return len(text) >= len(substr) && 
		   (text == substr || 
		    len(text) > len(substr) && 
			(text[:len(substr)] == substr || 
			 text[len(text)-len(substr):] == substr ||
			 containsMiddle(text, substr)))
}

func containsMiddle(text, substr string) bool {
	for i := 1; i < len(text)-len(substr)+1; i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}