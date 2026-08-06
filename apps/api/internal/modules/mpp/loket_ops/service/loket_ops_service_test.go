package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	antriandto "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/dto"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
	loket1ID   = "a5000000-0000-0000-0000-000000000001"
	loket2ID   = "a5000000-0000-0000-0000-000000000002"
	petugasID  = "a9000000-0000-0000-0000-000000000001"
	// A second operator, so "someone else holds this counter" is testable.
	supervisorID = "a9000000-0000-0000-0000-000000000003"
)

var jakarta = mustJakarta()

func mustJakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

type harness struct {
	pool    *pgxpool.Pool
	ops     *service.LoketOpsService
	antrian *antriansvc.AntrianService
}

func setup(t *testing.T) *harness {
	t.Helper()

	pool := testutil.RequireDB(t)
	rdb := testutil.RequireRedis(t)
	testutil.TruncateMPP(t, pool)

	cfg := configsvc.NewConfigService(configrepo.NewConfigRepository(pool), jakarta)
	instRepo := instansirepo.NewInstansiRepository(pool, companyID)
	lokRepo := loketrepo.NewLoketRepository(pool, companyID)
	antRepo := antrianrepo.NewAntrianRepository(pool, rdb)

	antrian := antriansvc.NewAntrianService(
		antRepo, instRepo, lokRepo,
		bookingrepo.NewBookingRepository(pool, companyID),
		cfg, companyID, jakarta,
	)

	key := "mpp:counter:" + instansiID + ":" + antrian.OperatingDay().Format("20060102")
	if err := rdb.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("flush counter: %v", err)
	}

	// hub nil: publishing is best-effort and not what these tests assert.
	ops := service.NewLoketOpsService(
		repository.NewLoketOpsRepository(pool), lokRepo, antRepo, instRepo, antrian, cfg, nil, companyID)

	return &harness{pool: pool, ops: ops, antrian: antrian}
}

func (h *harness) enqueue(t *testing.T, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		_, err := h.antrian.WalkIn(context.Background(), &antriandto.WalkInRequest{
			InstansiID: instansiID,
			LayananID:  layananID,
			Pemohon: antriandto.PemohonRequest{
				Name:  fmt.Sprintf("Antre %d", i),
				Phone: fmt.Sprintf("62815000%04d", i),
			},
		})
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
}

func (h *harness) openSession(t *testing.T, loketID, userID string) {
	t.Helper()

	if _, err := h.ops.Session(context.Background(), loketID, userID, "open"); err != nil {
		t.Fatalf("open session on %s: %v", loketID, err)
	}
}

// ----------------------------------------------------------------------

func TestSessionOpenThenClose(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	opened, err := h.ops.Session(ctx, loket1ID, petugasID, "open")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !opened.IsActive || opened.LoketName != "Loket 1" {
		t.Fatalf("session = %+v, want active session on Loket 1", opened)
	}

	closed, err := h.ops.Session(ctx, loket1ID, petugasID, "close")
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.IsActive || closed.ClosedAt == nil {
		t.Fatalf("session = %+v, want closed", closed)
	}
}

func TestSessionRefusesACounterHeldByAnotherOperator(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.openSession(t, loket1ID, petugasID)

	if _, err := h.ops.Session(ctx, loket1ID, supervisorID, "open"); !errors.Is(err, service.ErrNotYourLoket) {
		t.Fatalf("second operator open = %v, want ErrNotYourLoket", err)
	}
}

func TestCallNextOnEmptyStreamReturnsNothing(t *testing.T) {
	h := setup(t)
	h.openSession(t, loket1ID, petugasID)

	res, err := h.ops.CallNext(context.Background(), loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if res != nil {
		t.Fatalf("CallNext on an empty queue = %+v, want nil", res)
	}
}

func TestCallNextRequiresAnOpenSession(t *testing.T) {
	h := setup(t)
	h.enqueue(t, 1)

	_, err := h.ops.CallNext(context.Background(), loket1ID, petugasID)
	if !errors.Is(err, service.ErrNotYourLoket) {
		t.Fatalf("CallNext without a session = %v, want ErrNotYourLoket", err)
	}
}

// The full happy path of the slice: call → recall ×2 → start → done.
func TestCallRecallStartDone(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 2)
	h.openSession(t, loket1ID, petugasID)

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if called.Nomor != "A-001" || called.Status != "CALLED" || called.CallCount != 1 {
		t.Fatalf("called = %+v, want A-001 CALLED call_count 1", called)
	}
	if called.TTSText != "Nomor antrian A - nol nol satu, silakan menuju loket satu" {
		t.Fatalf("tts_text = %q", called.TTSText)
	}

	for want := 2; want <= repository.MaxCallCount; want++ {
		res, err := h.ops.Recall(ctx, called.AntrianID, petugasID)
		if err != nil {
			t.Fatalf("recall to %d: %v", want, err)
		}
		if res.CallCount != want {
			t.Fatalf("call_count = %d, want %d", res.CallCount, want)
		}
	}

	// BR-16: the fourth call is refused, not silently capped.
	if _, err := h.ops.Recall(ctx, called.AntrianID, petugasID); !errors.Is(err, service.ErrNoTransition) {
		t.Fatalf("4th recall = %v, want ErrNoTransition", err)
	}

	started, err := h.ops.Start(ctx, called.AntrianID, petugasID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Status != "SERVING" {
		t.Fatalf("status = %q, want SERVING", started.Status)
	}

	done, err := h.ops.Done(ctx, called.AntrianID, petugasID)
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if done.Status != "DONE" {
		t.Fatalf("status = %q, want DONE", done.Status)
	}
	if done.DurasiDetik == nil || done.DoneAt == nil {
		t.Fatalf("done response missing duration/timestamp: %+v", done)
	}

	var outcome string
	if err := h.pool.QueryRow(ctx,
		`SELECT outcome::text FROM mpp.serving_session WHERE antrian_id = $1`,
		called.AntrianID).Scan(&outcome); err != nil {
		t.Fatalf("read serving session: %v", err)
	}
	if outcome != "DONE" {
		t.Fatalf("serving_session outcome = %q, want DONE", outcome)
	}
}

func TestStartRequiresCalled(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 1)
	h.openSession(t, loket1ID, petugasID)

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if _, err := h.ops.Start(ctx, called.AntrianID, petugasID); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := h.ops.Start(ctx, called.AntrianID, petugasID); !errors.Is(err, service.ErrNoTransition) {
		t.Fatalf("second Start = %v, want ErrNoTransition", err)
	}
}

func TestDoneRequiresServing(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 1)
	h.openSession(t, loket1ID, petugasID)

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}

	if _, err := h.ops.Done(ctx, called.AntrianID, petugasID); !errors.Is(err, service.ErrNoTransition) {
		t.Fatalf("Done on a CALLED item = %v, want ErrNoTransition", err)
	}
}

func TestSkipFreesTheCounter(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 1)
	h.openSession(t, loket1ID, petugasID)

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}

	var before time.Time
	if err := h.pool.QueryRow(ctx,
		`SELECT last_idle_at FROM mpp.loket WHERE id = $1`, loket1ID).Scan(&before); err != nil {
		t.Fatalf("read last_idle_at: %v", err)
	}

	skipped, err := h.ops.Skip(ctx, called.AntrianID, petugasID)
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if skipped.Status != "SKIPPED" {
		t.Fatalf("status = %q, want SKIPPED", skipped.Status)
	}

	var after time.Time
	if err := h.pool.QueryRow(ctx,
		`SELECT last_idle_at FROM mpp.loket WHERE id = $1`, loket1ID).Scan(&after); err != nil {
		t.Fatalf("read last_idle_at: %v", err)
	}
	if !after.After(before) {
		t.Fatalf("last_idle_at not refreshed on skip (%s → %s)", before, after)
	}
}

// TestConcurrentCallNextNeverDoubleCalls is the SKIP LOCKED proof: two
// counters pressing "Panggil berikutnya" at the same instant must take
// two different people.
func TestConcurrentCallNextNeverDoubleCalls(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 6)
	h.openSession(t, loket1ID, petugasID)
	h.openSession(t, loket2ID, supervisorID)

	type call struct {
		loket string
		user  string
	}
	calls := []call{{loket1ID, petugasID}, {loket2ID, supervisorID}}

	var wg sync.WaitGroup
	ids := make(chan string, len(calls))

	for _, c := range calls {
		wg.Add(1)
		go func(c call) {
			defer wg.Done()

			res, err := h.ops.CallNext(ctx, c.loket, c.user)
			if err != nil {
				t.Errorf("CallNext on %s: %v", c.loket, err)
				return
			}
			if res == nil {
				t.Errorf("CallNext on %s returned nothing with 6 people waiting", c.loket)
				return
			}
			ids <- res.AntrianID
		}(c)
	}

	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("two counters called the same person (%s)", id)
		}
		seen[id] = true
	}
	if len(seen) != 2 {
		t.Fatalf("got %d distinct calls, want 2", len(seen))
	}
}
