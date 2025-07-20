-- Additional tables for the recommendation system (Task 8)
-- This extends the main schema.sql with recommendation-specific tables

-- User interactions table for tracking all user-problem interactions
CREATE TABLE IF NOT EXISTS user_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    interaction_type VARCHAR(50) NOT NULL, -- 'view', 'attempt', 'solve', 'hint_used'
    duration INTEGER NOT NULL DEFAULT 0, -- seconds spent
    success BOOLEAN DEFAULT FALSE, -- whether problem was solved
    attempt_count INTEGER DEFAULT 1,
    language_used VARCHAR(50),
    solution_quality DECIMAL(3,2) DEFAULT 0.0, -- 0.0 to 1.0
    difficulty_rating DECIMAL(4,1) DEFAULT 0.0, -- user's perceived difficulty
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes for efficient querying
    INDEX idx_user_interactions_user_id (user_id),
    INDEX idx_user_interactions_problem_id (problem_id),
    INDEX idx_user_interactions_timestamp (timestamp),
    INDEX idx_user_interactions_type (interaction_type),
    INDEX idx_user_interactions_success (success)
);

-- User profiles table for storing extracted user features and preferences
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    skill_vector JSONB NOT NULL DEFAULT '{}', -- skill category -> proficiency
    preferred_difficulty INTEGER[] DEFAULT ARRAY[800, 1200], -- [min, max] difficulty range
    preferred_tags TEXT[] DEFAULT ARRAY[]::TEXT[],
    preferred_languages TEXT[] DEFAULT ARRAY[]::TEXT[],
    solved_problems UUID[] DEFAULT ARRAY[]::UUID[],
    attempted_problems UUID[] DEFAULT ARRAY[]::UUID[],
    weak_areas TEXT[] DEFAULT ARRAY[]::TEXT[],
    learning_goals TEXT[] DEFAULT ARRAY[]::TEXT[],
    activity_pattern JSONB DEFAULT '{}', -- time of day -> activity score
    last_active TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes
    INDEX idx_user_profiles_updated_at (updated_at),
    INDEX idx_user_profiles_last_active (last_active),
    INDEX idx_user_profiles_skill_vector (skill_vector) USING GIN,
    INDEX idx_user_profiles_preferred_tags (preferred_tags) USING GIN
);

-- Problem features table for storing extracted problem characteristics
CREATE TABLE IF NOT EXISTS problem_features (
    problem_id UUID PRIMARY KEY REFERENCES problems(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    difficulty INTEGER NOT NULL,
    tags TEXT[] DEFAULT ARRAY[]::TEXT[],
    acceptance_rate DECIMAL(5,2) DEFAULT 0.0,
    average_attempts DECIMAL(6,2) DEFAULT 0.0,
    average_solve_time DECIMAL(8,2) DEFAULT 0.0, -- in minutes
    topic_vector JSONB DEFAULT '{}', -- topic -> weight
    complexity_score DECIMAL(4,3) DEFAULT 0.0,
    popularity_score DECIMAL(4,3) DEFAULT 0.0,
    similar_problems UUID[] DEFAULT ARRAY[]::UUID[],
    prerequisites UUID[] DEFAULT ARRAY[]::UUID[],
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes
    INDEX idx_problem_features_difficulty (difficulty),
    INDEX idx_problem_features_tags (tags) USING GIN,
    INDEX idx_problem_features_updated_at (updated_at),
    INDEX idx_problem_features_complexity (complexity_score),
    INDEX idx_problem_features_popularity (popularity_score)
);

-- User similarities table for collaborative filtering
CREATE TABLE IF NOT EXISTS user_similarities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id_1 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_id_2 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    similarity_type VARCHAR(50) NOT NULL, -- 'cosine', 'pearson', 'jaccard'
    score DECIMAL(6,5) NOT NULL, -- 0.0 to 1.0
    shared_problems INTEGER DEFAULT 0,
    computed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(user_id_1, user_id_2, similarity_type),
    CHECK (user_id_1 != user_id_2),
    
    -- Indexes
    INDEX idx_user_similarities_user1 (user_id_1),
    INDEX idx_user_similarities_user2 (user_id_2),
    INDEX idx_user_similarities_type (similarity_type),
    INDEX idx_user_similarities_score (score),
    INDEX idx_user_similarities_computed_at (computed_at)
);

-- Problem similarities table for content-based filtering
CREATE TABLE IF NOT EXISTS problem_similarities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    problem_id_1 UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    problem_id_2 UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    similarity_type VARCHAR(50) NOT NULL, -- 'content', 'collaborative', 'difficulty'
    score DECIMAL(6,5) NOT NULL, -- 0.0 to 1.0
    common_tags TEXT[] DEFAULT ARRAY[]::TEXT[],
    computed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(problem_id_1, problem_id_2, similarity_type),
    CHECK (problem_id_1 != problem_id_2),
    
    -- Indexes
    INDEX idx_problem_similarities_problem1 (problem_id_1),
    INDEX idx_problem_similarities_problem2 (problem_id_2),
    INDEX idx_problem_similarities_type (similarity_type),
    INDEX idx_problem_similarities_score (score),
    INDEX idx_problem_similarities_computed_at (computed_at)
);

-- Recommendation cache table for storing pre-computed recommendations
CREATE TABLE IF NOT EXISTS recommendation_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cache_key VARCHAR(255) NOT NULL,
    recommendations JSONB NOT NULL, -- array of recommendation results
    model_version VARCHAR(50) NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    hit_count INTEGER DEFAULT 0,
    
    UNIQUE(user_id, cache_key),
    
    -- Indexes
    INDEX idx_recommendation_cache_user_id (user_id),
    INDEX idx_recommendation_cache_key (cache_key),
    INDEX idx_recommendation_cache_expires_at (expires_at),
    INDEX idx_recommendation_cache_generated_at (generated_at)
);

-- Model metadata table for tracking recommendation models
CREATE TABLE IF NOT EXISTS recommendation_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_type VARCHAR(100) NOT NULL, -- 'content_based', 'collaborative', 'hybrid'
    version VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'training', -- 'training', 'ready', 'updating', 'failed'
    model_data JSONB NOT NULL DEFAULT '{}', -- serialized model parameters
    training_metrics JSONB DEFAULT '{}', -- training performance metrics
    validation_metrics JSONB DEFAULT '{}', -- validation performance metrics
    trained_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(model_type, version),
    
    -- Indexes
    INDEX idx_recommendation_models_type (model_type),
    INDEX idx_recommendation_models_status (status),
    INDEX idx_recommendation_models_trained_at (trained_at),
    INDEX idx_recommendation_models_version (version)
);

-- Training data snapshots for reproducibility
CREATE TABLE IF NOT EXISTS training_data_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_name VARCHAR(255) NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    validation_split DECIMAL(3,2) DEFAULT 0.2,
    test_split DECIMAL(3,2) DEFAULT 0.1,
    interaction_count INTEGER DEFAULT 0,
    user_count INTEGER DEFAULT 0,
    problem_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(snapshot_name),
    
    -- Indexes
    INDEX idx_training_snapshots_name (snapshot_name),
    INDEX idx_training_snapshots_created_at (created_at)
);

-- Model performance metrics for A/B testing and monitoring
CREATE TABLE IF NOT EXISTS model_performance_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_id UUID NOT NULL REFERENCES recommendation_models(id) ON DELETE CASCADE,
    model_type VARCHAR(100) NOT NULL,
    metric_name VARCHAR(100) NOT NULL, -- 'precision', 'recall', 'f1_score', 'map', 'ndcg', etc.
    metric_value DECIMAL(10,6) NOT NULL,
    evaluation_set VARCHAR(50) NOT NULL, -- 'train', 'validation', 'test', 'production'
    evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    evaluation_context JSONB DEFAULT '{}', -- additional context about the evaluation
    
    -- Indexes
    INDEX idx_model_metrics_model_id (model_id),
    INDEX idx_model_metrics_type (model_type),
    INDEX idx_model_metrics_name (metric_name),
    INDEX idx_model_metrics_set (evaluation_set),
    INDEX idx_model_metrics_evaluated_at (evaluated_at)
);

-- User feedback on recommendations for improving models
CREATE TABLE IF NOT EXISTS recommendation_feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    recommendation_score DECIMAL(4,3) NOT NULL, -- original recommendation score
    feedback_type VARCHAR(50) NOT NULL, -- 'clicked', 'solved', 'dismissed', 'rated'
    feedback_value DECIMAL(3,2), -- numeric feedback (1-5 rating, etc.)
    feedback_text TEXT, -- optional text feedback
    model_version VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes
    INDEX idx_recommendation_feedback_user_id (user_id),
    INDEX idx_recommendation_feedback_problem_id (problem_id),
    INDEX idx_recommendation_feedback_type (feedback_type),
    INDEX idx_recommendation_feedback_created_at (created_at),
    INDEX idx_recommendation_feedback_model_version (model_version)
);

-- Create trigger to update user profiles when interactions happen
CREATE OR REPLACE FUNCTION trigger_user_interaction_processing()
RETURNS TRIGGER AS $$
BEGIN
    -- Mark user profile for update
    UPDATE user_profiles 
    SET updated_at = NOW() 
    WHERE user_id = NEW.user_id;
    
    -- If profile doesn't exist, create a basic one
    INSERT INTO user_profiles (user_id, updated_at)
    VALUES (NEW.user_id, NOW())
    ON CONFLICT (user_id) DO NOTHING;
    
    -- Mark problem features for update
    UPDATE problem_features 
    SET updated_at = NOW() 
    WHERE problem_id = NEW.problem_id;
    
    -- If problem features don't exist, create basic entry
    INSERT INTO problem_features (problem_id, title, difficulty, tags, updated_at)
    SELECT NEW.problem_id, p.title, p.difficulty, p.tags, NOW()
    FROM problems p 
    WHERE p.id = NEW.problem_id
    ON CONFLICT (problem_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for user interactions
CREATE TRIGGER user_interaction_processing_trigger
    AFTER INSERT ON user_interactions
    FOR EACH ROW
    EXECUTE FUNCTION trigger_user_interaction_processing();

-- Create function to populate user interactions from existing submissions
CREATE OR REPLACE FUNCTION populate_user_interactions_from_submissions()
RETURNS INTEGER AS $$
DECLARE
    inserted_count INTEGER := 0;
    submission_record RECORD;
BEGIN
    FOR submission_record IN 
        SELECT user_id, problem_id, created_at, status, language, execution_time
        FROM submissions 
        WHERE created_at >= NOW() - INTERVAL '30 days'
        ORDER BY created_at
    LOOP
        INSERT INTO user_interactions (
            user_id, 
            problem_id, 
            interaction_type, 
            duration, 
            success, 
            attempt_count, 
            language_used, 
            solution_quality, 
            timestamp
        ) VALUES (
            submission_record.user_id,
            submission_record.problem_id,
            'attempt',
            COALESCE(submission_record.execution_time, 0) / 1000, -- Convert ms to seconds
            submission_record.status = 'AC',
            1,
            submission_record.language,
            CASE WHEN submission_record.status = 'AC' THEN 1.0 ELSE 0.0 END,
            submission_record.created_at
        )
        ON CONFLICT DO NOTHING;
        
        inserted_count := inserted_count + 1;
    END LOOP;
    
    RETURN inserted_count;
END;
$$ LANGUAGE plpgsql;

-- RLS policies for recommendation tables
ALTER TABLE user_interactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE problem_features ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_similarities ENABLE ROW LEVEL SECURITY;
ALTER TABLE problem_similarities ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendation_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendation_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE training_data_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE model_performance_metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendation_feedback ENABLE ROW LEVEL SECURITY;

-- User interactions policies
CREATE POLICY "Users can view their own interactions" ON user_interactions
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can create their own interactions" ON user_interactions
    FOR INSERT WITH CHECK (auth.uid() = user_id);

CREATE POLICY "System can manage all interactions" ON user_interactions
    FOR ALL USING (current_setting('role') = 'service_role');

-- User profiles policies
CREATE POLICY "Users can view their own profile" ON user_profiles
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage user profiles" ON user_profiles
    FOR ALL USING (current_setting('role') = 'service_role');

-- Problem features policies (public read)
CREATE POLICY "Anyone can view problem features" ON problem_features
    FOR SELECT USING (true);

CREATE POLICY "System can manage problem features" ON problem_features
    FOR ALL USING (current_setting('role') = 'service_role');

-- User similarities policies
CREATE POLICY "Users can view similarities involving them" ON user_similarities
    FOR SELECT USING (auth.uid() = user_id_1 OR auth.uid() = user_id_2);

CREATE POLICY "System can manage user similarities" ON user_similarities
    FOR ALL USING (current_setting('role') = 'service_role');

-- Problem similarities policies (public read)
CREATE POLICY "Anyone can view problem similarities" ON problem_similarities
    FOR SELECT USING (true);

CREATE POLICY "System can manage problem similarities" ON problem_similarities
    FOR ALL USING (current_setting('role') = 'service_role');

-- Recommendation cache policies
CREATE POLICY "Users can view their own cached recommendations" ON recommendation_cache
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "System can manage recommendation cache" ON recommendation_cache
    FOR ALL USING (current_setting('role') = 'service_role');

-- Model metadata policies (public read for basic info)
CREATE POLICY "Anyone can view model info" ON recommendation_models
    FOR SELECT USING (true);

CREATE POLICY "System can manage models" ON recommendation_models
    FOR ALL USING (current_setting('role') = 'service_role');

-- Training data snapshots policies (public read)
CREATE POLICY "Anyone can view training snapshots" ON training_data_snapshots
    FOR SELECT USING (true);

CREATE POLICY "System can manage training snapshots" ON training_data_snapshots
    FOR ALL USING (current_setting('role') = 'service_role');

-- Model performance metrics policies (public read)
CREATE POLICY "Anyone can view model metrics" ON model_performance_metrics
    FOR SELECT USING (true);

CREATE POLICY "System can manage model metrics" ON model_performance_metrics
    FOR ALL USING (current_setting('role') = 'service_role');

-- Recommendation feedback policies
CREATE POLICY "Users can view their own feedback" ON recommendation_feedback
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can create feedback" ON recommendation_feedback
    FOR INSERT WITH CHECK (auth.uid() = user_id);

CREATE POLICY "System can manage feedback" ON recommendation_feedback
    FOR ALL USING (current_setting('role') = 'service_role');

-- Create maintenance functions
CREATE OR REPLACE FUNCTION cleanup_expired_recommendation_cache()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM recommendation_cache 
    WHERE expires_at < NOW();
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create function to get recommendation statistics
CREATE OR REPLACE FUNCTION get_recommendation_system_stats()
RETURNS TABLE (
    total_interactions BIGINT,
    active_users BIGINT,
    problems_with_features BIGINT,
    cached_recommendations BIGINT,
    trained_models BIGINT,
    avg_user_interactions NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        (SELECT COUNT(*) FROM user_interactions) as total_interactions,
        (SELECT COUNT(DISTINCT user_id) FROM user_interactions WHERE timestamp >= NOW() - INTERVAL '30 days') as active_users,
        (SELECT COUNT(*) FROM problem_features) as problems_with_features,
        (SELECT COUNT(*) FROM recommendation_cache WHERE expires_at > NOW()) as cached_recommendations,
        (SELECT COUNT(*) FROM recommendation_models WHERE status = 'ready') as trained_models,
        (SELECT AVG(interaction_count) FROM (
            SELECT COUNT(*) as interaction_count 
            FROM user_interactions 
            WHERE timestamp >= NOW() - INTERVAL '30 days'
            GROUP BY user_id
        ) user_counts) as avg_user_interactions;
END;
$$ LANGUAGE plpgsql;