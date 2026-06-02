package worker

import (
	"context"
	"log"
	"log/slog"
	"sync"

	"github.com/zorojuro75/notiq/pkg/metrics"
)

type Job func()

type Pool struct {
	numWorkers int
	jobCh      chan Job
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	once       sync.Once
}

func NewPool(numWorkers int, queueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		numWorkers: numWorkers,
		jobCh:      make(chan Job, queueSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (p *Pool) Start() {
	for range p.numWorkers {
		p.wg.Add(1)
		go p.runWorker()
	}
	log.Printf("worker pool started with %d workers", p.numWorkers)
}

func (p *Pool) Submit(job Job) {
	select {
	case p.jobCh <- job:
		metrics.WorkerPoolQueued.Inc()
	case <-p.ctx.Done():
		slog.Warn("pool shutting down — job dropped")
	}
}

func (p *Pool) Shutdown() {
	p.once.Do(func() {
		log.Println("pool shutting down — draining in-flight jobs...")
		p.cancel()
		close(p.jobCh)
		p.wg.Wait()
		log.Println("pool shutdown complete")
	})
}

func (p *Pool) runWorker() {
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.jobCh:
			if !ok {
				return
			}
			metrics.WorkerPoolActive.Inc()
			p.executeJob(job)
			metrics.WorkerPoolActive.Dec()

		case <-p.ctx.Done():
			for {
				select {
				case job, ok := <-p.jobCh:
					if !ok {
						return
					}
					metrics.WorkerPoolActive.Inc()
					p.executeJob(job)
					metrics.WorkerPoolActive.Dec()
				default:
					return
				}
			}
		}
	}
}

func (p *Pool) executeJob(job Job) {
	metrics.WorkerPoolQueued.Dec()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("worker recovered from panic", "error", r)
		}
	}()
	job()
}