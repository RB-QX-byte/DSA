package analytics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBayesianSkillModel_UpdateSkillEstimate(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	ctx := context.Background()
	
	userID := uuid.New()
	timestamp := time.Now()

	// Test initial skill estimate with no prior
	evidence := &SkillEvidence{
		UserID:       userID,
		SkillCategory: "problem_solving_speed",
		Outcome:      0.8,
		Confidence:   0.9,
		Timestamp:    timestamp,
	}

	estimate, err := model.UpdateSkillEstimate(ctx, nil, evidence)
	if err != nil {
		t.Fatalf("Failed to update skill estimate: %v", err)
	}

	// Validate the estimate
	if err := model.ValidateSkillEstimate(estimate); err != nil {
		t.Fatalf("Invalid skill estimate: %v", err)
	}

	// Check that mean is reasonable
	if estimate.Mean <= 0 || estimate.Mean >= 1 {
		t.Errorf("Expected mean between 0 and 1, got %f", estimate.Mean)
	}

	// Test updating existing estimate
	newEvidence := &SkillEvidence{
		UserID:       userID,
		SkillCategory: "problem_solving_speed",
		Outcome:      0.6,
		Confidence:   0.8,
		Timestamp:    timestamp.Add(time.Hour),
	}

	updatedEstimate, err := model.UpdateSkillEstimate(ctx, estimate, newEvidence)
	if err != nil {
		t.Fatalf("Failed to update existing skill estimate: %v", err)
	}

	// Evidence count should increase
	if updatedEstimate.EvidenceCount != estimate.EvidenceCount+1 {
		t.Errorf("Expected evidence count %d, got %d", 
			estimate.EvidenceCount+1, updatedEstimate.EvidenceCount)
	}

	// Mean should be between the two outcomes
	if updatedEstimate.Mean < 0.6 || updatedEstimate.Mean > 0.8 {
		t.Errorf("Updated mean %f should be influenced by both evidences", updatedEstimate.Mean)
	}
}

func TestBayesianSkillModel_ExtractEvidenceFromSubmission(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	ctx := context.Background()
	
	userID := uuid.New()
	timestamp := time.Now()

	// Test successful submission
	submissionData := &SubmissionEventData{
		SubmissionID:     uuid.New(),
		ProblemID:        uuid.New(),
		Status:           "AC",
		ExecutionTime:    intPtr(500),
		MemoryUsage:      intPtr(1024),
		Language:         "go",
		TestCasesPassed:  10,
		TotalTestCases:   10,
		SourceCodeLength: 150,
	}

	evidences, err := model.ExtractEvidenceFromSubmission(ctx, submissionData, userID, timestamp)
	if err != nil {
		t.Fatalf("Failed to extract evidence from submission: %v", err)
	}

	if len(evidences) == 0 {
		t.Fatal("Expected at least one evidence from successful submission")
	}

	// Check that we have evidence for multiple skills
	skillCategories := make(map[string]bool)
	for _, evidence := range evidences {
		skillCategories[evidence.SkillCategory] = true
		
		// Validate evidence
		if evidence.UserID != userID {
			t.Errorf("Evidence user ID mismatch")
		}
		if evidence.Outcome < 0 || evidence.Outcome > 1 {
			t.Errorf("Evidence outcome %f should be between 0 and 1", evidence.Outcome)
		}
		if evidence.Confidence < 0 || evidence.Confidence > 1 {
			t.Errorf("Evidence confidence %f should be between 0 and 1", evidence.Confidence)
		}
	}

	// Should have evidence for problem solving skills
	expectedSkills := []string{"problem_solving_speed", "debugging_efficiency", "algorithm_selection_accuracy"}
	for _, skill := range expectedSkills {
		if !skillCategories[skill] {
			t.Errorf("Expected evidence for skill %s", skill)
		}
	}
}

func TestBayesianSkillModel_ExtractEvidenceFromContest(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	ctx := context.Background()
	
	userID := uuid.New()
	timestamp := time.Now()

	// Test contest submission
	contestData := &ContestEventData{
		ContestID:     uuid.New(),
		Action:        "submit",
		ProblemID:     &uuid.UUID{},
		SubmissionID:  &uuid.UUID{},
		TimeFromStart: 30, // 30 minutes from start
		RankAtTime:    intPtr(25),
	}
	*contestData.ProblemID = uuid.New()
	*contestData.SubmissionID = uuid.New()

	evidences, err := model.ExtractEvidenceFromContest(ctx, contestData, userID, timestamp)
	if err != nil {
		t.Fatalf("Failed to extract evidence from contest: %v", err)
	}

	if len(evidences) == 0 {
		t.Fatal("Expected at least one evidence from contest submission")
	}

	// Check contest-specific skills
	skillCategories := make(map[string]bool)
	for _, evidence := range evidences {
		skillCategories[evidence.SkillCategory] = true
	}

	expectedContestSkills := []string{"contest_ranking_percentile", "time_pressure_performance"}
	for _, skill := range expectedContestSkills {
		if !skillCategories[skill] {
			t.Errorf("Expected evidence for contest skill %s", skill)
		}
	}
}

func TestBayesianSkillModel_CalculateOverallSkillRating(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	weights := DefaultMetricWeights()

	// Create sample skill estimates
	estimates := []*SkillEstimate{
		{
			SkillCategory: "problem_solving_speed",
			Mean:          0.8,
			Variance:      0.1,
		},
		{
			SkillCategory: "debugging_efficiency",
			Mean:          0.7,
			Variance:      0.05,
		},
		{
			SkillCategory: "contest_ranking_percentile",
			Mean:          0.6,
			Variance:      0.2,
		},
	}

	rating := model.CalculateOverallSkillRating(estimates, weights)

	// Rating should be between 0 and 1
	if rating < 0 || rating > 1 {
		t.Errorf("Overall rating %f should be between 0 and 1", rating)
	}

	// Rating should be influenced by the estimates
	if rating < 0.6 || rating > 0.8 {
		t.Errorf("Overall rating %f should be influenced by skill estimates", rating)
	}
}

func TestBayesianSkillModel_PredictPerformance(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	ctx := context.Background()

	estimates := []*SkillEstimate{
		{
			SkillCategory: "problem_solving_speed",
			Mean:          0.8,
			Variance:      0.1,
		},
		{
			SkillCategory: "algorithm_selection_accuracy",
			Mean:          0.7,
			Variance:      0.05,
		},
		{
			SkillCategory: "contest_ranking_percentile",
			Mean:          0.6,
			Variance:      0.2,
		},
	}

	// Test prediction for different scenarios
	scenarios := []string{"easy_problem", "medium_problem", "hard_problem", "contest"}

	for _, scenario := range scenarios {
		prediction, uncertainty, err := model.PredictPerformance(ctx, estimates, scenario)
		if err != nil {
			t.Fatalf("Failed to predict performance for scenario %s: %v", scenario, err)
		}

		// Validate prediction and uncertainty
		if prediction < 0 || prediction > 1 {
			t.Errorf("Prediction %f for scenario %s should be between 0 and 1", prediction, scenario)
		}
		if uncertainty < 0 {
			t.Errorf("Uncertainty %f for scenario %s should be non-negative", uncertainty, scenario)
		}
	}
}

func TestBayesianSkillModel_ValidateSkillEstimate(t *testing.T) {
	model := NewBayesianSkillModel(nil)

	testCases := []struct {
		name      string
		estimate  *SkillEstimate
		expectErr bool
	}{
		{
			name: "valid estimate",
			estimate: &SkillEstimate{
				Mean:                    0.7,
				Variance:                0.1,
				ConfidenceIntervalLower: 0.6,
				ConfidenceIntervalUpper: 0.8,
				Alpha:                   2.0,
				Beta:                    1.0,
			},
			expectErr: false,
		},
		{
			name: "invalid mean too high",
			estimate: &SkillEstimate{
				Mean:     1.5,
				Variance: 0.1,
				Alpha:    2.0,
				Beta:     1.0,
			},
			expectErr: true,
		},
		{
			name: "invalid mean too low",
			estimate: &SkillEstimate{
				Mean:     -0.1,
				Variance: 0.1,
				Alpha:    2.0,
				Beta:     1.0,
			},
			expectErr: true,
		},
		{
			name: "invalid negative variance",
			estimate: &SkillEstimate{
				Mean:     0.7,
				Variance: -0.1,
				Alpha:    2.0,
				Beta:     1.0,
			},
			expectErr: true,
		},
		{
			name: "invalid alpha",
			estimate: &SkillEstimate{
				Mean:     0.7,
				Variance: 0.1,
				Alpha:    -1.0,
				Beta:     1.0,
			},
			expectErr: true,
		},
		{
			name: "invalid confidence interval",
			estimate: &SkillEstimate{
				Mean:                    0.7,
				Variance:                0.1,
				ConfidenceIntervalLower: 0.8,
				ConfidenceIntervalUpper: 0.6,
				Alpha:                   2.0,
				Beta:                    1.0,
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := model.ValidateSkillEstimate(tc.estimate)
			if tc.expectErr && err == nil {
				t.Errorf("Expected validation error but got none")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestBayesianSkillModel_GetSkillTrend(t *testing.T) {
	model := NewBayesianSkillModel(nil)
	
	baseTime := time.Now()
	
	// Create estimates with increasing skill level
	estimates := []*SkillEstimate{
		{Mean: 0.5, LastUpdated: baseTime},
		{Mean: 0.6, LastUpdated: baseTime.Add(time.Hour)},
		{Mean: 0.7, LastUpdated: baseTime.Add(2 * time.Hour)},
		{Mean: 0.8, LastUpdated: baseTime.Add(3 * time.Hour)},
	}

	trend, err := model.GetSkillTrend(estimates, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to calculate skill trend: %v", err)
	}

	// Should show positive trend
	if trend <= 0 {
		t.Errorf("Expected positive trend for improving skill, got %f", trend)
	}

	// Test with insufficient data
	singleEstimate := []*SkillEstimate{{Mean: 0.5, LastUpdated: baseTime}}
	trend, err = model.GetSkillTrend(singleEstimate, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to calculate trend with single estimate: %v", err)
	}
	if trend != 0 {
		t.Errorf("Expected zero trend with single estimate, got %f", trend)
	}
}

func TestBayesianSkillModel_TimeDecay(t *testing.T) {
	config := &BayesianParameters{
		PriorAlpha:   1.0,
		PriorBeta:    1.0,
		LearningRate: 0.1,
		DecayFactor:  0.9, // Strong decay for testing
		MinEvidence:  1,
	}
	model := NewBayesianSkillModel(config)
	ctx := context.Background()

	userID := uuid.New()
	baseTime := time.Now()

	// Create initial estimate
	initialEvidence := &SkillEvidence{
		UserID:       userID,
		SkillCategory: "test_skill",
		Outcome:      0.9,
		Confidence:   0.9,
		Timestamp:    baseTime,
	}

	estimate, err := model.UpdateSkillEstimate(ctx, nil, initialEvidence)
	if err != nil {
		t.Fatalf("Failed to create initial estimate: %v", err)
	}

	initialMean := estimate.Mean

	// Add evidence much later (should cause decay)
	laterEvidence := &SkillEvidence{
		UserID:       userID,
		SkillCategory: "test_skill",
		Outcome:      0.1, // Low outcome
		Confidence:   0.9,
		Timestamp:    baseTime.Add(30 * 24 * time.Hour), // 30 days later
	}

	decayedEstimate, err := model.UpdateSkillEstimate(ctx, estimate, laterEvidence)
	if err != nil {
		t.Fatalf("Failed to update with decay: %v", err)
	}

	// The mean should have moved towards the new evidence due to decay
	if math.Abs(decayedEstimate.Mean-initialMean) < 0.1 {
		t.Errorf("Expected significant change due to time decay, initial: %f, final: %f", 
			initialMean, decayedEstimate.Mean)
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}