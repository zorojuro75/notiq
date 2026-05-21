package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zorojuro75/notiq/config"
	"github.com/zorojuro75/notiq/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	_, err = config.NewPostgres(&cfg.DB)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}

	_, err = config.NewRedis(&cfg.Redis)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}

	pool := worker.NewPool(10, 100)
	pool.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	sig := <-quit
	log.Printf("received signal: %s", sig)

	pool.Shutdown()
}