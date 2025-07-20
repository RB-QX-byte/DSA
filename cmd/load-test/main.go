package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"competitive-programming-platform/internal/realtime"
)

func main() {
	var (
		baseURL       = flag.String("url", "http://localhost:8080", "Base URL of the server")
		connections   = flag.Int("connections", 1000, "Total number of connections to create")
		connPerSec    = flag.Int("rate", 100, "Connections per second")
		duration      = flag.Duration("duration", 60*time.Second, "Test duration")
		contestID     = flag.String("contest", "", "Contest ID for contest-specific testing")
		authToken     = flag.String("token", "", "Authentication token")
		testType      = flag.String("type", "sse", "Test type: sse or websocket")
		outputFile    = flag.String("output", "", "Output file for results (JSON)")
		verbose       = flag.Bool("verbose", false, "Verbose logging")
		scenarios     = flag.Bool("scenarios", false, "Run predefined test scenarios")
	)
	flag.Parse()

	if *scenarios {
		runTestScenarios(*baseURL, *authToken, *contestID, *outputFile)
		return
	}

	// Single load test
	config := realtime.LoadTestConfig{
		BaseURL:           *baseURL,
		TotalConnections:  *connections,
		ConnectionsPerSec: *connPerSec,
		TestDuration:      *duration,
		AuthToken:         *authToken,
		ContestID:         *contestID,
		TestType:          *testType,
	}

	result, err := runSingleTest(config, *verbose)
	if err != nil {
		log.Fatalf("Load test failed: %v", err)
	}

	// Output results
	if *outputFile != "" {
		saveResults(*outputFile, result)
	}

	printResults(result)
}

func runSingleTest(config realtime.LoadTestConfig, verbose bool) (*realtime.LoadTestResult, error) {
	if verbose {
		log.Printf("Starting load test with config: %+v", config)
	}

	ctx := context.Background()
	
	// Capture initial system metrics
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)
	
	result, err := realtime.RunLoadTest(ctx, config)
	if err != nil {
		return nil, err
	}

	// Capture final system metrics
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	
	// Add memory usage to results
	result.MemoryUsage = int64(finalMemStats.Alloc-initialMemStats.Alloc) / (1024 * 1024) // MB

	return result, nil
}

func runTestScenarios(baseURL, authToken, contestID, outputFile string) {
	scenarios := []struct {
		name   string
		config realtime.LoadTestConfig
	}{
		{
			name: "Light Load",
			config: realtime.LoadTestConfig{
				BaseURL:           baseURL,
				TotalConnections:  100,
				ConnectionsPerSec: 10,
				TestDuration:      30 * time.Second,
				AuthToken:         authToken,
				ContestID:         contestID,
				TestType:          "sse",
			},
		},
		{
			name: "Medium Load",
			config: realtime.LoadTestConfig{
				BaseURL:           baseURL,
				TotalConnections:  1000,
				ConnectionsPerSec: 50,
				TestDuration:      60 * time.Second,
				AuthToken:         authToken,
				ContestID:         contestID,
				TestType:          "sse",
			},
		},
		{
			name: "Heavy Load",
			config: realtime.LoadTestConfig{
				BaseURL:           baseURL,
				TotalConnections:  5000,
				ConnectionsPerSec: 100,
				TestDuration:      90 * time.Second,
				AuthToken:         authToken,
				ContestID:         contestID,
				TestType:          "sse",
			},
		},
		{
			name: "Extreme Load",
			config: realtime.LoadTestConfig{
				BaseURL:           baseURL,
				TotalConnections:  20000,
				ConnectionsPerSec: 200,
				TestDuration:      120 * time.Second,
				AuthToken:         authToken,
				ContestID:         contestID,
				TestType:          "sse",
			},
		},
	}

	results := make(map[string]*realtime.LoadTestResult)
	
	for _, scenario := range scenarios {
		fmt.Printf("\n=== Running %s Scenario ===\n", scenario.name)
		
		result, err := runSingleTest(scenario.config, true)
		if err != nil {
			log.Printf("Scenario %s failed: %v", scenario.name, err)
			continue
		}
		
		results[scenario.name] = result
		printResults(result)
		
		// Wait between scenarios
		fmt.Println("Waiting 30 seconds before next scenario...")
		time.Sleep(30 * time.Second)
	}

	// Save all results
	if outputFile != "" {
		saveScenarioResults(outputFile, results)
	}

	// Print summary
	printScenarioSummary(results)
}

func printResults(result *realtime.LoadTestResult) {
	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("Test Duration: %v\n", result.TestDuration)
	fmt.Printf("Total Connections: %d\n", result.TotalConnections)
	fmt.Printf("Successful Connections: %d (%.1f%%)\n", 
		result.SuccessfulConnections, 
		float64(result.SuccessfulConnections)/float64(result.TotalConnections)*100)
	fmt.Printf("Failed Connections: %d (%.1f%%)\n", 
		result.FailedConnections,
		float64(result.FailedConnections)/float64(result.TotalConnections)*100)
	fmt.Printf("Messages Received: %d\n", result.MessagesReceived)
	fmt.Printf("Avg Messages/Second: %.2f\n", result.AvgMessagesPerSecond)
	fmt.Printf("Latency - Avg: %v, Min: %v, Max: %v\n", 
		result.AvgLatency, result.MinLatency, result.MaxLatency)
	fmt.Printf("Memory Usage: %d MB\n", result.MemoryUsage)
	fmt.Printf("Connections/Second: %d\n", result.ConnectionsPerSecond)
	
	// Performance assessment
	assessPerformance(result)
}

func assessPerformance(result *realtime.LoadTestResult) {
	fmt.Printf("\n=== Performance Assessment ===\n")
	
	successRate := float64(result.SuccessfulConnections) / float64(result.TotalConnections) * 100
	
	if successRate >= 99 {
		fmt.Printf("✅ Connection Success Rate: EXCELLENT (%.1f%%)\n", successRate)
	} else if successRate >= 95 {
		fmt.Printf("✅ Connection Success Rate: GOOD (%.1f%%)\n", successRate)
	} else if successRate >= 90 {
		fmt.Printf("⚠️  Connection Success Rate: FAIR (%.1f%%)\n", successRate)
	} else {
		fmt.Printf("❌ Connection Success Rate: POOR (%.1f%%)\n", successRate)
	}
	
	if result.AvgLatency < 100*time.Millisecond {
		fmt.Printf("✅ Average Latency: EXCELLENT (%v)\n", result.AvgLatency)
	} else if result.AvgLatency < 500*time.Millisecond {
		fmt.Printf("✅ Average Latency: GOOD (%v)\n", result.AvgLatency)
	} else if result.AvgLatency < 1*time.Second {
		fmt.Printf("⚠️  Average Latency: FAIR (%v)\n", result.AvgLatency)
	} else {
		fmt.Printf("❌ Average Latency: POOR (%v)\n", result.AvgLatency)
	}
	
	if result.AvgMessagesPerSecond > 1000 {
		fmt.Printf("✅ Message Throughput: EXCELLENT (%.2f msg/s)\n", result.AvgMessagesPerSecond)
	} else if result.AvgMessagesPerSecond > 500 {
		fmt.Printf("✅ Message Throughput: GOOD (%.2f msg/s)\n", result.AvgMessagesPerSecond)
	} else if result.AvgMessagesPerSecond > 100 {
		fmt.Printf("⚠️  Message Throughput: FAIR (%.2f msg/s)\n", result.AvgMessagesPerSecond)
	} else {
		fmt.Printf("❌ Message Throughput: POOR (%.2f msg/s)\n", result.AvgMessagesPerSecond)
	}
	
	// Recommendations
	fmt.Printf("\n=== Recommendations ===\n")
	if successRate < 95 {
		fmt.Println("• Consider increasing server resources or connection limits")
		fmt.Println("• Check for network bottlenecks")
	}
	if result.AvgLatency > 500*time.Millisecond {
		fmt.Println("• Optimize database queries and caching")
		fmt.Println("• Consider using a message queue for high-throughput scenarios")
	}
	if result.MemoryUsage > 1000 {
		fmt.Println("• Monitor memory usage for potential leaks")
		fmt.Println("• Consider implementing connection pooling")
	}
}

func printScenarioSummary(results map[string]*realtime.LoadTestResult) {
	fmt.Printf("\n=== Scenario Summary ===\n")
	fmt.Printf("%-15s %-10s %-15s %-12s %-15s\n", 
		"Scenario", "Connections", "Success Rate", "Avg Latency", "Messages/Sec")
	fmt.Println(strings.Repeat("-", 80))
	
	for name, result := range results {
		successRate := float64(result.SuccessfulConnections) / float64(result.TotalConnections) * 100
		fmt.Printf("%-15s %-10d %-15.1f%% %-12v %-15.2f\n",
			name, result.TotalConnections, successRate, 
			result.AvgLatency, result.AvgMessagesPerSecond)
	}
}

func saveResults(filename string, result *realtime.LoadTestResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal results: %v", err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("Failed to save results to %s: %v", filename, err)
		return
	}

	fmt.Printf("Results saved to %s\n", filename)
}

func saveScenarioResults(filename string, results map[string]*realtime.LoadTestResult) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal scenario results: %v", err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("Failed to save scenario results to %s: %v", filename, err)
		return
	}

	fmt.Printf("Scenario results saved to %s\n", filename)
}

