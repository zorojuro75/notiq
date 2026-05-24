package config

import (
	"fmt"

	"github.com/zorojuro75/notiq/internal/repository/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(cfg *DBConfig) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
        Logger: logger.Default.LogMode(gormLogLevel(cfg.LogLevel)),
    })
    if err != nil {
        return nil, fmt.Errorf("connecting to postgres: %w", err)
    }

    sqlDB, err := db.DB()
    if err != nil {
        return nil, fmt.Errorf("getting sql.DB: %w", err)
    }

    sqlDB.SetMaxOpenConns(25)
    sqlDB.SetMaxIdleConns(10)

    return db, nil
}

func RunMigrations(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.Job{},
        &models.Webhook{},
    )
}

func gormLogLevel(level string) logger.LogLevel {
    switch level {
    case "info":
        return logger.Info
    case "warn":
        return logger.Warn
    case "error":
        return logger.Error
    case "silent":
        return logger.Silent
    default:
        return logger.Warn
    }
}