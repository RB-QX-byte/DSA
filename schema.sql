-- Competitive Programming Platform - Initial Database Schema
-- This schema creates the foundational tables for user and problem management

-- Enable UUID extension for generating unique IDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create users table that extends Supabase auth.users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    username VARCHAR(50) UNIQUE NOT NULL,
    full_name VARCHAR(100),
    email VARCHAR(255) UNIQUE NOT NULL,
    rating INTEGER DEFAULT 1200,
    max_rating INTEGER DEFAULT 1200,
    problems_solved INTEGER DEFAULT 0,
    contests_participated INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create problems table for competitive programming problems
CREATE TABLE IF NOT EXISTS problems (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    difficulty INTEGER NOT NULL CHECK (difficulty >= 800 AND difficulty <= 3500),
    time_limit INTEGER NOT NULL DEFAULT 1000, -- in milliseconds
    memory_limit INTEGER NOT NULL DEFAULT 256, -- in MB
    tags TEXT[], -- array of problem tags like 'dp', 'greedy', 'graphs'
    acceptance_rate DECIMAL(5,2) DEFAULT 0.00,
    total_submissions INTEGER DEFAULT 0,
    accepted_submissions INTEGER DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create test_cases table for problem test cases
CREATE TABLE IF NOT EXISTS test_cases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    input_data TEXT NOT NULL,
    expected_output TEXT NOT NULL,
    is_sample BOOLEAN DEFAULT FALSE,
    points INTEGER DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create submissions table for tracking user submissions
CREATE TABLE IF NOT EXISTS submissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    source_code TEXT NOT NULL,
    language VARCHAR(50) NOT NULL,
    status VARCHAR(20) DEFAULT 'PE', -- PE = Pending, AC = Accepted, WA = Wrong Answer, etc.
    verdict VARCHAR(20) DEFAULT 'PE', -- Same as status for compatibility
    execution_time INTEGER, -- in milliseconds
    memory_usage INTEGER, -- in KB
    memory_used INTEGER, -- in KB (for backward compatibility)
    score INTEGER DEFAULT 0,
    test_cases_run INTEGER DEFAULT 0,
    test_cases_passed INTEGER DEFAULT 0,
    total_test_cases INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for optimized queries
-- Users table indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_rating ON users(rating);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- Problems table indexes
CREATE INDEX IF NOT EXISTS idx_problems_difficulty ON problems(difficulty);
CREATE INDEX IF NOT EXISTS idx_problems_tags ON problems USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_problems_slug ON problems(slug);
CREATE INDEX IF NOT EXISTS idx_problems_created_at ON problems(created_at);
CREATE INDEX IF NOT EXISTS idx_problems_acceptance_rate ON problems(acceptance_rate);

-- Test cases table indexes
CREATE INDEX IF NOT EXISTS idx_test_cases_problem_id ON test_cases(problem_id);
CREATE INDEX IF NOT EXISTS idx_test_cases_is_sample ON test_cases(is_sample);

-- Submissions table indexes
CREATE INDEX IF NOT EXISTS idx_submissions_user_id ON submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_problem_id ON submissions(problem_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);
CREATE INDEX IF NOT EXISTS idx_submissions_verdict ON submissions(verdict);
CREATE INDEX IF NOT EXISTS idx_submissions_created_at ON submissions(created_at);
CREATE INDEX IF NOT EXISTS idx_submissions_user_problem ON submissions(user_id, problem_id);

-- Create functions for updating timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for auto-updating timestamps
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_problems_updated_at 
    BEFORE UPDATE ON problems 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_submissions_updated_at 
    BEFORE UPDATE ON submissions 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to update problem statistics
CREATE OR REPLACE FUNCTION update_problem_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'AC' AND (OLD.status IS NULL OR OLD.status != 'AC') THEN
        UPDATE problems 
        SET 
            accepted_submissions = accepted_submissions + 1,
            total_submissions = total_submissions + 1,
            acceptance_rate = ROUND(
                (accepted_submissions + 1) * 100.0 / (total_submissions + 1), 2
            )
        WHERE id = NEW.problem_id;
        
        -- Update user's problems solved count
        UPDATE users 
        SET problems_solved = problems_solved + 1
        WHERE id = NEW.user_id;
    ELSIF NEW.status != 'AC' AND OLD.status IS NULL THEN
        UPDATE problems 
        SET 
            total_submissions = total_submissions + 1,
            acceptance_rate = ROUND(
                accepted_submissions * 100.0 / (total_submissions + 1), 2
            )
        WHERE id = NEW.problem_id;
    END IF;
    
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for updating problem statistics
CREATE TRIGGER update_problem_stats_trigger
    AFTER INSERT OR UPDATE ON submissions
    FOR EACH ROW
    EXECUTE FUNCTION update_problem_stats();

-- Insert sample data for testing
INSERT INTO problems (title, slug, description, difficulty, time_limit, memory_limit, tags) VALUES
    ('Two Sum', 'two-sum', 'Given an array of integers nums and an integer target, return indices of the two numbers such that they add up to target.', 800, 1000, 256, ARRAY['array', 'hash-table']),
    ('Maximum Subarray', 'maximum-subarray', 'Given an integer array nums, find the contiguous subarray (containing at least one number) which has the largest sum and return its sum.', 1200, 1000, 256, ARRAY['array', 'dynamic-programming']),
    ('Binary Tree Inorder Traversal', 'binary-tree-inorder-traversal', 'Given the root of a binary tree, return the inorder traversal of its nodes values.', 1000, 1000, 256, ARRAY['tree', 'stack', 'recursion'])
ON CONFLICT (slug) DO NOTHING;

-- Insert test cases for the sample problems
INSERT INTO test_cases (problem_id, input_data, expected_output, is_sample) 
SELECT 
    p.id, 
    '4
2 7 11 15
9', 
    '0 1', 
    TRUE
FROM problems p WHERE p.slug = 'two-sum'
ON CONFLICT DO NOTHING;

INSERT INTO test_cases (problem_id, input_data, expected_output, is_sample) 
SELECT 
    p.id, 
    '9
-2 1 -3 4 -1 2 1 -5 4', 
    '6', 
    TRUE
FROM problems p WHERE p.slug = 'maximum-subarray'
ON CONFLICT DO NOTHING;

-- Create RLS policies (to be enabled later)
-- Enable RLS on tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE problems ENABLE ROW LEVEL SECURITY;
ALTER TABLE submissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_cases ENABLE ROW LEVEL SECURITY;

-- Create policies for users table
CREATE POLICY "Users can view their own profile" ON users
    FOR SELECT USING (auth.uid() = id);

CREATE POLICY "Users can update their own profile" ON users
    FOR UPDATE USING (auth.uid() = id);

-- Create policies for problems table
CREATE POLICY "Anyone can view problems" ON problems
    FOR SELECT USING (true);

CREATE POLICY "Only authenticated users can create problems" ON problems
    FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);

-- Create policies for submissions table
CREATE POLICY "Users can view their own submissions" ON submissions
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can create their own submissions" ON submissions
    FOR INSERT WITH CHECK (auth.uid() = user_id);

-- Create policies for test_cases table
CREATE POLICY "Anyone can view sample test cases" ON test_cases
    FOR SELECT USING (is_sample = TRUE);

CREATE POLICY "Only authenticated users can view all test cases" ON test_cases
    FOR SELECT USING (auth.uid() IS NOT NULL);

-- Create contests table for contest management
CREATE TABLE IF NOT EXISTS contests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    rules TEXT,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    registration_start TIMESTAMP WITH TIME ZONE,
    registration_end TIMESTAMP WITH TIME ZONE,
    max_participants INTEGER,
    status VARCHAR(50) DEFAULT 'upcoming', -- upcoming, live, ended, cancelled
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT valid_contest_times CHECK (end_time > start_time),
    CONSTRAINT valid_registration_times CHECK (registration_end IS NULL OR registration_start IS NULL OR registration_end > registration_start)
);

-- Create contest_problems table to link problems to contests
CREATE TABLE IF NOT EXISTS contest_problems (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    problem_order INTEGER NOT NULL,
    points INTEGER DEFAULT 100,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(contest_id, problem_id),
    UNIQUE(contest_id, problem_order)
);

-- Create contest_registrations table to track user registrations
CREATE TABLE IF NOT EXISTS contest_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(contest_id, user_id)
);

-- Create contest_submissions table to track submissions within contests
CREATE TABLE IF NOT EXISTS contest_submissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    verdict VARCHAR(20),
    points INTEGER DEFAULT 0,
    penalty_minutes INTEGER DEFAULT 0,
    UNIQUE(submission_id)
);

-- Create contest-related indexes
CREATE INDEX IF NOT EXISTS idx_contests_status ON contests(status);
CREATE INDEX IF NOT EXISTS idx_contests_start_time ON contests(start_time);
CREATE INDEX IF NOT EXISTS idx_contests_end_time ON contests(end_time);
CREATE INDEX IF NOT EXISTS idx_contests_created_by ON contests(created_by);

CREATE INDEX IF NOT EXISTS idx_contest_problems_contest_id ON contest_problems(contest_id);
CREATE INDEX IF NOT EXISTS idx_contest_problems_problem_id ON contest_problems(problem_id);
CREATE INDEX IF NOT EXISTS idx_contest_problems_order ON contest_problems(contest_id, problem_order);

CREATE INDEX IF NOT EXISTS idx_contest_registrations_contest_id ON contest_registrations(contest_id);
CREATE INDEX IF NOT EXISTS idx_contest_registrations_user_id ON contest_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_contest_registrations_registered_at ON contest_registrations(registered_at);

CREATE INDEX IF NOT EXISTS idx_contest_submissions_contest_id ON contest_submissions(contest_id);
CREATE INDEX IF NOT EXISTS idx_contest_submissions_user_id ON contest_submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_contest_submissions_problem_id ON contest_submissions(problem_id);
CREATE INDEX IF NOT EXISTS idx_contest_submissions_submitted_at ON contest_submissions(submitted_at);

-- Create triggers for contest tables
CREATE TRIGGER update_contests_updated_at 
    BEFORE UPDATE ON contests 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to update contest status based on time
CREATE OR REPLACE FUNCTION update_contest_status()
RETURNS TRIGGER AS $$
BEGIN
    -- Update contest status based on current time
    IF NEW.start_time <= NOW() AND NEW.end_time > NOW() THEN
        NEW.status = 'live';
    ELSIF NEW.end_time <= NOW() THEN
        NEW.status = 'ended';
    ELSE
        NEW.status = 'upcoming';
    END IF;
    
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to auto-update contest status
CREATE TRIGGER update_contest_status_trigger
    BEFORE INSERT OR UPDATE ON contests
    FOR EACH ROW
    EXECUTE FUNCTION update_contest_status();

-- Create function to calculate contest submission penalty
CREATE OR REPLACE FUNCTION calculate_submission_penalty(
    p_contest_id UUID,
    p_user_id UUID,
    p_problem_id UUID,
    p_submission_time TIMESTAMP WITH TIME ZONE
) RETURNS INTEGER AS $$
DECLARE
    contest_start TIMESTAMP WITH TIME ZONE;
    wrong_attempts INTEGER;
    penalty_minutes INTEGER;
BEGIN
    -- Get contest start time
    SELECT start_time INTO contest_start 
    FROM contests 
    WHERE id = p_contest_id;
    
    -- Count wrong attempts before this submission
    SELECT COUNT(*) INTO wrong_attempts
    FROM contest_submissions cs
    JOIN submissions s ON cs.submission_id = s.id
    WHERE cs.contest_id = p_contest_id
    AND cs.user_id = p_user_id
    AND cs.problem_id = p_problem_id
    AND s.status != 'AC'
    AND cs.submitted_at < p_submission_time;
    
    -- Calculate penalty: 20 minutes per wrong attempt
    penalty_minutes = wrong_attempts * 20;
    
    -- Add time penalty (minutes from contest start)
    penalty_minutes = penalty_minutes + EXTRACT(EPOCH FROM (p_submission_time - contest_start)) / 60;
    
    RETURN penalty_minutes;
END;
$$ language 'plpgsql';

-- Create RLS policies for contest tables
ALTER TABLE contests ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_problems ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_submissions ENABLE ROW LEVEL SECURITY;

-- Contest policies
CREATE POLICY "Anyone can view contests" ON contests
    FOR SELECT USING (true);

CREATE POLICY "Only authenticated users can create contests" ON contests
    FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);

CREATE POLICY "Contest creators can update their contests" ON contests
    FOR UPDATE USING (auth.uid() = created_by);

CREATE POLICY "Contest creators can delete their contests" ON contests
    FOR DELETE USING (auth.uid() = created_by);

-- Contest problems policies
CREATE POLICY "Anyone can view contest problems" ON contest_problems
    FOR SELECT USING (true);

CREATE POLICY "Contest creators can manage contest problems" ON contest_problems
    FOR ALL USING (
        EXISTS (
            SELECT 1 FROM contests c 
            WHERE c.id = contest_id AND c.created_by = auth.uid()
        )
    );

-- Contest registrations policies
CREATE POLICY "Users can view contest registrations" ON contest_registrations
    FOR SELECT USING (true);

CREATE POLICY "Users can register for contests" ON contest_registrations
    FOR INSERT WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can cancel their own registrations" ON contest_registrations
    FOR DELETE USING (auth.uid() = user_id);

-- Contest submissions policies
CREATE POLICY "Users can view contest submissions" ON contest_submissions
    FOR SELECT USING (true);

CREATE POLICY "Users can create contest submissions" ON contest_submissions
    FOR INSERT WITH CHECK (auth.uid() = user_id);

-- Insert sample contest data
INSERT INTO contests (title, description, rules, start_time, end_time, registration_start, registration_end, max_participants) VALUES
    ('Weekly Contest #1', 'A weekly competitive programming contest with 3 problems', 'Standard ICPC rules apply. No external libraries allowed.', 
     NOW() + INTERVAL '1 hour', NOW() + INTERVAL '3 hours', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 minutes', 1000),
    ('Monthly Challenge', 'Monthly contest with harder problems', 'Extended contest with partial scoring allowed.',
     NOW() + INTERVAL '1 day', NOW() + INTERVAL '1 day 2 hours', NOW() - INTERVAL '1 week', NOW() + INTERVAL '1 day', 500),
    ('Beginner Contest', 'Contest for beginners with easy problems', 'Open to all skill levels. Educational contest.',
     NOW() + INTERVAL '2 days', NOW() + INTERVAL '2 days 90 minutes', NOW(), NOW() + INTERVAL '2 days', 2000)
ON CONFLICT DO NOTHING;

-- Create leaderboard snapshots table for historical data
CREATE TABLE IF NOT EXISTS contest_leaderboard_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rank INTEGER NOT NULL,
    total_points INTEGER DEFAULT 0,
    total_penalty INTEGER DEFAULT 0,
    problems_solved INTEGER DEFAULT 0,
    snapshot_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(contest_id, user_id, snapshot_time)
);

-- Create indexes for leaderboard snapshots
CREATE INDEX IF NOT EXISTS idx_contest_leaderboard_snapshots_contest_id ON contest_leaderboard_snapshots(contest_id);
CREATE INDEX IF NOT EXISTS idx_contest_leaderboard_snapshots_user_id ON contest_leaderboard_snapshots(user_id);
CREATE INDEX IF NOT EXISTS idx_contest_leaderboard_snapshots_snapshot_time ON contest_leaderboard_snapshots(snapshot_time);

-- Create optimized leaderboard calculation function
CREATE OR REPLACE FUNCTION calculate_contest_leaderboard(p_contest_id UUID)
RETURNS TABLE (
    user_id UUID,
    username VARCHAR(50),
    full_name VARCHAR(100),
    total_points INTEGER,
    total_penalty INTEGER,
    problems_solved INTEGER,
    last_submission TIMESTAMP WITH TIME ZONE,
    problem_results JSON
) AS $$
BEGIN
    RETURN QUERY
    WITH contest_problems AS (
        SELECT cp.problem_id, cp.problem_order, cp.points
        FROM contest_problems cp
        WHERE cp.contest_id = p_contest_id
        ORDER BY cp.problem_order
    ),
    user_submissions AS (
        SELECT DISTINCT cr.user_id, u.username, u.full_name
        FROM contest_registrations cr
        JOIN users u ON cr.user_id = u.id
        WHERE cr.contest_id = p_contest_id
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
            AND cs.contest_id = p_contest_id
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
        COALESCE(ut.total_points, 0)::INTEGER as total_points,
        COALESCE(ut.total_penalty, 0)::INTEGER as total_penalty,
        COALESCE(ut.problems_solved, 0)::INTEGER as problems_solved,
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
            ), '[]'::json
        ) as problem_results
    FROM user_submissions us
    LEFT JOIN user_totals ut ON us.user_id = ut.user_id
    LEFT JOIN problem_results pr ON us.user_id = pr.user_id
    GROUP BY ut.user_id, us.username, us.full_name, ut.total_points, 
             ut.total_penalty, ut.problems_solved, ut.last_submission
    ORDER BY 
        COALESCE(ut.total_points, 0) DESC,
        COALESCE(ut.total_penalty, 0) ASC,
        ut.last_submission ASC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

-- Create function to get contest leaderboard stats
CREATE OR REPLACE FUNCTION get_contest_leaderboard_stats(p_contest_id UUID)
RETURNS TABLE (
    total_participants INTEGER,
    total_submissions INTEGER,
    total_problems INTEGER,
    average_score NUMERIC,
    participation_rate NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(DISTINCT cr.user_id)::INTEGER as total_participants,
        COUNT(DISTINCT cs.submission_id)::INTEGER as total_submissions,
        COUNT(DISTINCT cp.problem_id)::INTEGER as total_problems,
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
        WHERE cs.contest_id = p_contest_id
        GROUP BY cs.user_id
    ) user_scores ON cr.user_id = user_scores.user_id
    WHERE cr.contest_id = p_contest_id;
END;
$$ LANGUAGE plpgsql;

-- Create function to update leaderboard efficiently
CREATE OR REPLACE FUNCTION update_contest_leaderboard_cache(p_contest_id UUID)
RETURNS VOID AS $$
BEGIN
    -- This function can be used to update materialized views or cache tables
    -- For now, it's a placeholder for future optimization
    PERFORM pg_notify('leaderboard_updated', p_contest_id::text);
END;
$$ LANGUAGE plpgsql;

-- Create trigger to notify leaderboard updates
CREATE OR REPLACE FUNCTION notify_leaderboard_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Only notify for contest submissions
    IF EXISTS (SELECT 1 FROM contest_submissions WHERE submission_id = NEW.id) THEN
        PERFORM pg_notify('submission_update', 
            json_build_object(
                'submission_id', NEW.id,
                'user_id', NEW.user_id,
                'status', NEW.status,
                'updated_at', NEW.updated_at
            )::text
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for submission updates
CREATE TRIGGER submission_update_notify_trigger
    AFTER UPDATE ON submissions
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION notify_leaderboard_update();

-- Performance Analytics Tables for Task 7
-- =====================================

-- User performance metrics table for tracking 15 key metrics over time
CREATE TABLE IF NOT EXISTS user_performance_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recorded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Problem-solving metrics (1-5)
    problem_solving_speed NUMERIC(10,4), -- Average time to solve problems (minutes)
    debugging_efficiency NUMERIC(5,4), -- Ratio of successful fixes to total debug attempts
    code_complexity_score NUMERIC(10,4), -- Average cyclomatic complexity of submitted code
    pattern_recognition_accuracy NUMERIC(5,4), -- Success rate in identifying problem patterns
    algorithm_selection_accuracy NUMERIC(5,4), -- Correctness of algorithm choice for problem type
    
    -- Contest performance metrics (6-10)
    contest_ranking_percentile NUMERIC(5,4), -- Percentile ranking in contests
    time_pressure_performance NUMERIC(5,4), -- Performance degradation under time pressure
    multi_problem_efficiency NUMERIC(5,4), -- Ability to handle multiple problems simultaneously
    contest_consistency NUMERIC(5,4), -- Variance in contest performance
    penalty_optimization NUMERIC(5,4), -- Ability to minimize time penalties
    
    -- Learning and adaptation metrics (11-15)
    learning_velocity NUMERIC(10,4), -- Rate of improvement over time
    knowledge_retention NUMERIC(5,4), -- Performance on previously solved problem types
    error_pattern_reduction NUMERIC(5,4), -- Reduction in recurring error patterns
    adaptive_strategy_usage NUMERIC(5,4), -- Use of different strategies based on problem type
    meta_cognitive_awareness NUMERIC(5,4), -- Self-assessment accuracy vs actual performance
    
    -- Supporting data
    total_submissions INTEGER DEFAULT 0,
    accepted_submissions INTEGER DEFAULT 0,
    problems_attempted INTEGER DEFAULT 0,
    contest_participations INTEGER DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Raw performance events table for ingestion pipeline
CREATE TABLE IF NOT EXISTS performance_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL, -- 'submission', 'contest_join', 'problem_view', etc.
    event_data JSONB NOT NULL, -- Flexible event data storage
    submission_id UUID REFERENCES submissions(id) ON DELETE SET NULL,
    contest_id UUID REFERENCES contests(id) ON DELETE SET NULL,
    problem_id UUID REFERENCES problems(id) ON DELETE SET NULL,
    recorded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE,
    
    -- Indexes for efficient querying
    INDEX idx_performance_events_user_id (user_id),
    INDEX idx_performance_events_type (event_type),
    INDEX idx_performance_events_recorded_at (recorded_at),
    INDEX idx_performance_events_processed (processed),
    INDEX idx_performance_events_data (event_data) USING GIN
);

-- User skill progression tracking for Bayesian model
CREATE TABLE IF NOT EXISTS user_skill_progression (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_category VARCHAR(100) NOT NULL, -- Maps to the 15 metrics
    skill_level NUMERIC(10,6) NOT NULL, -- Current estimated skill level
    confidence_interval_lower NUMERIC(10,6), -- Lower bound of confidence interval
    confidence_interval_upper NUMERIC(10,6), -- Upper bound of confidence interval
    prior_alpha NUMERIC(10,6), -- Bayesian prior parameters
    prior_beta NUMERIC(10,6),
    evidence_count INTEGER DEFAULT 0, -- Number of data points used for estimation
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(user_id, skill_category)
);

-- Performance analytics cache for dashboard
CREATE TABLE IF NOT EXISTS performance_analytics_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cache_key VARCHAR(255) NOT NULL, -- Type of cached data
    cache_data JSONB NOT NULL, -- Cached analytics results
    valid_until TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(user_id, cache_key),
    INDEX idx_performance_cache_user_id (user_id),
    INDEX idx_performance_cache_key (cache_key),
    INDEX idx_performance_cache_valid_until (valid_until)
);

-- Aggregated performance summaries by time period
CREATE TABLE IF NOT EXISTS performance_time_series (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    time_period VARCHAR(20) NOT NULL, -- 'daily', 'weekly', 'monthly'
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Aggregated metrics for the period
    avg_problem_solving_speed NUMERIC(10,4),
    avg_debugging_efficiency NUMERIC(5,4),
    total_submissions INTEGER DEFAULT 0,
    success_rate NUMERIC(5,4),
    improvement_trend NUMERIC(10,6), -- Trend indicator
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(user_id, time_period, period_start),
    INDEX idx_performance_time_series_user_period (user_id, time_period),
    INDEX idx_performance_time_series_start (period_start)
);

-- Performance indexes for optimization
CREATE INDEX IF NOT EXISTS idx_user_performance_metrics_user_id ON user_performance_metrics(user_id);
CREATE INDEX IF NOT EXISTS idx_user_performance_metrics_recorded_at ON user_performance_metrics(recorded_at);
CREATE INDEX IF NOT EXISTS idx_user_performance_metrics_user_recorded ON user_performance_metrics(user_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_user_skill_progression_user_id ON user_skill_progression(user_id);
CREATE INDEX IF NOT EXISTS idx_user_skill_progression_category ON user_skill_progression(skill_category);
CREATE INDEX IF NOT EXISTS idx_user_skill_progression_updated ON user_skill_progression(last_updated);

-- Performance analytics triggers
CREATE TRIGGER update_user_performance_metrics_on_submission
    AFTER INSERT OR UPDATE ON submissions
    FOR EACH ROW
    EXECUTE FUNCTION trigger_performance_event_ingestion();

-- Function to trigger performance event ingestion
CREATE OR REPLACE FUNCTION trigger_performance_event_ingestion()
RETURNS TRIGGER AS $$
BEGIN
    -- Insert performance event for further processing
    INSERT INTO performance_events (user_id, event_type, event_data, submission_id, problem_id)
    VALUES (
        NEW.user_id,
        'submission',
        jsonb_build_object(
            'submission_id', NEW.id,
            'problem_id', NEW.problem_id,
            'status', NEW.status,
            'execution_time', NEW.execution_time,
            'memory_usage', NEW.memory_usage,
            'language', NEW.language,
            'test_cases_passed', NEW.test_cases_passed,
            'total_test_cases', NEW.total_test_cases
        ),
        NEW.id,
        NEW.problem_id
    );
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to calculate problem solving speed
CREATE OR REPLACE FUNCTION calculate_problem_solving_speed(p_user_id UUID, p_problem_id UUID)
RETURNS NUMERIC AS $$
DECLARE
    first_attempt TIMESTAMP WITH TIME ZONE;
    accepted_time TIMESTAMP WITH TIME ZONE;
    solving_time_minutes NUMERIC;
BEGIN
    -- Get the time of first attempt and first acceptance
    SELECT MIN(created_at), MIN(CASE WHEN status = 'AC' THEN created_at END)
    INTO first_attempt, accepted_time
    FROM submissions
    WHERE user_id = p_user_id AND problem_id = p_problem_id;
    
    IF accepted_time IS NULL THEN
        RETURN NULL; -- Problem not solved yet
    END IF;
    
    solving_time_minutes = EXTRACT(EPOCH FROM (accepted_time - first_attempt)) / 60.0;
    
    RETURN solving_time_minutes;
END;
$$ LANGUAGE plpgsql;

-- Function to calculate debugging efficiency
CREATE OR REPLACE FUNCTION calculate_debugging_efficiency(p_user_id UUID, p_problem_id UUID)
RETURNS NUMERIC AS $$
DECLARE
    total_attempts INTEGER;
    successful_solve BOOLEAN;
    efficiency NUMERIC;
BEGIN
    SELECT COUNT(*), BOOL_OR(status = 'AC')
    INTO total_attempts, successful_solve
    FROM submissions
    WHERE user_id = p_user_id AND problem_id = p_problem_id;
    
    IF total_attempts = 0 OR NOT successful_solve THEN
        RETURN 0.0;
    END IF;
    
    -- Efficiency is inverse of attempts (1/attempts), normalized
    efficiency = 1.0 / total_attempts;
    
    RETURN LEAST(efficiency, 1.0);
END;
$$ LANGUAGE plpgsql;

-- Function to process performance events and update metrics
CREATE OR REPLACE FUNCTION process_performance_events()
RETURNS INTEGER AS $$
DECLARE
    event_record RECORD;
    processed_count INTEGER := 0;
BEGIN
    -- Process unprocessed events
    FOR event_record IN 
        SELECT * FROM performance_events 
        WHERE processed = FALSE 
        ORDER BY recorded_at ASC
        LIMIT 1000 -- Process in batches
    LOOP
        -- Process based on event type
        CASE event_record.event_type
            WHEN 'submission' THEN
                PERFORM update_user_metrics_from_submission(event_record);
            WHEN 'contest_join' THEN
                PERFORM update_user_metrics_from_contest(event_record);
            -- Add more event types as needed
        END CASE;
        
        -- Mark as processed
        UPDATE performance_events 
        SET processed = TRUE, processed_at = NOW()
        WHERE id = event_record.id;
        
        processed_count := processed_count + 1;
    END LOOP;
    
    RETURN processed_count;
END;
$$ LANGUAGE plpgsql;

-- Function to update user metrics from submission events
CREATE OR REPLACE FUNCTION update_user_metrics_from_submission(event_record RECORD)
RETURNS VOID AS $$
DECLARE
    user_metrics_record RECORD;
    solving_speed NUMERIC;
    debug_efficiency NUMERIC;
BEGIN
    -- Calculate current metrics for this submission
    solving_speed := calculate_problem_solving_speed(
        event_record.user_id, 
        (event_record.event_data->>'problem_id')::UUID
    );
    
    debug_efficiency := calculate_debugging_efficiency(
        event_record.user_id,
        (event_record.event_data->>'problem_id')::UUID
    );
    
    -- Insert new metrics record
    INSERT INTO user_performance_metrics (
        user_id,
        problem_solving_speed,
        debugging_efficiency,
        total_submissions,
        accepted_submissions
    ) VALUES (
        event_record.user_id,
        solving_speed,
        debug_efficiency,
        (SELECT COUNT(*) FROM submissions WHERE user_id = event_record.user_id),
        (SELECT COUNT(*) FROM submissions WHERE user_id = event_record.user_id AND status = 'AC')
    );
    
END;
$$ LANGUAGE plpgsql;

-- Function to update user metrics from contest events  
CREATE OR REPLACE FUNCTION update_user_metrics_from_contest(event_record RECORD)
RETURNS VOID AS $$
BEGIN
    -- Update contest-related metrics
    -- This will be implemented when contest events are processed
    NULL;
END;
$$ LANGUAGE plpgsql;

-- Create RLS policies for performance tables
ALTER TABLE user_performance_metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE performance_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_skill_progression ENABLE ROW LEVEL SECURITY;
ALTER TABLE performance_analytics_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE performance_time_series ENABLE ROW LEVEL SECURITY;

-- Performance metrics policies
CREATE POLICY "Users can view their own performance metrics" ON user_performance_metrics
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can insert performance metrics" ON user_performance_metrics
    FOR INSERT WITH CHECK (true);

-- Performance events policies
CREATE POLICY "Users can view their own performance events" ON performance_events
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage performance events" ON performance_events
    FOR ALL WITH CHECK (true);

-- Skill progression policies
CREATE POLICY "Users can view their own skill progression" ON user_skill_progression
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage skill progression" ON user_skill_progression
    FOR ALL WITH CHECK (true);

-- Analytics cache policies
CREATE POLICY "Users can view their own analytics cache" ON performance_analytics_cache
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage analytics cache" ON performance_analytics_cache
    FOR ALL WITH CHECK (true);

-- Time series policies
CREATE POLICY "Users can view their own time series data" ON performance_time_series
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage time series data" ON performance_time_series
    FOR ALL WITH CHECK (true);