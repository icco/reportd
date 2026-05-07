package db

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	dialector, dbType, err := dialector(databaseURL)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", dbType, err)
	}

	return db, nil
}

func dialector(databaseURL string) (gorm.Dialector, string, error) {
	if strings.HasPrefix(databaseURL, "sqlite://") {
		dsn := strings.TrimPrefix(databaseURL, "sqlite://")
		if dsn == "" {
			return nil, "", fmt.Errorf("connecting to sqlite: missing sqlite dsn")
		}
		return sqlite.Open(dsn), "sqlite", nil
	}

	if strings.HasPrefix(databaseURL, "file:") {
		return sqlite.Open(databaseURL), "sqlite", nil
	}

	return postgres.Open(databaseURL), "postgres", nil
}

func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(
		&WebVital{},
		&ReportToEntry{},
		&SecurityReportEntry{},
	); err != nil {
		return fmt.Errorf("auto-migrating: %w", err)
	}

	return nil
}
