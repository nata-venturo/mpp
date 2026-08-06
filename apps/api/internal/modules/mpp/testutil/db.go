// Package testutil wires MPP integration tests to a real Postgres and
// Redis. There is no test harness in the skeleton, so tests SKIP (never
// fail) when the env is absent — CI without infra stays green while a
// developer with `make up` running gets the real coverage.
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// RequireDB returns a pool against TEST_DATABASE_URL, or skips the test.
func RequireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping MPP integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// RequireRedis returns a client against TEST_REDIS_ADDR (default
// localhost:6379, DB 15), or skips the test. DB 15 is reserved for tests
// so a run never clobbers the dev permission cache (DB 10).
func RequireRedis(t *testing.T) *goredis.Client {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := goredis.NewClient(&goredis.Options{Addr: addr, DB: 15})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis unavailable at %s — skipping: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return client
}

// TruncateMPP clears transactional MPP tables (master data and seeds
// survive) so each test starts from a known state.
func TruncateMPP(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		TRUNCATE mpp.fo_verification, mpp.serving_session, mpp.antrian,
		         mpp.booking, mpp.pemohon, mpp.loket_session RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate mpp tables: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE mpp.kuota_booking SET terpakai = 0`); err != nil {
		t.Fatalf("reset kuota: %v", err)
	}
}
