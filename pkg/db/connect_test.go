package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestDialector(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantDialect string
		wantErr     bool
	}{
		{name: "sqlite scheme", url: "sqlite:///tmp/x.db", wantDialect: dialectSQLite},
		{name: "sqlite scheme with empty dsn", url: "sqlite://", wantErr: true},
		{name: "file prefix", url: "file::memory:", wantDialect: dialectSQLite},
		{name: "anything else is postgres", url: "postgres://localhost/db", wantDialect: "postgres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, dialect, err := dialector(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("dialector() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("dialector() error = %v", err)
			}
			if dialect != tt.wantDialect {
				t.Errorf("dialect = %q, want %q", dialect, tt.wantDialect)
			}
			if d == nil {
				t.Error("dialector() returned a nil dialector")
			}
		})
	}
}

func TestConnectRejectsEmptySQLiteDSN(t *testing.T) {
	if _, err := Connect(context.Background(), "sqlite://"); err == nil {
		t.Fatal("Connect() error = nil, want an error")
	}
}

// TestConnectFailsOnUnreachablePostgres covers the failure path after the
// dialector is built: port 1 is never listening, so opening or pinging fails.
// The DSN carries no userinfo — the connection never gets far enough to
// authenticate, and a password here would trip gosec's G101.
func TestConnectFailsOnUnreachablePostgres(t *testing.T) {
	dsn := "postgres://127.0.0.1:1/none?sslmode=disable&connect_timeout=1"
	_, err := Connect(context.Background(), dsn)
	if err == nil {
		t.Fatal("Connect() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Errorf("error = %v, want it to mention the dialect", err)
	}
}

func TestAutoMigrateFailsOnClosedConnection(t *testing.T) {
	ctx := context.Background()
	d, err := Connect(ctx, "sqlite://"+filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	sqlDB, err := d.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := AutoMigrate(ctx, d); err == nil {
		t.Fatal("AutoMigrate() error = nil, want an error")
	}
}
