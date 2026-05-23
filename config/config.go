package config

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
    App   AppConfig
    DB    DBConfig
    Redis RedisConfig
    Worker WorkerConfig
}

type AppConfig struct {
    Port string
}

type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    SSLMode  string
    LogLevel string
}

type RedisConfig struct {
    Addr     string
    Password string
    DB       int
}

type WorkerConfig struct {
    ShutdownTimeout time.Duration
}

func Load() (*Config, error) {
    viper.SetConfigFile(".env")
    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err != nil {
        log.Printf("no .env file found, reading from environment: %v", err)
    }

    return &Config{
        App: AppConfig{
            Port: viper.GetString("APP_PORT"),
        },
        DB: DBConfig{
            Host:     viper.GetString("DB_HOST"),
            Port:     viper.GetString("DB_PORT"),
            User:     viper.GetString("DB_USER"),
            Password: viper.GetString("DB_PASSWORD"),
            Name:     viper.GetString("DB_NAME"),
            SSLMode:  viper.GetString("DB_SSLMODE"),
            LogLevel: viper.GetString("DB_LOG_LEVEL"),
        },
        Redis: RedisConfig{
            Addr:     viper.GetString("REDIS_ADDR"),
            Password: viper.GetString("REDIS_PASSWORD"),
            DB:       viper.GetInt("REDIS_DB"),
        },
        Worker: WorkerConfig{
            ShutdownTimeout: viper.GetDuration("WORKER_SHUTDOWN_TIMEOUT"),
        },
    }, nil
}

func (c *DBConfig) DSN() string {
    return fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
    )
}