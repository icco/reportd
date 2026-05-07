package db

import (
	"context"
	"testing"
)

func TestConnectSQLiteAndQueryHelpers(t *testing.T) {
	ctx := context.Background()

	d, err := Connect(ctx, "sqlite://file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := AutoMigrate(ctx, d); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	const service = "svc"
	seedQueryFixtures(t, d, service)
	assertQueryHelpers(ctx, t, d, service)
}
