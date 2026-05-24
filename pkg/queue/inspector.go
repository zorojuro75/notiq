package queue

import (
	"fmt"
	"log"

	"github.com/hibiken/asynq"
)

type Inspector struct {
	inspector *asynq.Inspector
}

func NewInspector(redisAddr, redisPassword string, redisDB int) *Inspector {
	return &Inspector{
		inspector: asynq.NewInspector(asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		}),
	}
}

// DeleteTask removes a task from any queue by its task ID.
// Used when cancelling a scheduled job — removes it from the scheduled set immediately.
// Returns nil if the task doesn't exist — that's fine, it may have already run.
func (i *Inspector) DeleteTask(queueName, taskID string) error {
	err := i.inspector.DeleteTask(queueName, taskID)
	if err != nil {
		if err == asynq.ErrTaskNotFound {
			log.Printf("[INSPECTOR] task %s not found in queue %s — already processed", taskID, queueName)
			return nil
		}
		return fmt.Errorf("deleting task %s from queue %s: %w", taskID, queueName, err)
	}
	log.Printf("[INSPECTOR] task %s deleted from queue %s", taskID, queueName)
	return nil
}

// Close cleans up the inspector connection.
func (i *Inspector) Close() error {
	return i.inspector.Close()
}