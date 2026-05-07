package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const dialectSQLite = "sqlite"

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
	if dsn, ok := strings.CutPrefix(databaseURL, "sqlite://"); ok {
		if dsn == "" {
			return nil, "", errors.New("missing sqlite dsn")
		}
		return sqlite.Open(dsn), dialectSQLite, nil
	}

	if strings.HasPrefix(databaseURL, "file:") {
		return sqlite.Open(databaseURL), dialectSQLite, nil
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
