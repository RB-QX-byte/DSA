# Recommendation System - Implementation Guide

This document describes the implementation of the AI-powered problem recommendation engine for the competitive programming platform (Task 8).

## Overview

The recommendation system implements a hybrid approach combining:
- **Content-Based Filtering**: Using deep learning embeddings to match users with problems based on skill profiles and problem features
- **Collaborative Filtering**: Using matrix factorization to identify user preferences based on similar users' behavior
- **Hybrid Model**: Intelligently combining both approaches with optimized weights

## Architecture

### Core Components

1. **Data Pipeline (`pipeline.go`)**: Processes user interactions and extracts features
2. **Feature Engineering (`feature_engineering.go`)**: Extracts user profiles and problem features
3. **Content-Based Filter (`content_based.go`)**: Neural embedding model for content similarity
4. **Collaborative Filter (`collaborative_filtering.go`)**: Matrix factorization for collaborative recommendations
5. **Hybrid Engine (`hybrid_model.go`)**: Combines both approaches with optimized weights
6. **Service Layer (`service.go`)**: Main business logic and orchestration
7. **API Handlers (`handlers.go`)**: HTTP endpoints for the recommendation API

### Database Schema

The system extends the main database with the following tables:
- `user_interactions`: Tracks all user-problem interactions
- `user_profiles`: Stores extracted user skill profiles and preferences
- `problem_features`: Stores extracted problem characteristics
- `user_similarities`: Collaborative filtering similarity data
- `problem_similarities`: Content-based similarity data
- `recommendation_cache`: Caches recommendations for performance
- `recommendation_models`: Stores model metadata and performance metrics
- `recommendation_feedback`: Collects user feedback for model improvement

## Setup and Installation

### 1. Apply Database Schema

Run the migration to add recommendation system tables:

```bash
go run cmd/migrate-recommendation/main.go
```

### 2. Environment Variables

The system uses the following environment variables for ML models:
- `ANTHROPIC_API_KEY`: For Claude models (optional)
- `OPENAI_API_KEY`: For GPT models (optional)
- `PERPLEXITY_API_KEY`: For research features (optional)

At least one API key should be configured for model training.

### 3. Service Integration

The recommendation service is automatically integrated into the main application. It initializes on startup and begins processing data.

## API Endpoints

### Get Recommendations

**GET** `/api/v1/recommendations`

Query parameters:
- `user_id` (required): User UUID
- `count` (optional): Number of recommendations (default: 10, max: 100)
- `min_difficulty` (optional): Minimum problem difficulty
- `max_difficulty` (optional): Maximum problem difficulty
- `required_tags` (optional): Comma-separated list of required tags
- `exclude_tags` (optional): Comma-separated list of tags to exclude
- `focus_areas` (optional): Comma-separated list of skills to focus on
- `time_limit` (optional): Maximum estimated solve time in minutes
- `include_solved` (optional): Include already solved problems (default: false)
- `recommendation_type` (optional): Type of recommendation (skill_building, challenge, practice, contest_prep)

Example:
```bash
curl "http://localhost:8080/api/v1/recommendations?user_id=123e4567-e89b-12d3-a456-426614174000&count=5&recommendation_type=skill_building"
```

**POST** `/api/v1/recommendations`

JSON body:
```json
{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "count": 10,
  "min_difficulty": 800,
  "max_difficulty": 1600,
  "required_tags": ["dynamic-programming"],
  "focus_areas": ["algorithms"],
  "recommendation_type": "skill_building"
}
```

### User Profile

**GET** `/api/v1/users/{userId}/profile`

Returns the extracted user profile including skills, preferences, and statistics.

### Problem Features

**GET** `/api/v1/problems/{problemId}/features`

Returns the extracted features for a specific problem.

### Record Feedback

**POST** `/api/v1/users/{userId}/feedback`

Record user feedback on recommendations:
```json
{
  "problem_id": "123e4567-e89b-12d3-a456-426614174000",
  "feedback_type": "clicked",
  "feedback_value": 4.5,
  "feedback_text": "Great recommendation!"
}
```

Feedback types: `clicked`, `solved`, `dismissed`, `rated`

### Service Management

**GET** `/api/v1/recommendations/status`

Returns service status and metrics.

**GET** `/api/v1/recommendations/metrics`

Returns model performance metrics.

**POST** `/api/v1/recommendations/retrain`

Triggers model retraining (admin only).

## Model Training

### Automatic Training

The system automatically trains models on startup if no existing models are found. Training uses the last 90 days of user interaction data.

### Manual Retraining

Models can be retrained manually via the API or by restarting the service. Retraining should be done periodically (e.g., weekly) to incorporate new data.

### Training Data

The system uses:
- **User Interactions**: From submissions, problem views, and explicit feedback
- **User Profiles**: Skill assessments from the analytics system
- **Problem Features**: Difficulty, tags, acceptance rates, and computed features

## Performance and Scalability

### Caching

- Recommendations are cached for 1 hour per user
- Model predictions are cached during batch processing
- Database features are cached to reduce query load

### Real-time Processing

- New user interactions trigger profile updates
- Data pipeline processes changes every hour
- Models adapt to new data without full retraining

### Scalability Features

- Concurrent model training (content-based and collaborative run in parallel)
- Batch processing for efficient feature extraction
- Horizontal scaling support for multiple workers

## Monitoring and Metrics

### Key Metrics

The system tracks:
- **Precision@K**: Accuracy of top-K recommendations
- **Recall@K**: Coverage of relevant items in top-K
- **NDCG**: Normalized discounted cumulative gain
- **Coverage**: Percentage of problems recommended
- **Diversity**: Variety in recommendations
- **Click-through Rate**: User engagement with recommendations

### Model Performance

Monitor via:
- `/api/v1/recommendations/metrics` endpoint
- Database `model_performance_metrics` table
- Service status endpoint for operational metrics

## Troubleshooting

### Common Issues

1. **Service not initializing**: Check API keys and database connection
2. **No recommendations**: Ensure user has sufficient interaction history
3. **Poor recommendations**: Check model training status and data quality
4. **Slow responses**: Monitor cache hit rates and database performance

### Debugging

Enable debug logging:
```bash
export LOG_LEVEL=debug
```

Check service status:
```bash
curl http://localhost:8080/api/v1/recommendations/status
```

### Model Issues

If models fail to train:
1. Check available training data (need minimum 100 interactions)
2. Verify API keys for ML services
3. Check database schema is properly applied
4. Review logs for specific error messages

## Configuration

### Model Parameters

Key parameters in `models.go`:
- `EmbeddingDim`: Dimensionality of embeddings (default: 128)
- `FactorDim`: Matrix factorization factors (default: 50)
- `LearningRate`: Training learning rate (default: 0.01)
- `RegularizationL2`: L2 regularization (default: 0.01)
- `BatchSize`: Training batch size (default: 32/256)
- `Epochs`: Training epochs (default: 100)

### Pipeline Configuration

Key settings in `PipelineConfig`:
- `ProcessingInterval`: How often to process new data (default: 1 hour)
- `FeatureWindow`: How far back to look for features (default: 90 days)
- `MinInteractionsPerUser`: Minimum interactions to include user (default: 5)
- `CacheExpiration`: How long to cache recommendations (default: 1 hour)

## Future Enhancements

### Planned Features

1. **Real-time Learning**: Online learning from immediate feedback
2. **Multi-objective Optimization**: Balance learning value, difficulty, and engagement
3. **Contextual Recommendations**: Consider time of day, contest preparation, etc.
4. **Social Features**: Friend-based recommendations and collaborative learning
5. **Advanced Analytics**: Detailed learning path analysis and progress tracking

### A/B Testing

The system supports A/B testing of different recommendation strategies:
1. Implement multiple recommendation strategies
2. Use `recommendation_type` parameter to select strategy
3. Track performance metrics by strategy
4. Use feedback data to evaluate effectiveness

## Security and Privacy

### Data Protection

- All user data is subject to RLS (Row Level Security) policies
- Recommendation cache respects user permissions
- Model training uses aggregated, anonymized data where possible
- User feedback is encrypted and access-controlled

### API Security

- All endpoints require authentication
- Rate limiting prevents abuse
- Input validation prevents injection attacks
- CORS configured for allowed origins only

This recommendation system provides a robust, scalable foundation for personalized problem recommendations that will improve user engagement and learning outcomes on the competitive programming platform.