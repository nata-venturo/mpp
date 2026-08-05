package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
)

var jakarta = mustJakarta()

func mustJakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

func newService(pool *pgxpool.Pool, rdb *goredis.Client) *service.AntrianService {
	return service.NewAntrianService(
		antrianrepo.NewAntrianRepository(pool, rdb),
		instansirepo.NewInstansiRepository(pool, companyID),
		loketrepo.NewLoketRepository(pool, companyID),
		bookingrepo.NewBookingRepository(pool, companyID),
		configsvc.NewConfigService(configrepo.NewConfigRepository(pool), jakarta),
		companyID,
		jakarta,
	)
}

// setup gives a clean DB and a counter key that starts cold.
func setup(t *testing.T) (*pgxpool.Pool, *goredis.Client, *service.AntrianService) {
	t.Helper()

	pool := testutil.RequireDB(t)
	rdb := testutil.RequireRedis(t)
	testutil.TruncateMPP(t, pool)

	svc := newService(pool, rdb)
	flushCounter(t, rdb, svc.OperatingDay())

	return pool, rdb, svc
}

func flushCounter(t *testing.T, rdb *goredis.Client, day time.Time) {
	t.Helper()

	key := fmt.Sprintf("mpp:counter:%s:%s", instansiID, day.Format("20060102"))
	if err := rdb.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("flush counter: %v", err)
	}
}

func walkInRequest(name, phone string) *dto.WalkInRequest {
	return &dto.WalkInRequest{
		InstansiID: instansiID,
		LayananID:  layananID,
		Pemohon:    dto.PemohonRequest{Name: name, Phone: phone},
	}
}

// ----------------------------------------------------------------------
// Pure: the printed number format (BR-04).

func TestFormatNomor(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		seq    int
		format configsvc.NumberFormat
		want   string
	}{
		{"default pad 3", "A", 14, configsvc.NumberFormat{Pattern: "{prefix}-{seq}", Pad: 3}, "A-014"},
		{"no separator", "B", 7, configsvc.NumberFormat{Pattern: "{prefix}{seq}", Pad: 3}, "B007"},
		{"no padding", "C", 5, configsvc.NumberFormat{Pattern: "{prefix}-{seq}", Pad: 0}, "C-5"},
		{"seq longer than pad", "A", 1234, configsvc.NumberFormat{Pattern: "{prefix}-{seq}", Pad: 3}, "A-1234"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.FormatNomor(tc.prefix, tc.seq, tc.format); got != tc.want {
				t.Fatalf("FormatNomor = %q, want %q", got, tc.want)
			}
		})
	}
}

// ----------------------------------------------------------------------
// Numbering.

func TestWalkInIssuesSequentialNumbers(t *testing.T) {
	_, _, svc := setup(t)
	ctx := context.Background()

	first, err := svc.WalkIn(ctx, walkInRequest("Budi", "628130000001"))
	if err != nil {
		t.Fatalf("first WalkIn: %v", err)
	}
	if first.Nomor != "A-001" || first.NomorSeq != 1 {
		t.Fatalf("first ticket = %s (seq %d), want A-001 (1)", first.Nomor, first.NomorSeq)
	}
	if first.QueueStatus != domain.StatusWaiting {
		t.Errorf("status = %q, want WAITING", first.QueueStatus)
	}

	second, err := svc.WalkIn(ctx, walkInRequest("Citra", "628130000002"))
	if err != nil {
		t.Fatalf("second WalkIn: %v", err)
	}
	if second.Nomor != "A-002" {
		t.Fatalf("second ticket = %s, want A-002", second.Nomor)
	}
}

// TestEnqueueIsRaceFree proves numbering is atomic: N concurrent
// registrations produce 1..N exactly once each — no duplicate, no gap.
func TestEnqueueIsRaceFree(t *testing.T) {
	pool, _, svc := setup(t)
	ctx := context.Background()

	const n = 25
	var wg sync.WaitGroup
	numbers := make(chan int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			res, err := svc.WalkIn(ctx, walkInRequest(
				fmt.Sprintf("Warga %d", i), fmt.Sprintf("62813100%04d", i)))
			if err != nil {
				t.Errorf("WalkIn %d: %v", i, err)
				return
			}
			numbers <- res.NomorSeq
		}(i)
	}

	wg.Wait()
	close(numbers)

	seen := make(map[int]bool, n)
	for seq := range numbers {
		if seen[seq] {
			t.Fatalf("sequence %d issued twice", seq)
		}
		seen[seq] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct numbers, want %d", len(seen), n)
	}
	for i := 1; i <= n; i++ {
		if !seen[i] {
			t.Fatalf("sequence %d missing — numbering has a gap", i)
		}
	}

	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM mpp.antrian`).Scan(&rows); err != nil {
		t.Fatalf("count antrian: %v", err)
	}
	if rows != n {
		t.Fatalf("%d antrian rows, want %d", rows, n)
	}
}

// TestColdStartReseedsFromDatabase is the Redis-loss case: the counter
// must resume above the highest number already printed, never restart
// at 1 and hand out a duplicate.
func TestColdStartReseedsFromDatabase(t *testing.T) {
	_, rdb, svc := setup(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := svc.WalkIn(ctx, walkInRequest(
			fmt.Sprintf("Awal %d", i), fmt.Sprintf("62813200%04d", i))); err != nil {
			t.Fatalf("seed WalkIn %d: %v", i, err)
		}
	}

	flushCounter(t, rdb, svc.OperatingDay())

	next, err := svc.WalkIn(ctx, walkInRequest("Setelah Flush", "628132009999"))
	if err != nil {
		t.Fatalf("WalkIn after flush: %v", err)
	}
	if next.NomorSeq != 6 {
		t.Fatalf("sequence after Redis flush = %d, want 6 (reseeded from MAX(nomor_seq))", next.NomorSeq)
	}
	if next.Nomor != "A-006" {
		t.Fatalf("nomor = %s, want A-006", next.Nomor)
	}
}

// ----------------------------------------------------------------------
// Stream + ETA.

func TestQueueListsWaitingInArrivalOrder(t *testing.T) {
	_, _, svc := setup(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := svc.WalkIn(ctx, walkInRequest(
			fmt.Sprintf("Antre %d", i), fmt.Sprintf("62813300%04d", i))); err != nil {
			t.Fatalf("WalkIn %d: %v", i, err)
		}
	}

	list, total, err := svc.Queue(ctx, layananID, 1, 50)
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("queue total = %d (len %d), want 3", total, len(list))
	}
	if list[0].Nomor != "A-001" || list[2].Nomor != "A-003" {
		t.Fatalf("queue order = %s..%s, want A-001..A-003", list[0].Nomor, list[2].Nomor)
	}
	if list[0].Status != domain.StatusWaiting {
		t.Errorf("status = %q, want WAITING", list[0].Status)
	}
}

// The seeded agency runs 3 OPEN lokets and a 10-minute service, so the
// 4th person in line waits one full round (BR-29).
func TestEtaGrowsWithTheQueue(t *testing.T) {
	_, _, svc := setup(t)
	ctx := context.Background()

	first, err := svc.WalkIn(ctx, walkInRequest("Pertama", "628134000001"))
	if err != nil {
		t.Fatalf("WalkIn: %v", err)
	}
	if first.EtaMenit != 0 {
		t.Errorf("first ticket eta = %d, want 0 (nobody ahead)", first.EtaMenit)
	}

	var last *dto.AntrianResponse
	for i := 1; i < 4; i++ {
		last, err = svc.WalkIn(ctx, walkInRequest(
			fmt.Sprintf("Berikut %d", i), fmt.Sprintf("62813400%04d", i)))
		if err != nil {
			t.Fatalf("WalkIn %d: %v", i, err)
		}
	}

	// 3 ahead / 3 lokets = 1 round * 10 minutes.
	if last.EtaMenit != 10 {
		t.Fatalf("fourth ticket eta = %d, want 10", last.EtaMenit)
	}
}
