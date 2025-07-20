package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// BayesianSkillModel implements Bayesian inference for skill progression
type BayesianSkillModel struct {
	config *BayesianParameters
}

// NewBayesianSkillModel creates a new Bayesian skill progression model
func NewBayesianSkillModel(config *BayesianParameters) *BayesianSkillModel {
	if config == nil {
		config = DefaultBayesianParameters()
	}
	return &BayesianSkillModel{config: config}
}

// SkillEvidence represents evidence for skill level estimation
type SkillEvidence struct {
	UserID       uuid.UUID `json:"user_id"`
	SkillCategory string   `json:"skill_category"`
	Outcome      float64   `json:"outcome"`      // 0.0 to 1.0 (failure to success)
	Confidence   float64   `json:"confidence"`   // 0.0 to 1.0 (how confident we are in this evidence)
	Timestamp    time.Time `json:"timestamp"`
	Context      map[string]interface{} `json:"context,omitempty"` // Additional context
}

// SkillEstimate represents the current skill estimate with uncertainty
type SkillEstimate struct {
	UserID                  uuid.UUID `json:"user_id"`
	SkillCategory           string    `json:"skill_category"`
	Mean                    float64   `json:"mean"`                    // Estimated skill level
	Variance                float64   `json:"variance"`                // Uncertainty in estimate
	ConfidenceIntervalLower float64   `json:"confidence_interval_lower"`
	ConfidenceIntervalUpper float64   `json:"confidence_interval_upper"`
	Alpha                   float64   `json:"alpha"`                   // Beta distribution parameter
	Beta                    float64   `json:"beta"`                    // Beta distribution parameter
	EvidenceCount           int       `json:"evidence_count"`
	LastUpdated             time.Time `json:"last_updated"`
}

// UpdateSkillEstimate updates a user's skill estimate with new evidence using Bayesian inference
func (bsm *BayesianSkillModel) UpdateSkillEstimate(
	ctx context.Context, 
	currentEstimate *SkillEstimate, 
	evidence *SkillEvidence,
) (*SkillEstimate, error) {
	
	// Initialize with priors if no current estimate
	if currentEstimate == nil {
		currentEstimate = &SkillEstimate{
			UserID:        evidence.UserID,
			SkillCategory: evidence.SkillCategory,
			Alpha:         bsm.config.PriorAlpha,
			Beta:          bsm.config.PriorBeta,
			EvidenceCount: 0,
		}
	}

	// Apply time decay to existing estimate
	timeSinceUpdate := evidence.Timestamp.Sub(currentEstimate.LastUpdated)
	decayFactor := math.Pow(bsm.config.DecayFactor, timeSinceUpdate.Hours()/24.0) // Daily decay
	
	// Decay the confidence in previous evidence
	currentEstimate.Alpha = bsm.config.PriorAlpha + (currentEstimate.Alpha-bsm.config.PriorAlpha)*decayFactor
	currentEstimate.Beta = bsm.config.PriorBeta + (currentEstimate.Beta-bsm.config.PriorBeta)*decayFactor

	// Update with new evidence using Beta-Binomial conjugate prior
	// Treat evidence as success/failure with given confidence
	successEvidence := evidence.Outcome * evidence.Confidence
	failureEvidence := (1.0 - evidence.Outcome) * evidence.Confidence
	
	// Apply learning rate
	successEvidence *= bsm.config.LearningRate
	failureEvidence *= bsm.config.LearningRate

	newAlpha := currentEstimate.Alpha + successEvidence
	newBeta := currentEstimate.Beta + failureEvidence

	// Calculate new estimates
	newMean := newAlpha / (newAlpha + newBeta)
	newVariance := (newAlpha * newBeta) / (math.Pow(newAlpha+newBeta, 2) * (newAlpha + newBeta + 1))
	
	// Calculate 95% confidence interval using Beta distribution approximation
	stdDev := math.Sqrt(newVariance)
	lowerCI := math.Max(0.0, newMean-1.96*stdDev)
	upperCI := math.Min(1.0, newMean+1.96*stdDev)

	return &SkillEstimate{
		UserID:                  evidence.UserID,
		SkillCategory:           evidence.SkillCategory,
		Mean:                    newMean,
		Variance:                newVariance,
		ConfidenceIntervalLower: lowerCI,
		ConfidenceIntervalUpper: upperCI,
		Alpha:                   newAlpha,
		Beta:                    newBeta,
		EvidenceCount:           currentEstimate.EvidenceCount + 1,
		LastUpdated:             evidence.Timestamp,
	}, nil
}

// ExtractEvidenceFromSubmission extracts skill evidence from submission data
func (bsm *BayesianSkillModel) ExtractEvidenceFromSubmission(
	ctx context.Context,
	submissionData *SubmissionEventData,
	userID uuid.UUID,
	timestamp time.Time,
) ([]*SkillEvidence, error) {

	var evidences []*SkillEvidence

	// Problem solving speed evidence
	if submissionData.ExecutionTime != nil && submissionData.Status == "AC" {
		// Faster execution (relatively) suggests better problem solving speed
		// This is a simplified heuristic - in practice, you'd normalize against problem difficulty
		speedScore := 1.0
		if *submissionData.ExecutionTime > 1000 { // > 1 second suggests slower approach
			speedScore = math.Max(0.1, 1.0-float64(*submissionData.ExecutionTime)/10000.0)
		}
		
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "problem_solving_speed",
			Outcome:      speedScore,
			Confidence:   0.8,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"execution_time": *submissionData.ExecutionTime,
				"status":         submissionData.Status,
			},
		})
	}

	// Debugging efficiency evidence
	if submissionData.TestCasesPassed > 0 {
		debugScore := float64(submissionData.TestCasesPassed) / float64(submissionData.TotalTestCases)
		confidence := 0.9
		if submissionData.Status != "AC" {
			confidence = 0.6 // Lower confidence for failed submissions
		}

		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "debugging_efficiency",
			Outcome:      debugScore,
			Confidence:   confidence,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"test_cases_passed": submissionData.TestCasesPassed,
				"total_test_cases":  submissionData.TotalTestCases,
				"status":            submissionData.Status,
			},
		})
	}

	// Code complexity score evidence (simplified)
	if submissionData.SourceCodeLength > 0 {
		// Shorter code for solved problems might indicate better algorithm selection
		// This is very simplified - real complexity analysis would require AST parsing
		complexityScore := 1.0
		if submissionData.Status == "AC" && submissionData.SourceCodeLength < 1000 {
			complexityScore = math.Min(1.0, 1000.0/float64(submissionData.SourceCodeLength))
		} else if submissionData.SourceCodeLength > 2000 {
			complexityScore = math.Max(0.2, 2000.0/float64(submissionData.SourceCodeLength))
		}

		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "code_complexity_score",
			Outcome:      complexityScore,
			Confidence:   0.4, // Lower confidence as this is a rough heuristic
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"source_code_length": submissionData.SourceCodeLength,
				"status":             submissionData.Status,
			},
		})
	}

	// Algorithm selection accuracy evidence
	if submissionData.Status == "AC" {
		// Successful submission suggests good algorithm selection
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "algorithm_selection_accuracy",
			Outcome:      1.0,
			Confidence:   0.8,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"status":   submissionData.Status,
				"language": submissionData.Language,
			},
		})
	} else if submissionData.Status == "TLE" || submissionData.Status == "WA" {
		// Wrong algorithm choice
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "algorithm_selection_accuracy",
			Outcome:      0.2,
			Confidence:   0.6,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"status":   submissionData.Status,
				"language": submissionData.Language,
			},
		})
	}

	return evidences, nil
}

// ExtractEvidenceFromContest extracts skill evidence from contest participation
func (bsm *BayesianSkillModel) ExtractEvidenceFromContest(
	ctx context.Context,
	contestData *ContestEventData,
	userID uuid.UUID,
	timestamp time.Time,
) ([]*SkillEvidence, error) {

	var evidences []*SkillEvidence

	// Contest ranking percentile evidence
	if contestData.RankAtTime != nil {
		// Convert rank to percentile (lower rank = higher percentile)
		// This would need actual contest size for accurate calculation
		rankScore := math.Max(0.1, 1.0-float64(*contestData.RankAtTime)/1000.0) // Assuming max 1000 participants
		
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "contest_ranking_percentile",
			Outcome:      rankScore,
			Confidence:   0.9,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"rank":            *contestData.RankAtTime,
				"time_from_start": contestData.TimeFromStart,
				"contest_id":      contestData.ContestID,
			},
		})
	}

	// Time pressure performance evidence
	if contestData.Action == "submit" && contestData.TimeFromStart > 0 {
		// Earlier submissions under time pressure suggest better performance
		timePressureScore := math.Max(0.1, 1.0-float64(contestData.TimeFromStart)/180.0) // Normalize to 3 hours
		
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "time_pressure_performance",
			Outcome:      timePressureScore,
			Confidence:   0.7,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"time_from_start": contestData.TimeFromStart,
				"action":          contestData.Action,
				"contest_id":      contestData.ContestID,
			},
		})
	}

	// Multi-problem efficiency evidence
	if contestData.Action == "submit" {
		// Quick problem switching suggests good multi-problem efficiency
		// This is simplified - would need more context about problem sequence
		evidences = append(evidences, &SkillEvidence{
			UserID:       userID,
			SkillCategory: "multi_problem_efficiency",
			Outcome:      0.7, // Default moderate score for submission
			Confidence:   0.5,
			Timestamp:    timestamp,
			Context: map[string]interface{}{
				"action":     contestData.Action,
				"contest_id": contestData.ContestID,
			},
		})
	}

	return evidences, nil
}

// CalculateOverallSkillRating calculates an overall skill rating from individual skill estimates
func (bsm *BayesianSkillModel) CalculateOverallSkillRating(estimates []*SkillEstimate, weights *MetricWeights) float64 {
	if len(estimates) == 0 {
		return 0.5 // Default neutral rating
	}

	var totalWeightedScore float64
	var totalWeight float64

	for _, estimate := range estimates {
		var weight float64
		
		// Map skill categories to weight categories
		switch estimate.SkillCategory {
		case "problem_solving_speed", "debugging_efficiency", "pattern_recognition_accuracy", "algorithm_selection_accuracy":
			weight = weights.ProblemSolving / 4.0 // Divide among problem-solving skills
		case "contest_ranking_percentile", "time_pressure_performance", "multi_problem_efficiency", "contest_consistency", "penalty_optimization":
			weight = weights.Contest / 5.0 // Divide among contest skills
		case "learning_velocity", "knowledge_retention", "error_pattern_reduction", "adaptive_strategy_usage", "meta_cognitive_awareness":
			weight = weights.Learning / 5.0 // Divide among learning skills
		default:
			weight = 0.1 // Default weight for unknown categories
		}

		// Weight by confidence (inverse of variance)
		confidence := 1.0 / (1.0 + estimate.Variance)
		effectiveWeight := weight * confidence

		totalWeightedScore += estimate.Mean * effectiveWeight
		totalWeight += effectiveWeight
	}

	if totalWeight == 0 {
		return 0.5
	}

	return totalWeightedScore / totalWeight
}

// PredictPerformance predicts future performance based on current skill estimates
func (bsm *BayesianSkillModel) PredictPerformance(
	ctx context.Context,
	estimates []*SkillEstimate,
	scenario string, // "easy_problem", "medium_problem", "hard_problem", "contest"
) (float64, float64, error) {

	if len(estimates) == 0 {
		return 0.5, 0.3, nil // Default prediction with high uncertainty
	}

	var relevantSkills []*SkillEstimate
	
	// Select relevant skills based on scenario
	switch scenario {
	case "easy_problem":
		relevantSkills = filterSkillsByCategory(estimates, []string{
			"algorithm_selection_accuracy", "pattern_recognition_accuracy",
		})
	case "medium_problem":
		relevantSkills = filterSkillsByCategory(estimates, []string{
			"problem_solving_speed", "debugging_efficiency", "algorithm_selection_accuracy",
		})
	case "hard_problem":
		relevantSkills = filterSkillsByCategory(estimates, []string{
			"problem_solving_speed", "debugging_efficiency", "pattern_recognition_accuracy",
			"adaptive_strategy_usage", "meta_cognitive_awareness",
		})
	case "contest":
		relevantSkills = filterSkillsByCategory(estimates, []string{
			"contest_ranking_percentile", "time_pressure_performance", "multi_problem_efficiency",
		})
	default:
		relevantSkills = estimates
	}

	if len(relevantSkills) == 0 {
		return 0.5, 0.3, nil
	}

	// Calculate weighted prediction
	var totalPrediction float64
	var totalWeight float64
	var totalVariance float64

	for _, skill := range relevantSkills {
		weight := 1.0 / (1.0 + skill.Variance) // Higher weight for more certain estimates
		totalPrediction += skill.Mean * weight
		totalWeight += weight
		totalVariance += skill.Variance * weight * weight
	}

	prediction := totalPrediction / totalWeight
	uncertainty := math.Sqrt(totalVariance) / totalWeight

	return prediction, uncertainty, nil
}

// filterSkillsByCategory filters skill estimates by category names
func filterSkillsByCategory(estimates []*SkillEstimate, categories []string) []*SkillEstimate {
	categorySet := make(map[string]bool)
	for _, cat := range categories {
		categorySet[cat] = true
	}

	var filtered []*SkillEstimate
	for _, estimate := range estimates {
		if categorySet[estimate.SkillCategory] {
			filtered = append(filtered, estimate)
		}
	}
	return filtered
}

// ValidateSkillEstimate validates a skill estimate for consistency
func (bsm *BayesianSkillModel) ValidateSkillEstimate(estimate *SkillEstimate) error {
	if estimate.Mean < 0.0 || estimate.Mean > 1.0 {
		return fmt.Errorf("skill mean must be between 0 and 1, got %f", estimate.Mean)
	}
	
	if estimate.Variance < 0.0 {
		return fmt.Errorf("skill variance must be non-negative, got %f", estimate.Variance)
	}
	
	if estimate.Alpha <= 0.0 || estimate.Beta <= 0.0 {
		return fmt.Errorf("Beta distribution parameters must be positive, got alpha=%f, beta=%f", 
			estimate.Alpha, estimate.Beta)
	}
	
	if estimate.ConfidenceIntervalLower > estimate.ConfidenceIntervalUpper {
		return fmt.Errorf("confidence interval lower bound (%f) cannot be greater than upper bound (%f)",
			estimate.ConfidenceIntervalLower, estimate.ConfidenceIntervalUpper)
	}
	
	return nil
}

// GetSkillTrend calculates the trend in skill development over time
func (bsm *BayesianSkillModel) GetSkillTrend(estimates []*SkillEstimate, timeWindow time.Duration) (float64, error) {
	if len(estimates) < 2 {
		return 0.0, nil // No trend with insufficient data
	}

	// Sort by timestamp
	for i := 0; i < len(estimates)-1; i++ {
		for j := i + 1; j < len(estimates); j++ {
			if estimates[i].LastUpdated.After(estimates[j].LastUpdated) {
				estimates[i], estimates[j] = estimates[j], estimates[i]
			}
		}
	}

	// Calculate trend using linear regression on skill means
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(estimates))

	for i, estimate := range estimates {
		x := float64(i)
		y := estimate.Mean
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Linear regression slope
	if n*sumX2-sumX*sumX == 0 {
		return 0.0, nil
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope, nil
}