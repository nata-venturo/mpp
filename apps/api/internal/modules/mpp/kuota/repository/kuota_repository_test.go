package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
)

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// shrinkQuota narrows today's agency-wide row to `seats` and restores the
// seeded value afterwards, so a run never leaves the demo data mangled.
func shrinkQuota(t *testing.T, pool *pgxpool.Pool, seats int) {
	t.Helper()

	ctx := context.Background()
	var original int
	err := pool.QueryRow(ctx, `
		SELECT kuota FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, today()).Scan(&original)
	if err != nil {
		t.Fatalf("read seeded quota: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE mpp.kuota_booking SET kuota = $3, terpakai = 0
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, today(), seats); err != nil {
		t.Fatalf("shrink quota: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			UPDATE mpp.kuota_booking SET kuota = $3, terpakai = 0
			WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
			instansiID, today(), original)
	})
}

func TestFindSlotFallsBackToAgencyWideRow(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	repo := repository.NewKuotaRepository(pool, companyID)
	svc := layananID // no per-service quota row is seeded

	slot, err := repo.FindSlot(context.Background(), instansiID, &svc, today())
	if err != nil {
		t.Fatalf("FindSlot: %v", err)
	}
	if slot == nil {
		t.Fatal("want the agency-wide slot, got nil")
	}
	if slot.JenisLayananID != nil {
		t.Errorf("want an agency-wide slot (NULL service), got %v", *slot.JenisLayananID)
	}
	if slot.Terpakai != 0 {
		t.Errorf("terpakai = %d, want 0 after truncate", slot.Terpakai)
	}
}

func TestFindSlotUnknownDayReturnsNil(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewKuotaRepository(pool, companyID)

	slot, err := repo.FindSlot(context.Background(), instansiID, nil, today().AddDate(1, 0, 0))
	if err != nil {
		t.Fatalf("FindSlot: %v", err)
	}
	if slot != nil {
		t.Fatalf("want nil for an unconfigured day, got %+v", slot)
	}
}

func TestConsumeIncrementsThenRejectsWhenFull(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	shrinkQuota(t, pool, 1)

	ctx := context.Background()
	repo := repository.NewKuotaRepository(pool, companyID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback-on-exit is the point

	if err := repo.Consume(ctx, tx, instansiID, nil, today()); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	if err := repo.Consume(ctx, tx, instansiID, nil, today()); !errors.Is(err, repository.ErrQuotaFull) {
		t.Fatalf("second Consume err = %v, want ErrQuotaFull", err)
	}
}

func TestConsumeWithoutQuotaRowIsFull(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	repo := repository.NewKuotaRepository(pool, companyID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	err = repo.Consume(ctx, tx, instansiID, nil, today().AddDate(1, 0, 0))
	if !errors.Is(err, repository.ErrQuotaFull) {
		t.Fatalf("Consume on an unconfigured day = %v, want ErrQuotaFull", err)
	}
}

// TestConsumeIsRaceFree is the NFR-DATA-02 proof: N concurrent consumers
// against a 1-seat quota must produce exactly one winner and terpakai == 1.
func TestConsumeIsRaceFree(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	shrinkQuota(t, pool, 1)

	ctx := context.Background()
	repo := repository.NewKuotaRepository(pool, companyID)

	const n = 20
	var wg sync.WaitGroup
	results := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			tx, err := pool.Begin(ctx)
			if err != nil {
				results <- err
				return
			}

			if err := repo.Consume(ctx, tx, instansiID, nil, today()); err != nil {
				_ = tx.Rollback(ctx)
				results <- err
				return
			}
			results <- tx.Commit(ctx)
		}()
	}

	wg.Wait()
	close(results)

	var ok, full int
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, repository.ErrQuotaFull):
			full++
		default:
			t.Fatalf("unexpected Consume error: %v", err)
		}
	}

	if ok != 1 || full != n-1 {
		t.Fatalf("winners = %d, quota-full = %d; want 1 and %d", ok, full, n-1)
	}

	var terpakai int
	if err := pool.QueryRow(ctx, `
		SELECT terpakai FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, today()).Scan(&terpakai); err != nil {
		t.Fatalf("read terpakai: %v", err)
	}
	if terpakai != 1 {
		t.Fatalf("terpakai = %d after %d concurrent bookings, want 1 (overbooking!)", terpakai, n)
	}
}

func TestReleaseIsTheInverseOfConsume(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	shrinkQuota(t, pool, 2)

	ctx := context.Background()
	repo := repository.NewKuotaRepository(pool, companyID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := repo.Consume(ctx, tx, instansiID, nil, today()); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := repo.Release(ctx, tx, instansiID, nil, today()); err != nil {
		t.Fatalf("Release: %v", err)
	}

	var terpakai int
	if err := tx.QueryRow(ctx, `
		SELECT terpakai FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, today()).Scan(&terpakai); err != nil {
		t.Fatalf("read terpakai: %v", err)
	}
	if terpakai != 0 {
		t.Fatalf("terpakai = %d after consume+release, want 0", terpakai)
	}
}
