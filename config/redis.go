package config

import (
    "context"
    "fmt"

    "github.com/redis/go-redis/v9"
)

func NewRedis(cfg *RedisConfig) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     cfg.Addr,
        Password: cfg.Password,
        DB:       cfg.DB,
    })

    if err := client.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("connecting to redis: %w", err)
    }

    return client, nil
}