package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// TestConnectPostgresAndQueryHelpers exercises the same query helpers as the
// SQLite test against a real Postgres so we catch dialect-specific bugs
// (placeholder expansion, COALESCE/NULLIF behavior, date scanning, etc.).
//
// It is skipped unless TEST_DATABASE_URL is set to a postgres:// connection
// string. CI provides a service container; locally you can point it at any
// throwaway database — the test isolates rows under a randomly-generated
// service name and cleans them up on teardown.
func TestConnectPostgresAndQueryHelpers(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" || !strings.HasPrefix(dsn, "postgres://") {
		t.Skip("TEST_DATABASE_URL not set to a postgres:// URL; skipping postgres integration test")
	}

	ctx := context.Background()

	d, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := AutoMigrate(ctx, d); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	service := "test-" + randHex(t, 8)
	t.Cleanup(func() { cleanupService(t, d, service) })

	seedQueryFixtures(t, d, service)
	assertQueryHelpers(ctx, t, d, service)
}

func randHex(t *testing.T, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return hex.EncodeToString(buf)
}

func cleanupService(t *testing.T, d *gorm.DB, service string) {
	t.Helper()
	for _, model := range []any{&WebVital{}, &ReportToEntry{}, &SecurityReportEntry{}} {
		if err := d.Unscoped().Where("service = ?", service).Delete(model).Error; err != nil {
			t.Logf("cleanup %T for service %q: %v", model, service, err)
		}
	}
}
