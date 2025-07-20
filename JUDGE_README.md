# Judge System Implementation

This document describes the implementation of Task 3: "Implement Basic Code Execution Judge System".

## Overview

The judge system is a secure, distributed code execution environment that:
- Accepts code submissions in multiple programming languages
- Executes code safely using sandboxing (Isolate or mock sandbox)
- Runs submissions against test cases
- Returns verdicts (AC, WA, TLE, MLE, RE, CE, etc.)
- Handles concurrent submissions using Redis/Asynq queues

## Architecture

### Components

1. **Judge Service** (`internal/judge/service.go`)
   - Main service handling submission logic
   - Manages the full judging pipeline

2. **Queue Manager** (`internal/judge/queue.go`)
   - Handles Asynq client/server for distributed processing
   - Manages Redis connection and queue operations

3. **Sandbox Manager** (`internal/judge/sandbox.go`)
   - Integrates with Isolate for secure code execution
   - Falls back to mock sandbox when Isolate is unavailable

4. **Test Case Manager** (`internal/judge/testcase.go`)
   - Manages test cases for problems
   - Handles test case iteration and verdict generation

5. **API Handler** (`internal/judge/api.go`)
   - Provides REST endpoints for judge operations

### Supported Languages

- C++ (g++)
- Java (javac/java)
- Python (python3)
- Go (go build)

## Setup

### Prerequisites

1. **Go 1.21+** - Install from https://golang.org/dl/
2. **Redis** - For queue management
3. **PostgreSQL** - For database (via Supabase)
4. **Isolate** (Optional) - For secure sandboxing

### Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
# Database
DATABASE_URL=postgresql://user:password@localhost:5432/competitive_programming

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# Server
PORT=8080
JWT_SECRET=your-secret-key
```

### Database Setup

Run the schema migration:

```bash
# Apply schema to your database
psql -d competitive_programming -f schema.sql
```

### Installing Isolate (Optional)

For secure sandboxing, install Isolate:

```bash
# On Ubuntu/Debian
sudo apt-get install isolate

# Or build from source
git clone https://github.com/ioi/isolate.git
cd isolate
make install
```

If Isolate is not available, the system automatically falls back to a mock sandbox for testing.

## Running the System

### 1. Start the Main Server

```bash
export PATH=$HOME/go/bin:$PATH
go run main.go
```

### 2. Start the Judge Worker

In a separate terminal:

```bash
export PATH=$HOME/go/bin:$PATH
go run cmd/judge-worker/main.go
```

### 3. Test the System

```bash
# Test basic functionality
go run test_integration.go

# Test queue operations
go run test_queue.go
```

## API Endpoints

### Submit Solution

```bash
POST /api/v1/problems/{id}/submit
Authorization: Bearer <token>
Content-Type: application/json

{
  "source_code": "#include <iostream>\nint main() { std::cout << \"Hello World\" << std::endl; return 0; }",
  "language": "cpp"
}
```

### Get Submission Status

```bash
GET /api/v1/submissions/{id}
Authorization: Bearer <token>
```

### Get Queue Statistics

```bash
GET /api/v1/judge/queue/stats
Authorization: Bearer <token>
```

## Verdict Types

- **AC** - Accepted
- **WA** - Wrong Answer
- **TLE** - Time Limit Exceeded
- **MLE** - Memory Limit Exceeded
- **RE** - Runtime Error
- **CE** - Compilation Error
- **PE** - Pending
- **IE** - Internal Error

## Security Features

### Sandbox Security

When using Isolate:
- Process isolation
- Resource limits (time, memory, processes)
- Filesystem restrictions
- Network isolation

### Mock Sandbox

For testing without Isolate:
- Temporary directory isolation
- Basic resource monitoring
- Simulated execution results

## Development

### Project Structure

```
internal/judge/
├── service.go      # Main judge service
├── queue.go        # Queue management
├── sandbox.go      # Isolate integration
├── mock_sandbox.go # Mock sandbox for testing
├── testcase.go     # Test case management
├── types.go        # Data structures
└── api.go          # API handlers

cmd/judge-worker/
└── main.go         # Worker executable
```

### Building

```bash
# Build main server
go build -o server main.go

# Build judge worker
go build -o judge-worker cmd/judge-worker/main.go
```

### Testing

```bash
# Run integration tests
go run test_integration.go

# Test queue functionality
go run test_queue.go
```

## Troubleshooting

### Common Issues

1. **Redis Connection Failed**
   - Ensure Redis is running on localhost:6379
   - Check REDIS_ADDR and REDIS_PASSWORD environment variables

2. **Database Connection Failed**
   - Verify DATABASE_URL is correct
   - Ensure database exists and schema is applied

3. **Isolate Not Found**
   - System automatically falls back to mock sandbox
   - Install Isolate for production use

4. **Compilation Errors**
   - Ensure required compilers are installed (gcc, javac, python3, go)
   - Check language configuration in `types.go`

### Logging

The system logs to stdout. Key log messages:
- `Judge worker started successfully` - Worker is ready
- `Submission {id} queued for judging` - Submission accepted
- `Submission {id} judged successfully: {verdict}` - Judging complete

## Performance

### Metrics

- **Concurrency**: 10 concurrent workers by default
- **Queue Priority**: critical > default > low
- **Timeout**: Configurable per problem (default 1000ms)
- **Memory Limit**: Configurable per problem (default 256MB)

### Monitoring

Queue statistics available at `/api/v1/judge/queue/stats`:
- Queue lengths per priority
- Active workers
- Processing metrics

## Future Enhancements

1. **Enhanced Security**
   - Seccomp-bpf filtering
   - Docker containerization
   - Multi-layer sandboxing

2. **Scalability**
   - Kubernetes deployment
   - Auto-scaling workers
   - Load balancing

3. **Features**
   - More programming languages
   - Custom test case types
   - Interactive problems
   - Partial scoring

## Implementation Status

✅ **Completed Tasks:**
- Basic queue infrastructure with Asynq/Redis
- Isolate integration with fallback mock
- Core compilation and execution logic
- Test case iteration and verdict generation
- API and database integration

This implementation satisfies all requirements for Task 3 and provides a solid foundation for further development.