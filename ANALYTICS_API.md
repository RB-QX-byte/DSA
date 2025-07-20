# Analytics API Documentation

## Overview

The Analytics API provides comprehensive performance data and insights for users on the competitive programming platform. All endpoints are designed to return data structures optimized for direct consumption by frontend charting libraries like Chart.js and D3.js.

## Base URL

```
/api/v1/analytics
```

## Authentication

All analytics endpoints require user authentication. Include the authorization token in the request headers:

```
Authorization: Bearer <token>
```

## Rate Limiting

- 100 requests per minute per user
- Cached responses are served faster and don't count toward rate limits

## Data Caching

- User summary: 1 hour
- Skill radar: 15 minutes  
- Performance trends: 1 hour
- Recommendations: 24 hours

## Endpoints

### 1. User Performance Summary

Get a high-level performance overview for dashboard cards.

**Endpoint:** `GET /analytics/users/{userID}/summary`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "summary": {
      "overall_rating": 75.5,
      "performance_level": "Intermediate",
      "rank": 1250,
      "total_users": 10000,
      "recent_trend": "improving",
      "trend_percentage": 8.5,
      "last_active": "2025-01-15T10:30:00Z"
    },
    "badges": [
      {
        "id": "speed_demon",
        "name": "Speed Demon",
        "description": "Solved 10 problems in under 5 minutes",
        "icon": "⚡",
        "color": "#FFD700",
        "earned_at": "2025-01-10T15:20:00Z",
        "rarity": "rare"
      }
    ],
    "stats": [
      {
        "title": "Problems Solved",
        "value": 145,
        "unit": "problems",
        "change": 12.5,
        "change_type": "positive",
        "icon": "✓",
        "description": "Total problems successfully solved"
      }
    ]
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 2. Skill Radar Chart Data

Get skill estimates optimized for radar/spider chart visualization.

**Endpoint:** `GET /analytics/users/{userID}/skills`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "data": [
      {
        "skill": "Problem Solving Speed",
        "score": 75.5,
        "confidence": 85.2,
        "category": "problem_solving",
        "description": "Your Problem Solving Speed skill level based on recent performance"
      },
      {
        "skill": "Debugging Efficiency", 
        "score": 68.3,
        "confidence": 78.9,
        "category": "problem_solving",
        "description": "Your Debugging Efficiency skill level based on recent performance"
      }
    ],
    "meta": {
      "max_score": 100.0,
      "last_updated": "2025-01-15T11:45:00Z",
      "total_skills": 15,
      "average_score": 71.8,
      "strongest_skill": "Algorithm Selection",
      "weakest_skill": "Time Pressure Performance"
    }
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 3. Performance Trends

Get time-series data for line/area charts showing performance over time.

**Endpoint:** `GET /analytics/users/{userID}/trends`

**Query Parameters:**
- `period`: "daily", "weekly", "monthly" (default: "weekly")
- `limit`: Number of data points (default: 52)

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "period": "weekly",
    "datasets": [
      {
        "label": "Problem Solving Speed",
        "metric_key": "problem_solving_speed",
        "data": [
          ["2025-01-01T00:00:00Z", 15.5],
          ["2025-01-08T00:00:00Z", 14.2],
          ["2025-01-15T00:00:00Z", 13.8]
        ],
        "color": "#FF6384",
        "unit": "minutes",
        "description": "Average time to solve problems"
      }
    ],
    "meta": {
      "date_range": {
        "start": "2024-01-01T00:00:00Z",
        "end": "2025-01-15T00:00:00Z"
      },
      "total_points": 52,
      "trend_summary": {
        "problem_solving_speed": {
          "direction": "up",
          "change": -10.9,
          "period": "last_weekly"
        }
      }
    }
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 4. Performance Metrics

Get detailed performance metrics with historical data.

**Endpoint:** `GET /analytics/users/{userID}/performance`

**Query Parameters:**
- `limit`: Number of records (default: 50)

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "metrics": [
      {
        "id": "uuid",
        "recorded_at": "2025-01-15T10:00:00Z",
        "problem_solving_speed": 12.5,
        "debugging_efficiency": 0.75,
        "total_submissions": 156,
        "accepted_submissions": 98
      }
    ],
    "count": 50
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 5. Performance Predictions

Get AI-powered predictions for different scenarios.

**Endpoint:** `GET /analytics/users/{userID}/predictions`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "predictions": {
      "easy_problem": {
        "prediction": 85.5,
        "uncertainty": 12.3
      },
      "contest": {
        "prediction": 45.2,
        "uncertainty": 25.8
      }
    },
    "generated_at": "2025-01-15T12:00:00Z"
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 6. Peer Comparison

Get comparison data with peer groups.

**Endpoint:** `GET /analytics/users/{userID}/comparison`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "categories": [
      {
        "category": "Problem Solving Speed",
        "user_score": 75.5,
        "peer_average": 68.2,
        "percentile": 72.0,
        "rank": 280,
        "total_peers": 1000,
        "status": "above_average"
      }
    ],
    "meta": {
      "peer_group_size": 1000,
      "comparison_basis": "rating_band",
      "last_updated": "2025-01-15T11:00:00Z"
    }
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 7. Personalized Recommendations

Get AI-generated recommendations for improvement.

**Endpoint:** `GET /analytics/users/{userID}/recommendations`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "skill_focus": [
      {
        "skill": "Time Pressure Performance",
        "current_score": 45.2,
        "target_score": 65.0,
        "priority": "high",
        "reason": "Low performance in timed contests",
        "estimated_time": "2-3 weeks"
      }
    ],
    "problem_types": [
      {
        "type": "dynamic-programming",
        "difficulty": 1400,
        "count": 10,
        "priority": "high",
        "tags": ["dp", "optimization"],
        "description": "Focus on medium difficulty DP problems"
      }
    ],
    "learning_path": [
      {
        "step_number": 1,
        "title": "Master Basic DP Patterns",
        "description": "Learn fundamental dynamic programming patterns",
        "skills": ["algorithm_selection_accuracy"],
        "resources": [
          {
            "type": "article",
            "title": "DP Patterns Guide",
            "url": "/resources/dp-guide",
            "description": "Comprehensive guide to DP patterns",
            "duration": "30 minutes"
          }
        ],
        "estimated_time": "1 week",
        "status": "pending",
        "progress": 0
      }
    ],
    "difficulty_range": {
      "min": 1200,
      "max": 1600,
      "current": 1350,
      "recommended": 1400,
      "description": "Gradually increase difficulty"
    },
    "generated_at": "2025-01-15T10:00:00Z"
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### 8. System Health

Get analytics system health status (admin only).

**Endpoint:** `GET /analytics/health`

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "components": [
      {
        "name": "Event Processor",
        "status": "healthy",
        "description": "Processing events normally",
        "last_check": "2025-01-15T12:00:00Z"
      }
    ],
    "event_processing_lag": "2 minutes",
    "unprocessed_events": 150,
    "cache_hit_rate": 85.5,
    "last_processing_time": "2025-01-15T11:58:00Z",
    "uptime": "5 days 12 hours"
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

## Chart.js Integration

### Radar Chart Example

```javascript
// Use the skills endpoint data directly
const response = await fetch('/api/v1/analytics/users/me/skills');
const { data } = await response.json();

const chartData = {
  labels: data.data.map(point => point.skill),
  datasets: [{
    label: 'Your Skills',
    data: data.data.map(point => point.score),
    backgroundColor: 'rgba(255, 99, 132, 0.2)',
    borderColor: 'rgba(255, 99, 132, 1)',
    pointBackgroundColor: data.data.map(point => 
      point.category === 'problem_solving' ? '#FF6384' : 
      point.category === 'contest' ? '#36A2EB' : '#FFCE56'
    )
  }]
};
```

### Line Chart Example

```javascript
// Use the trends endpoint data directly
const response = await fetch('/api/v1/analytics/users/me/trends?period=weekly');
const { data } = await response.json();

const chartData = {
  datasets: data.datasets.map(dataset => ({
    label: dataset.label,
    data: dataset.data.map(point => ({
      x: point[0], // timestamp
      y: point[1]  // value
    })),
    borderColor: dataset.color,
    backgroundColor: dataset.color + '20'
  }))
};
```

## D3.js Integration

### Radar Chart Example

```javascript
// Transform skills data for D3 radar chart
const response = await fetch('/api/v1/analytics/users/me/skills');
const { data } = await response.json();

const radarData = data.data.map(point => ({
  axis: point.skill,
  value: point.score / 100, // D3 expects 0-1 scale
  category: point.category
}));
```

## Error Handling

### Error Response Format

```json
{
  "success": false,
  "error": {
    "code": "INVALID_USER_ID",
    "message": "The provided user ID is not valid",
    "details": "User ID must be a valid UUID"
  },
  "timestamp": "2025-01-15T12:00:00Z"
}
```

### Common Error Codes

- `INVALID_USER_ID`: Invalid user ID format
- `FORBIDDEN`: User cannot access this data
- `USER_NOT_FOUND`: User does not exist
- `INSUFFICIENT_DATA`: Not enough data for analysis
- `PROCESSING_ERROR`: Error during data processing
- `CACHE_ERROR`: Error accessing cached data
- `RATE_LIMITED`: Too many requests

## Data Freshness

- **Real-time**: Health endpoints
- **Near real-time (< 5 min)**: Performance metrics, trends
- **Periodic (15-60 min)**: Skills, summaries, predictions
- **Daily**: Recommendations, peer comparisons

## Best Practices

### Client-Side Optimization

1. **Cache responses** locally for repeated requests
2. **Use conditional requests** with ETags when available
3. **Implement progressive loading** for dashboard components
4. **Handle loading states** gracefully during API calls
5. **Implement error boundaries** for failed requests

### Performance Tips

1. Use appropriate `limit` parameters to avoid large responses
2. Leverage cached endpoints when data freshness isn't critical
3. Batch multiple requests when possible
4. Use WebSocket connections for real-time updates (when available)

### Visualization Guidelines

1. **Color consistency**: Use provided colors for consistent UX
2. **Responsive design**: Ensure charts work on all screen sizes
3. **Accessibility**: Include proper ARIA labels and descriptions
4. **Loading states**: Show skeletons while data loads
5. **Error states**: Gracefully handle missing or invalid data

## Changelog

### Version 1.0.0 (2025-01-15)
- Initial release
- All endpoints implemented
- Chart.js and D3.js optimized formats
- Comprehensive error handling
- Caching system implemented