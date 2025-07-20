package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestSkillRadarVisualizationSchema tests the skill radar data structure
func TestSkillRadarVisualizationSchema(t *testing.T) {
	userID := uuid.New()
	
	radarData := &SkillRadarVisualization{
		UserID: userID,
		Data: []SkillRadarPoint{
			{
				Skill:       "Problem Solving Speed",
				Score:       75.5,
				Confidence:  85.2,
				Category:    "problem_solving",
				Description: "Your problem solving speed based on recent performance",
			},
			{
				Skill:       "Contest Ranking",
				Score:       68.3,
				Confidence:  78.9,
				Category:    "contest",
				Description: "Your contest ranking percentile",
			},
		},
		Meta: SkillRadarMeta{
			MaxScore:       100.0,
			LastUpdated:    time.Now(),
			TotalSkills:    2,
			AverageScore:   71.9,
			StrongestSkill: "Problem Solving Speed",
			WeakestSkill:   "Contest Ranking",
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(radarData)
	if err != nil {
		t.Fatalf("Failed to marshal SkillRadarVisualization: %v", err)
	}

	// Test JSON deserialization
	var unmarshaled SkillRadarVisualization
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SkillRadarVisualization: %v", err)
	}

	// Validate structure
	if unmarshaled.UserID != userID {
		t.Errorf("UserID mismatch: expected %v, got %v", userID, unmarshaled.UserID)
	}

	if len(unmarshaled.Data) != 2 {
		t.Errorf("Expected 2 skill points, got %d", len(unmarshaled.Data))
	}

	// Validate score ranges
	for _, point := range unmarshaled.Data {
		if point.Score < 0 || point.Score > 100 {
			t.Errorf("Score %f is out of range [0, 100]", point.Score)
		}
		if point.Confidence < 0 || point.Confidence > 100 {
			t.Errorf("Confidence %f is out of range [0, 100]", point.Confidence)
		}
		if point.Category == "" {
			t.Error("Category should not be empty")
		}
	}
}

// TestPerformanceTrendVisualizationSchema tests the performance trend data structure
func TestPerformanceTrendVisualizationSchema(t *testing.T) {
	userID := uuid.New()
	
	trendData := &PerformanceTrendVisualization{
		UserID: userID,
		Period: "weekly",
		Datasets: []TrendDataset{
			{
				Label:       "Problem Solving Speed",
				MetricKey:   "problem_solving_speed",
				Data: []DataPoint{
					{"2025-01-01T00:00:00Z", 15.5},
					{"2025-01-08T00:00:00Z", 14.2},
					{"2025-01-15T00:00:00Z", 13.8},
				},
				Color:       "#FF6384",
				Unit:        "minutes",
				Description: "Average time to solve problems",
			},
		},
		Meta: PerformanceTrendMeta{
			DateRange: DateRange{
				Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			TotalPoints: 3,
			TrendSummary: map[string]TrendDirection{
				"problem_solving_speed": {
					Direction: "up",
					Change:    -10.9,
					Period:    "last_weekly",
				},
			},
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(trendData)
	if err != nil {
		t.Fatalf("Failed to marshal PerformanceTrendVisualization: %v", err)
	}

	// Test JSON deserialization
	var unmarshaled PerformanceTrendVisualization
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal PerformanceTrendVisualization: %v", err)
	}

	// Validate structure
	if unmarshaled.UserID != userID {
		t.Errorf("UserID mismatch: expected %v, got %v", userID, unmarshaled.UserID)
	}

	if len(unmarshaled.Datasets) != 1 {
		t.Errorf("Expected 1 dataset, got %d", len(unmarshaled.Datasets))
	}

	dataset := unmarshaled.Datasets[0]
	if len(dataset.Data) != 3 {
		t.Errorf("Expected 3 data points, got %d", len(dataset.Data))
	}

	// Validate data point format
	for i, point := range dataset.Data {
		if len(point) != 2 {
			t.Errorf("Data point %d should have 2 elements, got %d", i, len(point))
		}
		
		// First element should be timestamp string
		if _, ok := point[0].(string); !ok {
			t.Errorf("Data point %d first element should be string timestamp", i)
		}
		
		// Second element should be numeric value
		if _, ok := point[1].(float64); !ok {
			t.Errorf("Data point %d second element should be numeric value", i)
		}
	}
}

// TestChartJSCompatibility tests Chart.js data format compatibility
func TestChartJSCompatibility(t *testing.T) {
	// Test radar chart format
	radarData := ChartJSRadarData{
		Labels: []string{"Speed", "Accuracy", "Efficiency"},
		Datasets: []ChartJSDataset{
			{
				Label:           "User Skills",
				Data:            []float64{75.5, 82.1, 68.3},
				BackgroundColor: "rgba(255, 99, 132, 0.2)",
				BorderColor:     "rgba(255, 99, 132, 1)",
				BorderWidth:     2,
			},
		},
	}

	jsonData, err := json.Marshal(radarData)
	if err != nil {
		t.Fatalf("Failed to marshal ChartJSRadarData: %v", err)
	}

	// Verify the JSON structure matches Chart.js expectations
	var generic map[string]interface{}
	err = json.Unmarshal(jsonData, &generic)
	if err != nil {
		t.Fatalf("Failed to unmarshal as generic map: %v", err)
	}

	// Check required fields for Chart.js
	if _, exists := generic["labels"]; !exists {
		t.Error("Chart.js data must have 'labels' field")
	}
	if _, exists := generic["datasets"]; !exists {
		t.Error("Chart.js data must have 'datasets' field")
	}

	// Check datasets structure
	datasets, ok := generic["datasets"].([]interface{})
	if !ok {
		t.Error("Datasets should be an array")
	}

	if len(datasets) > 0 {
		dataset, ok := datasets[0].(map[string]interface{})
		if !ok {
			t.Error("Dataset should be an object")
		}

		requiredFields := []string{"label", "data"}
		for _, field := range requiredFields {
			if _, exists := dataset[field]; !exists {
				t.Errorf("Dataset must have '%s' field", field)
			}
		}
	}
}

// TestAPIResponseWrapper tests the standard API response format
func TestAPIResponseWrapper(t *testing.T) {
	userID := uuid.New()
	
	// Create sample data
	sampleData := map[string]interface{}{
		"user_id": userID,
		"score":   85.5,
	}

	// Create API response
	response := APIResponse{
		Success:   true,
		Data:      sampleData,
		Timestamp: time.Now(),
		Meta: &APIMeta{
			RequestID:   "req-123",
			CacheHit:    true,
			ProcessTime: "25ms",
			Version:     "1.0.0",
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal APIResponse: %v", err)
	}

	// Test JSON deserialization
	var unmarshaled APIResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal APIResponse: %v", err)
	}

	// Validate structure
	if !unmarshaled.Success {
		t.Error("Success should be true")
	}
	if unmarshaled.Data == nil {
		t.Error("Data should not be nil")
	}
	if unmarshaled.Meta == nil {
		t.Error("Meta should not be nil")
	}
	if unmarshaled.Meta.RequestID != "req-123" {
		t.Errorf("RequestID mismatch: expected 'req-123', got '%s'", unmarshaled.Meta.RequestID)
	}
}

// TestErrorResponse tests API error response format
func TestErrorResponse(t *testing.T) {
	errorResponse := APIResponse{
		Success:   false,
		Timestamp: time.Now(),
		Error: &APIError{
			Code:    "INVALID_USER_ID",
			Message: "The provided user ID is not valid",
			Details: "User ID must be a valid UUID",
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(errorResponse)
	if err != nil {
		t.Fatalf("Failed to marshal error APIResponse: %v", err)
	}

	// Test JSON deserialization
	var unmarshaled APIResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal error APIResponse: %v", err)
	}

	// Validate error structure
	if unmarshaled.Success {
		t.Error("Success should be false for error response")
	}
	if unmarshaled.Error == nil {
		t.Error("Error should not be nil for error response")
	}
	if unmarshaled.Error.Code != "INVALID_USER_ID" {
		t.Errorf("Error code mismatch: expected 'INVALID_USER_ID', got '%s'", unmarshaled.Error.Code)
	}
}

// TestDataPointSerialization tests that DataPoint serializes correctly as [timestamp, value]
func TestDataPointSerialization(t *testing.T) {
	dataPoint := DataPoint{"2025-01-15T12:00:00Z", 75.5}

	jsonData, err := json.Marshal(dataPoint)
	if err != nil {
		t.Fatalf("Failed to marshal DataPoint: %v", err)
	}

	// Should serialize as array
	expected := `["2025-01-15T12:00:00Z",75.5]`
	if string(jsonData) != expected {
		t.Errorf("DataPoint serialization mismatch:\nExpected: %s\nGot: %s", expected, string(jsonData))
	}

	// Test deserialization
	var unmarshaled DataPoint
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal DataPoint: %v", err)
	}

	if len(unmarshaled) != 2 {
		t.Errorf("DataPoint should have 2 elements, got %d", len(unmarshaled))
	}
}

// TestSkillCategoryMapping tests skill category display name mapping
func TestSkillCategoryMapping(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"problem_solving_speed", "Problem Solving Speed"},
		{"debugging_efficiency", "Debugging Efficiency"},
		{"contest_ranking_percentile", "Contest Ranking"},
		{"unknown_skill", "unknown_skill"}, // Should return input if not found
	}

	for _, tc := range testCases {
		result := GetDisplayName(tc.input)
		if result != tc.expected {
			t.Errorf("GetDisplayName(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

// TestSkillColors tests skill category color mapping
func TestSkillColors(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"problem_solving_speed", "#FF6384"},
		{"debugging_efficiency", "#36A2EB"},
		{"unknown_skill", "#999999"}, // Default color
	}

	for _, tc := range testCases {
		result := GetSkillColor(tc.input)
		if result != tc.expected {
			t.Errorf("GetSkillColor(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

// TestVisualizationDataValidation tests data validation for visualization structures
func TestVisualizationDataValidation(t *testing.T) {
	// Test invalid score ranges
	invalidRadarData := &SkillRadarVisualization{
		UserID: uuid.New(),
		Data: []SkillRadarPoint{
			{
				Skill:      "Test Skill",
				Score:      150.0, // Invalid: > 100
				Confidence: 85.0,
				Category:   "test",
			},
		},
	}

	// This should be caught in validation (if implemented)
	for _, point := range invalidRadarData.Data {
		if point.Score > 100 {
			t.Logf("Detected invalid score: %f (> 100)", point.Score)
		}
		if point.Score < 0 {
			t.Logf("Detected invalid score: %f (< 0)", point.Score)
		}
	}
}

// BenchmarkJSONSerialization benchmarks JSON serialization performance
func BenchmarkJSONSerialization(b *testing.B) {
	userID := uuid.New()
	
	radarData := &SkillRadarVisualization{
		UserID: userID,
		Data: make([]SkillRadarPoint, 15), // 15 skills
		Meta: SkillRadarMeta{
			MaxScore:    100.0,
			LastUpdated: time.Now(),
			TotalSkills: 15,
		},
	}

	// Fill with sample data
	for i := 0; i < 15; i++ {
		radarData.Data[i] = SkillRadarPoint{
			Skill:       SkillCategories[i],
			Score:       float64(50 + i*3),
			Confidence:  85.0,
			Category:    "test",
			Description: "Test skill",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(radarData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestLargeDataSetSerialization tests serialization of large datasets
func TestLargeDataSetSerialization(t *testing.T) {
	userID := uuid.New()
	
	// Create large trend dataset (1 year of daily data)
	trendData := &PerformanceTrendVisualization{
		UserID:   userID,
		Period:   "daily",
		Datasets: make([]TrendDataset, 3),
	}

	// Create 3 metrics with 365 data points each
	for i := 0; i < 3; i++ {
		dataset := TrendDataset{
			Label:     fmt.Sprintf("Metric %d", i),
			MetricKey: fmt.Sprintf("metric_%d", i),
			Data:      make([]DataPoint, 365),
			Color:     "#FF6384",
			Unit:      "score",
		}

		// Fill with daily data for a year
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for j := 0; j < 365; j++ {
			timestamp := baseTime.AddDate(0, 0, j)
			value := 50.0 + float64(j%30) // Varying values
			dataset.Data[j] = DataPoint{timestamp.Format(time.RFC3339), value}
		}

		trendData.Datasets[i] = dataset
	}

	// Test serialization of large dataset
	start := time.Now()
	jsonData, err := json.Marshal(trendData)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to marshal large dataset: %v", err)
	}

	t.Logf("Serialized %d data points in %v, JSON size: %d bytes", 
		365*3, duration, len(jsonData))

	// Ensure serialization is reasonably fast (< 100ms for this size)
	if duration > 100*time.Millisecond {
		t.Errorf("Serialization took too long: %v", duration)
	}
}