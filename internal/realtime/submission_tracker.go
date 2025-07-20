package realtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"competitive-programming-platform/pkg/database"
)

// SubmissionTracker tracks submission status changes and broadcasts updates
type SubmissionTracker struct {
	db      *database.DB
	service *Service
}

// NewSubmissionTracker creates a new submission tracker
func NewSubmissionTracker(db *database.DB, service *Service) *SubmissionTracker {
	return &SubmissionTracker{
		db:      db,
		service: service,
	}
}

// StartTracking starts tracking submission status changes
func (st *SubmissionTracker) StartTracking(ctx context.Context) {
	log.Println("Starting submission tracker")
	
	// Start polling for submission updates
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastCheck := time.Now().Add(-1 * time.Minute) // Start with 1 minute ago

	for {
		select {
		case <-ticker.C:
			if err := st.checkSubmissionUpdates(ctx, lastCheck); err != nil {
				log.Printf("Error checking submission updates: %v", err)
			}
			lastCheck = time.Now()

		case <-ctx.Done():
			log.Println("Submission tracker shutting down")
			return
		}
	}
}

// checkSubmissionUpdates checks for recent submission updates and broadcasts them
func (st *SubmissionTracker) checkSubmissionUpdates(ctx context.Context, since time.Time) error {
	query := `
		SELECT s.id, s.user_id, s.problem_id, s.source_code, s.language, 
		       s.status, s.verdict, s.execution_time, s.memory_usage, s.score,
		       s.test_cases_passed, s.total_test_cases, s.updated_at,
		       cs.contest_id
		FROM submissions s
		LEFT JOIN contest_submissions cs ON s.id = cs.submission_id
		WHERE s.updated_at > $1
		ORDER BY s.updated_at ASC
	`

	rows, err := st.db.Pool.Query(ctx, query, since)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var submission struct {
			ID              string
			UserID          string
			ProblemID       string
			SourceCode      string
			Language        string
			Status          string
			Verdict         string
			ExecutionTime   sql.NullInt32
			MemoryUsage     sql.NullInt32
			Score           int
			TestCasesPassed int
			TotalTestCases  int
			UpdatedAt       time.Time
			ContestID       sql.NullString
		}

		err := rows.Scan(
			&submission.ID, &submission.UserID, &submission.ProblemID,
			&submission.SourceCode, &submission.Language, &submission.Status,
			&submission.Verdict, &submission.ExecutionTime, &submission.MemoryUsage,
			&submission.Score, &submission.TestCasesPassed, &submission.TotalTestCases,
			&submission.UpdatedAt, &submission.ContestID,
		)
		if err != nil {
			log.Printf("Error scanning submission: %v", err)
			continue
		}

		// Create submission update
		update := SubmissionStatusUpdate{
			SubmissionID:    submission.ID,
			UserID:          submission.UserID,
			ProblemID:       submission.ProblemID,
			Status:          submission.Status,
			Verdict:         submission.Verdict,
			Score:           submission.Score,
			TestCasesPassed: submission.TestCasesPassed,
			TotalTestCases:  submission.TotalTestCases,
			Language:        submission.Language,
			Timestamp:       submission.UpdatedAt,
		}

		if submission.ExecutionTime.Valid {
			update.ExecutionTime = int(submission.ExecutionTime.Int32)
		}

		if submission.MemoryUsage.Valid {
			update.MemoryUsage = int(submission.MemoryUsage.Int32)
		}

		if submission.ContestID.Valid {
			update.ContestID = submission.ContestID.String
		}

		// Broadcast the update
		st.service.BroadcastSubmissionUpdate(update)

		// If this is a contest submission and it's accepted, update leaderboard
		if submission.ContestID.Valid && submission.Status == "AC" {
			go st.updateContestLeaderboard(ctx, submission.ContestID.String)
		}
	}

	return rows.Err()
}

// updateContestLeaderboard updates and broadcasts contest leaderboard
func (st *SubmissionTracker) updateContestLeaderboard(ctx context.Context, contestID string) {
	leaderboard, err := st.calculateContestLeaderboard(ctx, contestID)
	if err != nil {
		log.Printf("Error calculating leaderboard for contest %s: %v", contestID, err)
		return
	}

	update := LeaderboardUpdate{
		ContestID:  contestID,
		Rankings:   leaderboard,
		Timestamp:  time.Now(),
		UpdateType: "full", // For now, always send full updates
	}

	st.service.BroadcastLeaderboardUpdate(update)
}

// calculateContestLeaderboard calculates the current leaderboard for a contest
func (st *SubmissionTracker) calculateContestLeaderboard(ctx context.Context, contestID string) ([]LeaderboardEntry, error) {
	query := `
		WITH contest_problems AS (
			SELECT cp.problem_id, cp.problem_order, cp.points
			FROM contest_problems cp
			WHERE cp.contest_id = $1
			ORDER BY cp.problem_order
		),
		user_submissions AS (
			SELECT DISTINCT cr.user_id, u.username, u.full_name
			FROM contest_registrations cr
			JOIN users u ON cr.user_id = u.id
			WHERE cr.contest_id = $1
		),
		problem_results AS (
			SELECT 
				us.user_id,
				cp.problem_id,
				cp.problem_order,
				cp.points,
				COUNT(cs.id) as attempts,
				BOOL_OR(s.status = 'AC') as solved,
				MIN(CASE WHEN s.status = 'AC' THEN cs.submitted_at END) as solve_time,
				MIN(CASE WHEN s.status = 'AC' THEN cs.penalty_minutes ELSE 0 END) as penalty_minutes,
				MAX(CASE WHEN s.status = 'AC' THEN cp.points ELSE 0 END) as earned_points
			FROM user_submissions us
			CROSS JOIN contest_problems cp
			LEFT JOIN contest_submissions cs ON us.user_id = cs.user_id 
				AND cp.problem_id = cs.problem_id 
				AND cs.contest_id = $1
			LEFT JOIN submissions s ON cs.submission_id = s.id
			GROUP BY us.user_id, cp.problem_id, cp.problem_order, cp.points
		),
		user_totals AS (
			SELECT 
				pr.user_id,
				SUM(pr.earned_points) as total_points,
				SUM(pr.penalty_minutes) as total_penalty,
				COUNT(CASE WHEN pr.solved THEN 1 END) as problems_solved,
				MAX(pr.solve_time) as last_submission
			FROM problem_results pr
			GROUP BY pr.user_id
		)
		SELECT 
			ut.user_id,
			us.username,
			us.full_name,
			COALESCE(ut.total_points, 0) as total_points,
			COALESCE(ut.total_penalty, 0) as total_penalty,
			COALESCE(ut.problems_solved, 0) as problems_solved,
			ut.last_submission,
			COALESCE(
				json_agg(
					json_build_object(
						'problem_id', pr.problem_id,
						'problem_order', pr.problem_order,
						'points', pr.earned_points,
						'attempts', pr.attempts,
						'solved', pr.solved,
						'solve_time', pr.solve_time,
						'penalty_minutes', pr.penalty_minutes
					) ORDER BY pr.problem_order
				), '[]'
			) as problem_results
		FROM user_submissions us
		LEFT JOIN user_totals ut ON us.user_id = ut.user_id
		LEFT JOIN problem_results pr ON us.user_id = pr.user_id
		GROUP BY ut.user_id, us.username, us.full_name, ut.total_points, 
				 ut.total_penalty, ut.problems_solved, ut.last_submission
		ORDER BY 
			COALESCE(ut.total_points, 0) DESC,
			COALESCE(ut.total_penalty, 0) ASC,
			ut.last_submission ASC NULLS LAST
	`

	rows, err := st.db.Pool.Query(ctx, query, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	rank := 1

	for rows.Next() {
		var entry LeaderboardEntry
		var problemResultsJSON string
		var lastSubmission sql.NullTime

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

		// Parse problem results JSON
		if problemResultsJSON != "[]" {
			var problemResults []map[string]interface{}
			if err := json.Unmarshal([]byte(problemResultsJSON), &problemResults); err == nil {
				for _, pr := range problemResults {
					result := LeaderboardProblemResult{
						ProblemID:      getString(pr, "problem_id"),
						ProblemOrder:   getInt(pr, "problem_order"),
						Points:         getInt(pr, "points"),
						Attempts:       getInt(pr, "attempts"),
						Solved:         getBool(pr, "solved"),
						PenaltyMinutes: getInt(pr, "penalty_minutes"),
					}

					if solveTimeStr := getString(pr, "solve_time"); solveTimeStr != "" {
						if solveTime, err := time.Parse(time.RFC3339, solveTimeStr); err == nil {
							result.SolveTime = &solveTime
						}
					}

					entry.ProblemResults = append(entry.ProblemResults, result)
				}
			}
		}

		leaderboard = append(leaderboard, entry)
	}

	return leaderboard, rows.Err()
}

// Helper functions for JSON parsing
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case int64:
			return int(val)
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// TrackSubmissionStatus manually tracks a submission status change
func (st *SubmissionTracker) TrackSubmissionStatus(submissionID, userID, problemID, contestID, status, verdict string) {
	update := SubmissionStatusUpdate{
		SubmissionID: submissionID,
		UserID:       userID,
		ProblemID:    problemID,
		ContestID:    contestID,
		Status:       status,
		Verdict:      verdict,
		Timestamp:    time.Now(),
	}

	st.service.BroadcastSubmissionUpdate(update)

	// If this is a contest submission and it's accepted, update leaderboard
	if contestID != "" && status == "AC" {
		go st.updateContestLeaderboard(context.Background(), contestID)
	}
}