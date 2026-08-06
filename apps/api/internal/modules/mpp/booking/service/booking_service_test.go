package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotarepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
	// Belongs to Imigrasi, not Dukcapil — used for the cross-agency case.
	foreignLayananID = "a3000000-0000-0000-0000-000000000011"
)

var jakarta = mustLoadJakarta()

func mustLoadJakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

func newService(pool *pgxpool.Pool) *service.BookingService {
	return service.NewBookingService(
		bookingrepo.NewBookingRepository(pool, companyID),
		instansirepo.NewInstansiRepository(pool, companyID),
		kuotarepo.NewKuotaRepository(pool, companyID),
		configsvc.NewConfigService(configrepo.NewConfigRepository(pool), jakarta),
		jakarta,
	)
}

func todayLocal() string {
	return time.Now().In(jakarta).Format(service.DateLayout)
}

// setQuota narrows today's agency-wide quota and restores it afterwards.
func setQuota(t *testing.T, pool *pgxpool.Pool, seats int) {
	t.Helper()

	ctx := context.Background()
	day, _ := time.ParseInLocation(service.DateLayout, todayLocal(), jakarta)

	var original int
	if err := pool.QueryRow(ctx, `
		SELECT kuota FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, day).Scan(&original); err != nil {
		t.Fatalf("read seeded quota: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE mpp.kuota_booking SET kuota = $3, terpakai = 0
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, day, seats); err != nil {
		t.Fatalf("set quota: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			UPDATE mpp.kuota_booking SET kuota = $3, terpakai = 0
			WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
			instansiID, day, original)
	})
}

func request(name, phone string) *dto.CreateBookingRequest {
	return &dto.CreateBookingRequest{
		InstansiID: instansiID,
		LayananID:  layananID,
		Tanggal:    todayLocal(),
		Pemohon:    dto.PemohonRequest{Name: name, Phone: phone},
	}
}

// ----------------------------------------------------------------------
// Pure logic: the QR validity window.

func TestExpiryFor(t *testing.T) {
	tanggal := time.Date(2026, 8, 6, 0, 0, 0, 0, jakarta)

	tests := []struct {
		name   string
		window configsvc.CheckinWindow
		want   time.Time
	}{
		{
			name:   "default end of local day",
			window: configsvc.CheckinWindow{},
			// 2026-08-06 23:59:59 WIB == 16:59:59 UTC
			want: time.Date(2026, 8, 6, 16, 59, 59, 0, time.UTC),
		},
		{
			name:   "explicit end_of_day",
			window: configsvc.CheckinWindow{Mode: "end_of_day"},
			want:   time.Date(2026, 8, 6, 16, 59, 59, 0, time.UTC),
		},
		{
			name:   "fixed hours from local midnight",
			window: configsvc.CheckinWindow{Mode: "fixed_hours", Hours: 12},
			// 2026-08-06 12:00 WIB == 05:00 UTC
			want: time.Date(2026, 8, 6, 5, 0, 0, 0, time.UTC),
		},
		{
			name:   "fixed hours with a nonsense value falls back",
			window: configsvc.CheckinWindow{Mode: "fixed_hours", Hours: 0},
			want:   time.Date(2026, 8, 6, 16, 59, 59, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.ExpiryFor(tanggal, tc.window, jakarta)
			if !got.Equal(tc.want) {
				t.Fatalf("ExpiryFor = %s, want %s", got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
			if got.Location() != time.UTC {
				t.Errorf("expiry must be UTC, got %s", got.Location())
			}
		})
	}
}

// ----------------------------------------------------------------------
// Create.

func TestCreateReturnsBookedWithUniqueTokens(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	setQuota(t, pool, 10)

	svc := newService(pool)
	ctx := context.Background()

	first, err := svc.Create(ctx, request("Ibu Sari", "628123000001"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if first.Status != "BOOKED" {
		t.Errorf("status = %q, want BOOKED", first.Status)
	}
	if first.Channel != "WEB" {
		t.Errorf("channel = %q, want WEB", first.Channel)
	}
	if first.QRToken == nil || *first.QRToken == "" {
		t.Fatal("want a qr_token on create")
	}
	if first.QRExpiresAt == nil {
		t.Fatal("want a qr_expires_at on create")
	}
	if _, err := time.Parse(time.RFC3339, *first.QRExpiresAt); err != nil {
		t.Errorf("qr_expires_at %q is not RFC3339: %v", *first.QRExpiresAt, err)
	}

	second, err := svc.Create(ctx, request("Pak Budi", "628123000002"))
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if *first.QRToken == *second.QRToken {
		t.Fatal("two bookings share a qr_token")
	}
}

func TestCreateRejectsCrossAgencyService(t *testing.T) {
	pool := testutil.RequireDB(t)
	svc := newService(pool)

	req := request("Salah Instansi", "628123000003")
	req.LayananID = foreignLayananID

	if _, err := svc.Create(context.Background(), req); !errors.Is(err, service.ErrLayananNotFound) {
		t.Fatalf("Create = %v, want ErrLayananNotFound", err)
	}
}

func TestCreateOnFullDayIsQuotaFull(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	setQuota(t, pool, 1)

	svc := newService(pool)
	ctx := context.Background()

	if _, err := svc.Create(ctx, request("Yang Pertama", "628123000010")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := svc.Create(ctx, request("Yang Kedua", "628123000011")); !errors.Is(err, service.ErrQuotaFull) {
		t.Fatalf("second Create = %v, want ErrQuotaFull", err)
	}
}

// TestCreateDoesNotOverbook is the end-to-end NFR-DATA-02 proof: the
// whole service path (validate → consume → insert → commit) under
// concurrency must still hand out exactly one seat.
func TestCreateDoesNotOverbook(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	setQuota(t, pool, 1)

	svc := newService(pool)
	ctx := context.Background()

	const n = 15
	var wg sync.WaitGroup
	results := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Create(ctx, request("Pendaftar", "62812399"+string(rune('0'+i%10))))
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	var ok, full int
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, service.ErrQuotaFull):
			full++
		default:
			t.Fatalf("unexpected Create error: %v", err)
		}
	}

	if ok != 1 || full != n-1 {
		t.Fatalf("created = %d, quota-full = %d; want 1 and %d", ok, full, n-1)
	}

	var bookings int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM mpp.booking`).Scan(&bookings); err != nil {
		t.Fatalf("count bookings: %v", err)
	}
	if bookings != 1 {
		t.Fatalf("%d booking rows persisted, want 1", bookings)
	}
}

func TestGetByIDReturnsTokenAndCatalogNames(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)
	setQuota(t, pool, 5)

	svc := newService(pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, request("Ibu Sari", "628123000020"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	detail, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if detail.QRToken == nil || *detail.QRToken != *created.QRToken {
		t.Errorf("detail token = %v, want %v", detail.QRToken, created.QRToken)
	}
	if detail.Instansi.Prefix != "A" {
		t.Errorf("instansi prefix = %q, want A", detail.Instansi.Prefix)
	}
	if detail.Layanan.Name == "" {
		t.Error("want the service name on the detail response")
	}
	if detail.PemohonName != "Ibu Sari" {
		t.Errorf("pemohon = %q, want 'Ibu Sari'", detail.PemohonName)
	}
}

func TestGetByIDUnknownIsNotFound(t *testing.T) {
	pool := testutil.RequireDB(t)
	svc := newService(pool)

	_, err := svc.GetByID(context.Background(), "a7000000-0000-0000-0000-0000000009ff")
	if !errors.Is(err, service.ErrBookingNotFound) {
		t.Fatalf("GetByID = %v, want ErrBookingNotFound", err)
	}
}
