package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zorojuro75/notiq/internal/repository/models"
)

// setupPostgres starts a real Postgres container and returns a GORM connection.
// The container is destroyed when the test finishes.
func setupPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithDatabase("notiq_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		// wait until Postgres is actually accepting connections
		// not just when the port opens
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminating postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}

	if err := db.AutoMigrate(&models.Job{}, &models.Webhook{}); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	return db
}
// setupRedis starts a real Redis container and returns an asynq client.
func setupRedis(t *testing.T) (*asynq.Client, string) {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.RunContainer(ctx)
	if err != nil {
		t.Fatalf("starting redis container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminating redis container: %v", err)
		}
	})

	addr, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("getting redis endpoint: %v", err)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: addr})
	t.Cleanup(func() { client.Close() })

	return client, addr
}

// waitForJobStatus polls Postgres until the job reaches the expected status
// or times out. Useful in tests where the worker processes asynchronously.
func waitForJobStatus(t *testing.T, db *gorm.DB, jobID string, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var m models.Job
		if err := db.First(&m, "id = ?", jobID).Error; err == nil {
			if m.Status == expected {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach status %s within %s", jobID, expected, timeout)
}