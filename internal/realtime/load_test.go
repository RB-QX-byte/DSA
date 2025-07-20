package realtime

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// LoadTester conducts load testing on the real-time infrastructure
type LoadTester struct {
	baseURL           string
	totalConnections  int
	connectionsPerSec int
	testDuration      time.Duration
	authToken         string
	contestID         string
	
	// Metrics
	successfulConnections int64
	failedConnections     int64
	messagesReceived      int64
	messagesPerSecond     int64
	avgLatency            int64
	maxLatency            int64
	minLatency            int64
	
	// Control
	stopCh   chan struct{}
	resultCh chan LoadTestResult
	
	mu sync.RWMutex
}

// LoadTestResult contains the results of a load test
type LoadTestResult struct {
	TotalConnections      int           `json:"total_connections"`
	SuccessfulConnections int64         `json:"successful_connections"`
	FailedConnections     int64         `json:"failed_connections"`
	MessagesReceived      int64         `json:"messages_received"`
	AvgMessagesPerSecond  float64       `json:"avg_messages_per_second"`
	AvgLatency            time.Duration `json:"avg_latency"`
	MaxLatency            time.Duration `json:"max_latency"`
	MinLatency            time.Duration `json:"min_latency"`
	TestDuration          time.Duration `json:"test_duration"`
	ConnectionsPerSecond  int           `json:"connections_per_second"`
	MemoryUsage           int64         `json:"memory_usage_mb"`
	CPUUsage              float64       `json:"cpu_usage_percent"`
}

// SSEConnection represents a Server-Sent Events connection for load testing
type SSEConnection struct {
	id            int
	resp          *http.Response
	connected     bool
	messagesRecv  int64
	lastMessage   time.Time
	connectedAt   time.Time
	disconnectedAt time.Time
	errors        []string
}

// NewLoadTester creates a new load tester
func NewLoadTester(baseURL string, totalConnections int, connectionsPerSec int, testDuration time.Duration) *LoadTester {
	return &LoadTester{
		baseURL:           baseURL,
		totalConnections:  totalConnections,
		connectionsPerSec: connectionsPerSec,
		testDuration:      testDuration,
		stopCh:           make(chan struct{}),
		resultCh:         make(chan LoadTestResult, 1),
		minLatency:       time.Hour, // Start with high value
	}
}

// SetAuthToken sets the authentication token for connections
func (lt *LoadTester) SetAuthToken(token string) {
	lt.authToken = token
}

// SetContestID sets the contest ID for contest-specific testing
func (lt *LoadTester) SetContestID(contestID string) {
	lt.contestID = contestID
}

// RunSSELoadTest performs load testing on SSE connections
func (lt *LoadTester) RunSSELoadTest(ctx context.Context) (*LoadTestResult, error) {
	log.Printf("Starting SSE load test: %d connections, %d conn/sec, duration: %v", 
		lt.totalConnections, lt.connectionsPerSec, lt.testDuration)

	startTime := time.Now()
	var wg sync.WaitGroup
	connections := make(chan *SSEConnection, lt.totalConnections)
	
	// Start connection monitor
	go lt.monitorConnections(ctx, connections)
	
	// Start metrics collector
	go lt.collectMetrics(ctx)
	
	// Create connections at specified rate
	ticker := time.NewTicker(time.Second / time.Duration(lt.connectionsPerSec))
	defer ticker.Stop()
	
	connCount := 0
	for connCount < lt.totalConnections {
		select {
		case <-ticker.C:
			if connCount < lt.totalConnections {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					conn := lt.createSSEConnection(ctx, id)
					connections <- conn
				}(connCount)
				connCount++
			}
		case <-ctx.Done():
			break
		case <-time.After(lt.testDuration):
			break
		}
	}
	
	// Wait for test duration
	testTimer := time.NewTimer(lt.testDuration)
	defer testTimer.Stop()
	
	select {
	case <-testTimer.C:
		log.Println("Load test duration completed")
	case <-ctx.Done():
		log.Println("Load test cancelled")
	}
	
	// Signal stop and wait for cleanup
	close(lt.stopCh)
	wg.Wait()
	close(connections)
	
	// Calculate final results
	result := lt.calculateResults(time.Since(startTime))
	return &result, nil
}

// createSSEConnection creates a single SSE connection
func (lt *LoadTester) createSSEConnection(ctx context.Context, id int) *SSEConnection {
	conn := &SSEConnection{
		id:          id,
		connectedAt: time.Now(),
	}
	
	// Build SSE URL
	sseURL := fmt.Sprintf("%s/api/v1/realtime/sse", lt.baseURL)
	if lt.contestID != "" {
		sseURL = fmt.Sprintf("%s/api/v1/realtime/contests/%s/sse", lt.baseURL, lt.contestID)
	}
	
	// Add auth token if available
	if lt.authToken != "" {
		sseURL += "?token=" + url.QueryEscape(lt.authToken)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		conn.errors = append(conn.errors, fmt.Sprintf("Request creation failed: %v", err))
		atomic.AddInt64(&lt.failedConnections, 1)
		return conn
	}
	
	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	
	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		conn.errors = append(conn.errors, fmt.Sprintf("Connection failed: %v", err))
		atomic.AddInt64(&lt.failedConnections, 1)
		return conn
	}
	
	if resp.StatusCode != http.StatusOK {
		conn.errors = append(conn.errors, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
		atomic.AddInt64(&lt.failedConnections, 1)
		resp.Body.Close()
		return conn
	}
	
	conn.resp = resp
	conn.connected = true
	atomic.AddInt64(&lt.successfulConnections, 1)
	
	// Start reading messages
	go lt.readSSEMessages(ctx, conn)
	
	return conn
}

// readSSEMessages reads messages from an SSE connection
func (lt *LoadTester) readSSEMessages(ctx context.Context, conn *SSEConnection) {
	defer func() {
		if conn.resp != nil {
			conn.resp.Body.Close()
		}
		conn.disconnectedAt = time.Now()
		conn.connected = false
	}()
	
	buffer := make([]byte, 8192)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-lt.stopCh:
			return
		default:
			// Set read timeout
			if netConn, ok := conn.resp.Body.(interface{ SetReadDeadline(time.Time) error }); ok {
				netConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			}
			
			n, err := conn.resp.Body.Read(buffer)
			if err != nil {
				if !isTimeoutError(err) {
					conn.errors = append(conn.errors, fmt.Sprintf("Read error: %v", err))
				}
				return
			}
			
			if n > 0 {
				// Process SSE data
				data := string(buffer[:n])
				if len(data) > 0 {
					conn.messagesRecv++
					conn.lastMessage = time.Now()
					atomic.AddInt64(&lt.messagesReceived, 1)
					
					// Calculate latency (simplified - would need timestamp in message)
					latency := time.Since(conn.lastMessage)
					lt.updateLatencyMetrics(latency)
				}
			}
		}
	}
}

// monitorConnections monitors the health of connections
func (lt *LoadTester) monitorConnections(ctx context.Context, connections <-chan *SSEConnection) {
	activeConnections := make(map[int]*SSEConnection)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case conn, ok := <-connections:
			if !ok {
				return
			}
			activeConnections[conn.id] = conn
			
		case <-ticker.C:
			// Log connection statistics
			connected := 0
			failed := 0
			totalMessages := int64(0)
			
			for _, conn := range activeConnections {
				if conn.connected {
					connected++
				} else {
					failed++
				}
				totalMessages += conn.messagesRecv
			}
			
			log.Printf("Connections: %d active, %d failed, %d total messages", 
				connected, failed, totalMessages)
			
		case <-ctx.Done():
			return
		case <-lt.stopCh:
			return
		}
	}
}

// collectMetrics collects performance metrics during the test
func (lt *LoadTester) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	var lastMessagesReceived int64
	
	for {
		select {
		case <-ticker.C:
			currentMessages := atomic.LoadInt64(&lt.messagesReceived)
			messagesThisSecond := currentMessages - lastMessagesReceived
			atomic.StoreInt64(&lt.messagesPerSecond, messagesThisSecond)
			lastMessagesReceived = currentMessages
			
		case <-ctx.Done():
			return
		case <-lt.stopCh:
			return
		}
	}
}

// updateLatencyMetrics updates latency statistics
func (lt *LoadTester) updateLatencyMetrics(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if latency > time.Duration(lt.maxLatency) {
		lt.maxLatency = int64(latency)
	}
	
	if latency < time.Duration(lt.minLatency) {
		lt.minLatency = int64(latency)
	}
	
	// Simple average calculation (in production, use proper statistical methods)
	currentAvg := time.Duration(lt.avgLatency)
	lt.avgLatency = int64((currentAvg + latency) / 2)
}

// calculateResults calculates the final test results
func (lt *LoadTester) calculateResults(duration time.Duration) LoadTestResult {
	totalMessages := atomic.LoadInt64(&lt.messagesReceived)
	avgMsgPerSec := float64(totalMessages) / duration.Seconds()
	
	return LoadTestResult{
		TotalConnections:      lt.totalConnections,
		SuccessfulConnections: atomic.LoadInt64(&lt.successfulConnections),
		FailedConnections:     atomic.LoadInt64(&lt.failedConnections),
		MessagesReceived:      totalMessages,
		AvgMessagesPerSecond:  avgMsgPerSec,
		AvgLatency:            time.Duration(atomic.LoadInt64(&lt.avgLatency)),
		MaxLatency:            time.Duration(atomic.LoadInt64(&lt.maxLatency)),
		MinLatency:            time.Duration(atomic.LoadInt64(&lt.minLatency)),
		TestDuration:          duration,
		ConnectionsPerSecond:  lt.connectionsPerSec,
	}
}

// RunWebSocketLoadTest performs load testing on WebSocket connections (for comparison)
func (lt *LoadTester) RunWebSocketLoadTest(ctx context.Context) (*LoadTestResult, error) {
	log.Printf("Starting WebSocket load test: %d connections", lt.totalConnections)
	
	startTime := time.Now()
	var wg sync.WaitGroup
	
	for i := 0; i < lt.totalConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lt.createWebSocketConnection(ctx, id)
		}(i)
		
		// Rate limiting
		if lt.connectionsPerSec > 0 {
			time.Sleep(time.Second / time.Duration(lt.connectionsPerSec))
		}
	}
	
	// Wait for test duration
	testTimer := time.NewTimer(lt.testDuration)
	select {
	case <-testTimer.C:
	case <-ctx.Done():
	}
	
	close(lt.stopCh)
	wg.Wait()
	
	result := lt.calculateResults(time.Since(startTime))
	return &result, nil
}

// createWebSocketConnection creates a WebSocket connection for testing
func (lt *LoadTester) createWebSocketConnection(ctx context.Context, id int) {
	wsURL := fmt.Sprintf("ws://localhost:8080/api/v1/realtime/ws")
	if lt.contestID != "" {
		wsURL += "?contest_id=" + lt.contestID
	}
	
	headers := http.Header{}
	if lt.authToken != "" {
		headers.Set("Authorization", "Bearer "+lt.authToken)
	}
	
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}
	
	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		atomic.AddInt64(&lt.failedConnections, 1)
		return
	}
	defer conn.Close()
	
	atomic.AddInt64(&lt.successfulConnections, 1)
	
	// Read messages until test ends
	for {
		select {
		case <-ctx.Done():
			return
		case <-lt.stopCh:
			return
		default:
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			atomic.AddInt64(&lt.messagesReceived, 1)
		}
	}
}

// Utility functions

func isTimeoutError(err error) bool {
	// Check if error is a timeout
	if netErr, ok := err.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}
	return false
}

// LoadTestConfig represents load test configuration
type LoadTestConfig struct {
	BaseURL           string        `json:"base_url"`
	TotalConnections  int           `json:"total_connections"`
	ConnectionsPerSec int           `json:"connections_per_sec"`
	TestDuration      time.Duration `json:"test_duration"`
	AuthToken         string        `json:"auth_token,omitempty"`
	ContestID         string        `json:"contest_id,omitempty"`
	TestType          string        `json:"test_type"` // "sse" or "websocket"
}

// RunLoadTest runs a load test with the specified configuration
func RunLoadTest(ctx context.Context, config LoadTestConfig) (*LoadTestResult, error) {
	tester := NewLoadTester(
		config.BaseURL,
		config.TotalConnections,
		config.ConnectionsPerSec,
		config.TestDuration,
	)
	
	if config.AuthToken != "" {
		tester.SetAuthToken(config.AuthToken)
	}
	
	if config.ContestID != "" {
		tester.SetContestID(config.ContestID)
	}
	
	switch config.TestType {
	case "websocket", "ws":
		return tester.RunWebSocketLoadTest(ctx)
	case "sse", "":
		fallthrough
	default:
		return tester.RunSSELoadTest(ctx)
	}
}