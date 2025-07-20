package realtime

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"competitive-programming-platform/pkg/database"
)

// LeaderboardAggregator handles leaderboard data aggregation and caching
type LeaderboardAggregator struct {
	db    *database.DB
	cache map[string]*CachedLeaderboard
	mutex sync.RWMutex
}

// CachedLeaderboard represents a cached leaderboard
type CachedLeaderboard struct {
	ContestID   string
	Leaderboard []LeaderboardEntry
	LastUpdated time.Time
	Version     int
}

// NewLeaderboardAggregator creates a new leaderboard aggregator
func NewLeaderboardAggregator(db *database.DB) *LeaderboardAggregator {
	return &LeaderboardAggregator{
		db:    db,
		cache: make(map[string]*CachedLeaderboard),
	}
}

// StartAggregation starts the leaderboard aggregation process
func (la *LeaderboardAggregator) StartAggregation(ctx context.Context) {
	log.Println("Starting leaderboard aggregator")
	
	// Periodic cache cleanup
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			la.cleanupCache()
		case <-ctx.Done():
			log.Println("Leaderboard aggregator shutting down")
			return
		}
	}
}

// GetLeaderboard returns the current leaderboard for a contest
func (la *LeaderboardAggregator) GetLeaderboard(ctx context.Context, contestID string) ([]LeaderboardEntry, error) {
	la.mutex.RLock()
	cached, exists := la.cache[contestID]
	la.mutex.RUnlock()

	// Check if cache is valid (updated within last 30 seconds)
	if exists && time.Since(cached.LastUpdated) < 30*time.Second {
		return cached.Leaderboard, nil
	}

	// Calculate fresh leaderboard
	leaderboard, err := la.calculateLeaderboard(ctx, contestID)
	if err != nil {
		return nil, err
	}

	// Update cache
	la.mutex.Lock()
	la.cache[contestID] = &CachedLeaderboard{
		ContestID:   contestID,
		Leaderboard: leaderboard,
		LastUpdated: time.Now(),
		Version:     1,
	}
	if cached != nil {
		la.cache[contestID].Version = cached.Version + 1
	}
	la.mutex.Unlock()

	return leaderboard, nil
}

// InvalidateLeaderboard invalidates the cached leaderboard for a contest
func (la *LeaderboardAggregator) InvalidateLeaderboard(contestID string) {
	la.mutex.Lock()
	defer la.mutex.Unlock()
	delete(la.cache, contestID)
}

// cleanupCache removes old cached leaderboards
func (la *LeaderboardAggregator) cleanupCache() {
	la.mutex.Lock()
	defer la.mutex.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for contestID, cached := range la.cache {
		if cached.LastUpdated.Before(cutoff) {
			delete(la.cache, contestID)
			log.Printf("Cleaned up cached leaderboard for contest %s", contestID)
		}
	}
}

// calculateLeaderboard calculates the current leaderboard for a contest
func (la *LeaderboardAggregator) calculateLeaderboard(ctx context.Context, contestID string) ([]LeaderboardEntry, error) {
	// Use the optimized leaderboard calculation function
	query := `SELECT * FROM calculate_contest_leaderboard($1)`

	rows, err := la.db.Pool.Query(ctx, query, contestID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate leaderboard: %w", err)
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	rank := 1

	for rows.Next() {
		var entry LeaderboardEntry
		var lastSubmission sql.NullTime
		var problemResultsJSON string

		err := rows.Scan(
			&entry.UserID, &entry.Username, &entry.FullName,
			&entry.TotalPoints, &entry.TotalPenalty, &entry.ProblemsSolved,
			&lastSubmission, &problemResultsJSON,
		)
		if err != nil {
			log.Printf("Error scanning leaderboard entry: %v", err)
			continue
		}

		entry.Rank = rank
		rank++

		if lastSubmission.Valid {
			entry.LastSubmission = lastSubmission.Time
		}

		// Parse problem results
		entry.ProblemResults = la.parseProblemResults(problemResultsJSON)
		leaderboard = append(leaderboard, entry)
	}

	return leaderboard, rows.Err()
}

// parseProblemResults parses the JSON problem results
func (la *LeaderboardAggregator) parseProblemResults(jsonStr string) []LeaderboardProblemResult {
	var results []LeaderboardProblemResult
	
	if jsonStr == "" || jsonStr == "[]" {
		return results
	}

	// This would be a full JSON parsing implementation
	// For now, returning empty slice - in production, implement full JSON parsing
	return results
}

// GetLeaderboardDelta returns changes since a specific version
func (la *LeaderboardAggregator) GetLeaderboardDelta(ctx context.Context, contestID string, fromVersion int) (*LeaderboardDelta, error) {
	la.mutex.RLock()
	cached, exists := la.cache[contestID]
	la.mutex.RUnlock()

	if !exists || cached.Version <= fromVersion {
		return &LeaderboardDelta{
			ContestID:   contestID,
			FromVersion: fromVersion,
			ToVersion:   fromVersion,
			Changes:     []LeaderboardChange{},
		}, nil
	}

	// For simplicity, return full leaderboard as changes
	// In production, implement proper delta calculation
	changes := make([]LeaderboardChange, len(cached.Leaderboard))
	for i, entry := range cached.Leaderboard {
		changes[i] = LeaderboardChange{
			Type:  "update",
			Entry: entry,
		}
	}

	return &LeaderboardDelta{
		ContestID:   contestID,
		FromVersion: fromVersion,
		ToVersion:   cached.Version,
		Changes:     changes,
		Timestamp:   cached.LastUpdated,
	}, nil
}

// LeaderboardDelta represents changes in leaderboard
type LeaderboardDelta struct {
	ContestID   string              `json:"contest_id"`
	FromVersion int                 `json:"from_version"`
	ToVersion   int                 `json:"to_version"`
	Changes     []LeaderboardChange `json:"changes"`
	Timestamp   time.Time           `json:"timestamp"`
}

// LeaderboardChange represents a single change in leaderboard
type LeaderboardChange struct {
	Type  string           `json:"type"` // "update", "add", "remove"
	Entry LeaderboardEntry `json:"entry"`
}

// UpdateSubmissionResult updates a submission result and triggers leaderboard recalculation
func (la *LeaderboardAggregator) UpdateSubmissionResult(ctx context.Context, contestID, userID, problemID, submissionID string, accepted bool, penalty int) error {
	// Begin transaction
	tx, err := la.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update contest submission result
	_, err = tx.Exec(ctx, `
		UPDATE contest_submissions 
		SET verdict = CASE WHEN $5 THEN 'AC' ELSE 'WA' END,
		    points = CASE WHEN $5 THEN 
		        (SELECT points FROM contest_problems WHERE contest_id = $1 AND problem_id = $3)
		        ELSE 0 END,
		    penalty_minutes = $6
		WHERE contest_id = $1 AND user_id = $2 AND problem_id = $3 AND submission_id = $4
	`, contestID, userID, problemID, submissionID, accepted, penalty)
	
	if err != nil {
		return fmt.Errorf("failed to update contest submission: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Invalidate cached leaderboard
	la.InvalidateLeaderboard(contestID)

	return nil
}

// GetContestStats returns statistics for a contest leaderboard
func (la *LeaderboardAggregator) GetContestStats(ctx context.Context, contestID string) (*ContestLeaderboardStats, error) {
	query := `
		SELECT 
			COUNT(DISTINCT cr.user_id) as total_participants,
			COUNT(DISTINCT cs.submission_id) as total_submissions,
			COUNT(DISTINCT cp.problem_id) as total_problems,
			COALESCE(AVG(user_scores.total_points), 0) as average_score,
			COALESCE(COUNT(CASE WHEN user_scores.problems_solved > 0 THEN 1 END) * 100.0 / 
				NULLIF(COUNT(DISTINCT cr.user_id), 0), 0) as participation_rate
		FROM contest_registrations cr
		LEFT JOIN contest_submissions cs ON cr.contest_id = cs.contest_id AND cr.user_id = cs.user_id
		LEFT JOIN contest_problems cp ON cr.contest_id = cp.contest_id
		LEFT JOIN (
			SELECT 
				cs.user_id,
				SUM(CASE WHEN s.status = 'AC' THEN cp.points ELSE 0 END) as total_points,
				COUNT(DISTINCT CASE WHEN s.status = 'AC' THEN cs.problem_id END) as problems_solved
			FROM contest_submissions cs
			JOIN submissions s ON cs.submission_id = s.id
			JOIN contest_problems cp ON cs.contest_id = cp.contest_id AND cs.problem_id = cp.problem_id
			WHERE cs.contest_id = $1
			GROUP BY cs.user_id
		) user_scores ON cr.user_id = user_scores.user_id
		WHERE cr.contest_id = $1
	`

	var stats ContestLeaderboardStats
	err := la.db.Pool.QueryRow(ctx, query, contestID).Scan(
		&stats.TotalParticipants,
		&stats.TotalSubmissions,
		&stats.TotalProblems,
		&stats.AverageScore,
		&stats.ParticipationRate,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get contest stats: %w", err)
	}

	stats.ContestID = contestID
	stats.LastUpdated = time.Now()

	return &stats, nil
}

// ContestLeaderboardStats represents contest leaderboard statistics
type ContestLeaderboardStats struct {
	ContestID         string    `json:"contest_id"`
	TotalParticipants int       `json:"total_participants"`
	TotalSubmissions  int       `json:"total_submissions"`
	TotalProblems     int       `json:"total_problems"`
	AverageScore      float64   `json:"average_score"`
	ParticipationRate float64   `json:"participation_rate"`
	LastUpdated       time.Time `json:"last_updated"`
}

// CreateLeaderboardSnapshot creates a snapshot of the current leaderboard
func (la *LeaderboardAggregator) CreateLeaderboardSnapshot(ctx context.Context, contestID string) error {
	leaderboard, err := la.GetLeaderboard(ctx, contestID)
	if err != nil {
		return err
	}

	// Begin transaction
	tx, err := la.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Clear previous snapshot
	_, err = tx.Exec(ctx, `DELETE FROM contest_leaderboard_snapshots WHERE contest_id = $1`, contestID)
	if err != nil {
		return err
	}

	// Insert new snapshot
	for _, entry := range leaderboard {
		_, err = tx.Exec(ctx, `
			INSERT INTO contest_leaderboard_snapshots 
			(contest_id, user_id, rank, total_points, total_penalty, problems_solved, snapshot_time)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
		`, contestID, entry.UserID, entry.Rank, entry.TotalPoints, entry.TotalPenalty, entry.ProblemsSolved)
		
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}