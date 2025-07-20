# Go Backend Setup

This guide covers setting up the GoLang modular monolith backend for the competitive programming platform.

## Prerequisites

- Go 1.21 or higher
- PostgreSQL database (Supabase recommended)
- Git

## Installation

### 1. Install Go

#### Ubuntu/Debian:
```bash
sudo apt update
sudo apt install golang-go
```

#### macOS:
```bash
brew install go
```

#### Windows:
Download and install from https://golang.org/dl/

### 2. Verify Installation
```bash
go version
```

### 3. Set up Go Environment Variables
Add to your shell profile (`.bashrc`, `.zshrc`, etc.):
```bash
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

## Project Structure

```
competitive-programming-platform/
├── cmd/
│   └── server/           # Server command (future)
├── internal/
│   ├── auth/            # Authentication service
│   ├── problem/         # Problem management service
│   └── user/            # User management service
├── pkg/
│   ├── database/        # Database connection and utilities
│   └── middleware/      # HTTP middleware
├── main.go              # Application entry point
├── go.mod              # Go modules file
├── go.sum              # Go modules checksum
└── .env                # Environment variables
```

## Configuration

### 1. Environment Variables

Copy the example environment file:
```bash
cp .env.example .env
```

Update the `.env` file with your actual values:
```env
# Database Configuration
DATABASE_URL=postgresql://postgres:password@db.your-project-id.supabase.co:5432/postgres

# Supabase Configuration
SUPABASE_URL=https://your-project-id.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key

# JWT Configuration
JWT_SECRET=your-secret-key-change-this-in-production

# Server Configuration
PORT=8080

# Environment
ENV=development
```

### 2. Database Setup

Ensure your PostgreSQL database is running with the schema from `schema.sql` applied.

## Running the Application

### 1. Install Dependencies
```bash
go mod tidy
```

### 2. Run the Server
```bash
go run main.go
```

Or build and run:
```bash
go build -o bin/app main.go
./bin/app
```

### 3. Verify Health Check
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## API Endpoints

### Public Endpoints

- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/register` - User registration
- `GET /api/v1/problems` - Get problems list
- `GET /api/v1/problems/{id}` - Get specific problem

### Protected Endpoints (Require Authentication)

#### User Management
- `GET /api/v1/users/me` - Get current user profile
- `PUT /api/v1/users/me` - Update current user profile
- `GET /api/v1/users/{id}` - Get user profile by ID

#### Problem Management
- `POST /api/v1/problems` - Create new problem
- `PUT /api/v1/problems/{id}` - Update problem
- `DELETE /api/v1/problems/{id}` - Delete problem
- `POST /api/v1/problems/{id}/submit` - Submit solution

#### Submissions
- `GET /api/v1/submissions` - Get user's submissions
- `GET /api/v1/submissions/{id}` - Get specific submission

## Authentication

### JWT Token Format

The API uses JWT tokens for authentication. Include the token in the Authorization header:

```bash
Authorization: Bearer <your-jwt-token>
```

### Login Example

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "username": "testuser",
    "full_name": "Test User",
    "rating": 1200
  }
}
```

### Registration Example

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "password123",
    "username": "newuser",
    "full_name": "New User"
  }'
```

## Development

### Code Structure

#### Services
Each service follows the same pattern:
- `internal/{service}/service.go` - Main service implementation
- HTTP handlers for REST API endpoints
- Database operations using the connection pool

#### Middleware
- Authentication middleware validates JWT tokens
- CORS middleware for frontend integration
- Logging and recovery middleware

#### Database
- Connection pooling with pgx
- Transaction support
- Health check functionality

### Adding New Features

1. **Create a new service:**
   ```bash
   mkdir internal/newservice
   touch internal/newservice/service.go
   ```

2. **Implement service interface:**
   ```go
   type Service struct {
       db *database.DB
   }
   
   func NewService(db *database.DB) *Service {
       return &Service{db: db}
   }
   ```

3. **Add routes in main.go:**
   ```go
   newService := newservice.NewService(db)
   r.Get("/api/v1/newendpoint", newService.Handler)
   ```

### Testing

```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/auth

# Run with coverage
go test -cover ./...
```

## Deployment

### Building for Production

```bash
# Build for current platform
go build -o bin/app main.go

# Build for Linux (if on different platform)
GOOS=linux GOARCH=amd64 go build -o bin/app main.go
```

### Docker Deployment

Create a `Dockerfile`:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
```

Build and run:
```bash
docker build -t competitive-programming-platform .
docker run -p 8080:8080 competitive-programming-platform
```

### Environment Variables for Production

```env
ENV=production
JWT_SECRET=your-very-secure-secret-key-here
DATABASE_URL=postgresql://postgres:secure-password@production-db:5432/postgres
PORT=8080
```

## Monitoring and Logging

### Health Check
The application exposes a health check endpoint at `/health`:
```bash
curl http://localhost:8080/health
```

### Logging
All requests are logged with:
- Request method and path
- Response status code
- Response time
- User agent

### Metrics (Future Implementation)
- Request count and duration
- Database connection pool stats
- Error rates
- Memory usage

## Security Considerations

1. **JWT Secret**: Use a strong, randomly generated secret in production
2. **Database Credentials**: Never commit credentials to version control
3. **CORS**: Configure allowed origins for production
4. **Rate Limiting**: Implement rate limiting for API endpoints
5. **Input Validation**: Validate all user inputs
6. **SQL Injection**: Use parameterized queries (already implemented)

## Troubleshooting

### Common Issues

1. **Database Connection Errors**
   - Check DATABASE_URL format
   - Verify database is running
   - Check network connectivity

2. **JWT Token Errors**
   - Ensure JWT_SECRET is set
   - Check token expiration
   - Verify token format

3. **CORS Issues**
   - Check AllowedOrigins in main.go
   - Verify frontend URL is allowed
   - Check request headers

4. **Go Module Issues**
   ```bash
   go mod tidy
   go mod download
   ```

### Debug Mode

Set environment variable for debug logging:
```bash
export DEBUG=true
go run main.go
```

## Next Steps

1. Run the authentication test script: `node test-auth.js`
2. Test API endpoints with curl or Postman
3. Set up the frontend (Astro.js)
4. Implement the judge system for code execution
5. Add real-time features with WebSocket or SSE

The GoLang backend is now ready for development and testing!