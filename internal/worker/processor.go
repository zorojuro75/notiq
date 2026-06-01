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
	server         *asynq.Server
	mux            *asynq.ServeMux
	pool           *Pool
	emailHandler   *handlers.EmailHandler
	smsHandler     *handlers.SMSHandler
	webhookHandler *handlers.WebhookHandler
	reportHandler  *handlers.ReportHandler
}

func NewProcessor(
	redisAddr, redisPassword string,
	redisDB int,
	pool *Pool,
	emailHandler *handlers.EmailHandler,
	smsHandler *handlers.SMSHandler,
	webhookHandler *handlers.WebhookHandler,
	reportHandler *handlers.ReportHandler,
) *Processor {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		},
		asynq.Config{
			Concurrency: pool.numWorkers,

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
		server:         srv,
		mux:            mux,
		pool:           pool,
		emailHandler:   emailHandler,
		smsHandler:     smsHandler,
		webhookHandler: webhookHandler,
		reportHandler:  reportHandler,
	}

	p.registerHandlers()
	return p
}

func (p *Processor) registerHandlers() {
	p.mux.HandleFunc(queue.TypeEmail, p.wrap(p.emailHandler))
	p.mux.HandleFunc(queue.TypeSMS, p.wrap(p.smsHandler))
	p.mux.HandleFunc(queue.TypeWebhook, p.wrap(p.webhookHandler))
	p.mux.HandleFunc(queue.TypeReport, p.wrap(p.reportHandler))
}

func (p *Processor) wrap(h handlers.JobHandler) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var handlerErr error

		done := make(chan struct{})
		p.pool.Submit(func() {
			defer close(done)
			handlerErr = h.Handle(ctx, task)
		})

		<-done
		return handlerErr
	}
}

func (p *Processor) Start() error {
	slog.Info("processor starting — listening for tasks")
	return p.server.Start(p.mux)
}

func (p *Processor) Shutdown() {
	slog.Info("processor shutting down")
	p.server.Shutdown()
}