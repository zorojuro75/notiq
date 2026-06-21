package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/worker/handlers"
	"github.com/zorojuro75/notiq/pkg/queue"
	"github.com/zorojuro75/notiq/pkg/retry"
)

type Processor struct {
	server          *asynq.Server
	mux             *asynq.ServeMux
	emailHandler    *handlers.EmailHandler
	smsHandler      *handlers.SMSHandler
	webhookHandler  *handlers.WebhookHandler
	reportHandler   *handlers.ReportHandler
	deliveryHandler *handlers.WebhookDeliveryHandler
}

func NewProcessor(
	redisAddr, redisPassword string,
	redisDB int,
	concurrency int,
	emailHandler *handlers.EmailHandler,
	smsHandler *handlers.SMSHandler,
	webhookHandler *handlers.WebhookHandler,
	reportHandler *handlers.ReportHandler,
	deliveryHandler *handlers.WebhookDeliveryHandler,
) *Processor {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		},
		asynq.Config{
			// asynq runs up to Concurrency handler goroutines and bounds
			// parallelism itself — no separate worker pool is needed.
			Concurrency: concurrency,

			RetryDelayFunc: func(attempt int, err error, task *asynq.Task) time.Duration {
				delay := retry.Backoff(attempt)
				slog.Warn("retry scheduled",
					"task_type", task.Type(),
					"attempt", attempt,
					"next_in", delay.String(),
					"error", err.Error(),
				)
				return delay
			},

			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("task error",
					"task_type", task.Type(),
					"error", err.Error(),
				)
			}),
		},
	)

	mux := asynq.NewServeMux()

	p := &Processor{
		server:          srv,
		mux:             mux,
		emailHandler:    emailHandler,
		smsHandler:      smsHandler,
		webhookHandler:  webhookHandler,
		reportHandler:   reportHandler,
		deliveryHandler: deliveryHandler,
	}

	p.registerHandlers()
	return p
}

func (p *Processor) registerHandlers() {
	// Handlers are invoked directly on asynq's worker goroutines.
	p.mux.HandleFunc(queue.TypeEmail, p.emailHandler.Handle)
	p.mux.HandleFunc(queue.TypeSMS, p.smsHandler.Handle)
	p.mux.HandleFunc(queue.TypeWebhook, p.webhookHandler.Handle)
	p.mux.HandleFunc(queue.TypeReport, p.reportHandler.Handle)
	p.mux.HandleFunc(queue.TypeWebhookDelivery, p.deliveryHandler.Handle)
}

func (p *Processor) Start() error {
	slog.Info("processor starting — listening for tasks")
	return p.server.Start(p.mux)
}

func (p *Processor) Shutdown() {
	slog.Info("processor shutting down")
	p.server.Shutdown()
}
