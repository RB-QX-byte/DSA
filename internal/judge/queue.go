package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	// Task type names
	TaskTypeJudgeSubmission = "judge:submission"
	TaskTypePing           = "judge:ping"
)

// QueueManager manages the Asynq client and server
type QueueManager struct {
	Client *asynq.Client
	Server *asynq.Server
	Redis  *redis.Client
}

// NewQueueManager creates a new queue manager
func NewQueueManager() (*QueueManager, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0 // Default Redis DB

	redisOpt := asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	}

	client := asynq.NewClient(redisOpt)
	
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})

	// Create Redis client for direct operations
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	return &QueueManager{
		Client: client,
		Server: server,
		Redis:  rdb,
	}, nil
}

// Close closes the queue manager connections
func (qm *QueueManager) Close() error {
	if err := qm.Client.Close(); err != nil {
		return fmt.Errorf("failed to close asynq client: %w", err)
	}
	if err := qm.Redis.Close(); err != nil {
		return fmt.Errorf("failed to close redis client: %w", err)
	}
	return nil
}

// EnqueueSubmission enqueues a submission for judging
func (qm *QueueManager) EnqueueSubmission(ctx context.Context, payload *SubmissionPayload) error {
	tracer := otel.Tracer("judge-queue")
	ctx, span := tracer.Start(ctx, "queue.enqueue_submission")
	defer span.End()

	span.SetAttributes(
		attribute.String("queue.task_type", TaskTypeJudgeSubmission),
		attribute.String("queue.name", "default"),
		attribute.String("submission.id", payload.SubmissionID),
		attribute.String("submission.user_id", payload.UserID),
		attribute.String("submission.problem_id", payload.ProblemID),
	)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal submission payload: %w", err)
	}

	task := asynq.NewTask(TaskTypeJudgeSubmission, payloadBytes)
	
	info, err := qm.Client.EnqueueContext(ctx, task, asynq.Queue("default"))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to enqueue submission task: %w", err)
	}

	span.SetAttributes(attribute.String("queue.task_id", info.ID))
	log.Printf("Enqueued submission task: %s", info.ID)
	return nil
}

// EnqueuePing enqueues a ping task for testing
func (qm *QueueManager) EnqueuePing(ctx context.Context, message string) error {
	tracer := otel.Tracer("judge-queue")
	ctx, span := tracer.Start(ctx, "queue.enqueue_ping")
	defer span.End()

	span.SetAttributes(
		attribute.String("queue.task_type", TaskTypePing),
		attribute.String("queue.name", "low"),
		attribute.String("ping.message", message),
	)

	payload := map[string]string{"message": message}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal ping payload: %w", err)
	}

	task := asynq.NewTask(TaskTypePing, payloadBytes)
	
	info, err := qm.Client.EnqueueContext(ctx, task, asynq.Queue("low"))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to enqueue ping task: %w", err)
	}

	span.SetAttributes(attribute.String("queue.task_id", info.ID))
	log.Printf("Enqueued ping task: %s", info.ID)
	return nil
}

// RegisterHandlers registers task handlers with the server
func (qm *QueueManager) RegisterHandlers(judgeService *JudgeService) {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeJudgeSubmission, judgeService.HandleSubmissionTask)
	mux.HandleFunc(TaskTypePing, HandlePingTask)
	
	qm.Server = asynq.NewServer(asynq.RedisClientOpt{
		Addr:     qm.Redis.Options().Addr,
		Password: qm.Redis.Options().Password,
		DB:       qm.Redis.Options().DB,
	}, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})
	
	if err := qm.Server.Start(mux); err != nil {
		log.Fatal("Failed to start asynq server:", err)
	}
}

// HandlePingTask handles ping tasks for testing
func HandlePingTask(ctx context.Context, t *asynq.Task) error {
	tracer := otel.Tracer("judge-queue")
	ctx, span := tracer.Start(ctx, "queue.handle_ping")
	defer span.End()

	span.SetAttributes(
		attribute.String("queue.task_type", t.Type()),
		attribute.String("queue.task_id", t.ResultWriter().TaskID()),
	)

	var payload map[string]string
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to unmarshal ping payload: %w", err)
	}

	message := payload["message"]
	span.SetAttributes(attribute.String("ping.message", message))
	log.Printf("Ping task received: %s", message)
	return nil
}

// GetQueueStats returns basic queue statistics
func (qm *QueueManager) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Get queue lengths
	for queue := range map[string]int{"critical": 0, "default": 0, "low": 0} {
		length, err := qm.Redis.LLen(ctx, fmt.Sprintf("asynq:{%s}", queue)).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get queue length for %s: %w", queue, err)
		}
		stats[queue+"_length"] = length
	}
	
	return stats, nil
}