// Package db owns the SQL persistence layer: GORM models, dialect
// detection, and dashboard query helpers.
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

// Connect opens databaseURL ("sqlite://<dsn>", "file:...", or
// "postgres://...") and verifies it with PingContext.
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

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting underlying %s connection: %w", dbType, err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging %s: %w", dbType, err)
	}

	return db, nil
}

// dialector returns the GORM dialector and dialect name for databaseURL.
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

// AutoMigrate syncs the schema with the current models; safe on every startup.
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
