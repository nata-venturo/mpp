package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	bookingdto "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	bookingsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	configrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotarepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/service"
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

type harness struct {
	pool    *pgxpool.Pool
	booking *bookingsvc.BookingService
	checkin *service.CheckInService
}

func setup(t *testing.T) *harness {
	t.Helper()

	pool := testutil.RequireDB(t)
	rdb := testutil.RequireRedis(t)
	testutil.TruncateMPP(t, pool)

	cfg := configsvc.NewConfigService(configrepo.NewConfigRepository(pool), jakarta)
	bookRepo := bookingrepo.NewBookingRepository(pool, companyID)
	instRepo := instansirepo.NewInstansiRepository(pool, companyID)

	antrian := antriansvc.NewAntrianService(
		antrianrepo.NewAntrianRepository(pool, rdb),
		instRepo,
		loketrepo.NewLoketRepository(pool, companyID),
		bookRepo,
		cfg, companyID, jakarta,
	)

	// Start every test from a cold counter so numbers are predictable.
	key := "mpp:counter:" + instansiID + ":" + antrian.OperatingDay().Format("20060102")
	if err := rdb.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("flush counter: %v", err)
	}

	return &harness{
		pool: pool,
		booking: bookingsvc.NewBookingService(
			bookRepo, instRepo, kuotarepo.NewKuotaRepository(pool, companyID), cfg, jakarta),
		checkin: service.NewCheckInService(bookRepo, antrian, jakarta),
	}
}

// book creates a booking for today and returns its QR token.
func (h *harness) book(t *testing.T, name, phone string) string {
	t.Helper()

	res, err := h.booking.Create(context.Background(), &bookingdto.CreateBookingRequest{
		InstansiID: instansiID,
		LayananID:  layananID,
		Tanggal:    time.Now().In(jakarta).Format(bookingsvc.DateLayout),
		Pemohon:    bookingdto.PemohonRequest{Name: name, Phone: phone},
	})
	if err != nil {
		t.Fatalf("Create booking: %v", err)
	}
	if res.QRToken == nil {
		t.Fatal("booking has no qr_token")
	}

	return *res.QRToken
}

func TestCheckInIssuesANumber(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	token := h.book(t, "Ibu Sari", "628140000001")

	res, err := h.checkin.CheckIn(ctx, token)
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if res.Status != "CHECKED_IN" {
		t.Errorf("status = %q, want CHECKED_IN", res.Status)
	}
	if res.Nomor != "A-001" || res.NomorSeq != 1 {
		t.Errorf("nomor = %s (seq %d), want A-001 (1)", res.Nomor, res.NomorSeq)
	}
	if res.QueueStatus != "WAITING" {
		t.Errorf("queue_status = %q, want WAITING", res.QueueStatus)
	}
	if res.Instansi.Prefix != "A" || res.Layanan.Name == "" {
		t.Errorf("catalog labels missing: %+v / %+v", res.Instansi, res.Layanan)
	}

	var status string
	if err := h.pool.QueryRow(ctx,
		`SELECT status::text FROM mpp.booking WHERE id = $1`, res.BookingID).Scan(&status); err != nil {
		t.Fatalf("read booking: %v", err)
	}
	if status != "CHECKED_IN" {
		t.Fatalf("persisted booking status = %q, want CHECKED_IN", status)
	}
}

func TestCheckInRejectsReuse(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	token := h.book(t, "Pak Budi", "628140000002")

	if _, err := h.checkin.CheckIn(ctx, token); err != nil {
		t.Fatalf("first CheckIn: %v", err)
	}
	if _, err := h.checkin.CheckIn(ctx, token); !errors.Is(err, service.ErrTokenUsed) {
		t.Fatalf("second CheckIn = %v, want ErrTokenUsed", err)
	}
}

func TestCheckInRejectsUnknownToken(t *testing.T) {
	h := setup(t)

	_, err := h.checkin.CheckIn(context.Background(), "tidak-pernah-diterbitkan")
	if !errors.Is(err, service.ErrTokenNotFound) {
		t.Fatalf("CheckIn = %v, want ErrTokenNotFound", err)
	}
}

func TestCheckInRejectsExpiredToken(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	token := h.book(t, "Telat", "628140000003")

	// Walk the clock past the end of the booking day without sleeping.
	h.checkin.SetClock(func() time.Time { return time.Now().Add(48 * time.Hour) })

	if _, err := h.checkin.CheckIn(ctx, token); !errors.Is(err, service.ErrTokenExpired) {
		t.Fatalf("CheckIn = %v, want ErrTokenExpired", err)
	}
}

func TestCheckInRejectsWrongDay(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	token := h.book(t, "Salah Hari", "628140000004")

	// Push the QR window far out so only the day check can fire.
	if _, err := h.pool.Exec(ctx,
		`UPDATE mpp.booking SET qr_expires_at = NOW() + INTERVAL '30 day' WHERE qr_token = $1`,
		token); err != nil {
		t.Fatalf("extend expiry: %v", err)
	}

	h.checkin.SetClock(func() time.Time { return time.Now().Add(72 * time.Hour) })

	if _, err := h.checkin.CheckIn(ctx, token); !errors.Is(err, service.ErrTokenExpired) {
		t.Fatalf("CheckIn on the wrong day = %v, want ErrTokenExpired", err)
	}
}

// TestCheckInDoubleScanRace proves the status='BOOKED' guard: two kiosks
// (or one impatient scanner) hitting the same token concurrently must
// produce exactly one check-in and one number.
func TestCheckInDoubleScanRace(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	token := h.book(t, "Dobel Scan", "628140000005")

	const n = 6
	var wg sync.WaitGroup
	results := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.checkin.CheckIn(ctx, token)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	var ok, used int
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, service.ErrTokenUsed):
			used++
		default:
			t.Fatalf("unexpected CheckIn error: %v", err)
		}
	}

	if ok != 1 || used != n-1 {
		t.Fatalf("check-ins = %d, rejected = %d; want 1 and %d", ok, used, n-1)
	}

	var tickets int
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM mpp.antrian`).Scan(&tickets); err != nil {
		t.Fatalf("count antrian: %v", err)
	}
	if tickets != 1 {
		t.Fatalf("%d queue tickets issued for one token, want 1", tickets)
	}
}
