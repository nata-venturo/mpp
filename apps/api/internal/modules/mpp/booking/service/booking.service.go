package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotarepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	"github.com/ndollem/mpp/apps/api/pkg/crypto"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// DateLayout is the wire format of a booking day (a local calendar date).
const DateLayout = "2006-01-02"

// tokenBytes is 24 bytes = 192 bits of entropy, comfortably above the
// 128-bit floor the security doc sets for an unguessable handle.
const tokenBytes = 24

var (
	// ErrLayananNotFound covers "no such agency", "no such service" and
	// "that service belongs to a different agency" — all 404 to a caller
	// who should not be able to tell them apart.
	ErrLayananNotFound = errors.New("layanan not found")
	// ErrQuotaFull is re-exported so the handler depends on one package.
	ErrQuotaFull = kuotarepo.ErrQuotaFull
	// ErrBookingNotFound backs GET /booking/{id}.
	ErrBookingNotFound = errors.New("booking not found")
)

type BookingService struct {
	repo         *repository.BookingRepository
	instansiRepo *instansirepo.InstansiRepository
	kuotaRepo    *kuotarepo.KuotaRepository
	cfg          *configsvc.ConfigService
	loc          *time.Location
}

func NewBookingService(
	repo *repository.BookingRepository,
	instansiRepo *instansirepo.InstansiRepository,
	kuotaRepo *kuotarepo.KuotaRepository,
	cfg *configsvc.ConfigService,
	loc *time.Location,
) *BookingService {
	return &BookingService{
		repo:         repo,
		instansiRepo: instansiRepo,
		kuotaRepo:    kuotaRepo,
		cfg:          cfg,
		loc:          loc,
	}
}

// Create registers a booking. Quota is consumed inside the same
// transaction as the insert, so a seat is never charged for a booking
// that failed to persist — and never handed to two citizens at once.
func (s *BookingService) Create(ctx context.Context, req *dto.CreateBookingRequest) (*dto.BookingResponse, error) {
	tanggal, err := time.ParseInLocation(DateLayout, req.Tanggal, s.loc)
	if err != nil {
		return nil, err
	}

	layanan, _, err := s.instansiRepo.FindActiveLayanan(ctx, req.InstansiID, req.LayananID)
	if err != nil {
		return nil, err
	}
	if layanan == nil {
		return nil, ErrLayananNotFound
	}

	expiresAt, err := s.IssueTokenExpiry(ctx, req.InstansiID, tanggal)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// Reserve the seat FIRST: the cheapest way to reject an overbooking is
	// before any applicant row is written.
	if err := s.kuotaRepo.Consume(ctx, tx, req.InstansiID, &req.LayananID, tanggal); err != nil {
		return nil, err
	}

	pemohon := &domain.Pemohon{
		Name:  req.Pemohon.Name,
		Phone: &req.Pemohon.Phone,
		Email: req.Pemohon.Email,
	}
	if req.Pemohon.NIK != nil && *req.Pemohon.NIK != "" {
		sum := sha256.Sum256([]byte(*req.Pemohon.NIK))
		hashed := hex.EncodeToString(sum[:])
		pemohon.NIKHash = &hashed
	}

	pemohonID, err := s.repo.UpsertPemohon(ctx, tx, pemohon)
	if err != nil {
		return nil, err
	}

	booking := &domain.Booking{
		PemohonID:      pemohonID,
		InstansiID:     req.InstansiID,
		JenisLayananID: req.LayananID,
		Tanggal:        tanggal,
		Channel:        "WEB",
		QRExpiresAt:    &expiresAt,
	}

	// One retry covers the only realistic failure of a 192-bit token: a
	// collision that will never happen, and a bug that would show up here
	// twice rather than corrupting a row.
	for attempt := 0; attempt < 2; attempt++ {
		token, err := crypto.RandomToken(tokenBytes)
		if err != nil {
			return nil, err
		}
		booking.QRToken = &token

		err = s.repo.Create(ctx, tx, booking)
		if err == nil {
			break
		}
		if !errors.Is(err, repository.ErrDuplicateToken) {
			return nil, err
		}
		logger.Warn("QR token collision — regenerating")
		if attempt == 1 {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &dto.BookingResponse{
		ID:          booking.ID,
		Status:      booking.Status,
		InstansiID:  booking.InstansiID,
		LayananID:   booking.JenisLayananID,
		Tanggal:     booking.Tanggal.Format(DateLayout),
		Channel:     booking.Channel,
		QRToken:     booking.QRToken,
		QRExpiresAt: formatTime(booking.QRExpiresAt),
		CreatedAt:   booking.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// IssueTokenExpiry computes when a booking's QR stops working, honoring
// the agency's `checkin_window` config. Pure apart from the config read,
// so the window arithmetic is unit-testable on its own.
func (s *BookingService) IssueTokenExpiry(ctx context.Context, instansiID string, tanggal time.Time) (time.Time, error) {
	window := s.cfg.CheckinWindow(ctx, s.repo.DB(), &instansiID)
	return ExpiryFor(tanggal, window, s.loc), nil
}

// ExpiryFor is the pure window rule: "end_of_day" (default) means the QR
// dies at 23:59:59 local on the booking date; "fixed_hours" counts from
// that date's local midnight. The result is always UTC.
func ExpiryFor(tanggal time.Time, window configsvc.CheckinWindow, loc *time.Location) time.Time {
	startOfDay := time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 0, 0, 0, 0, loc)

	if window.Mode == "fixed_hours" && window.Hours > 0 {
		return startOfDay.Add(time.Duration(window.Hours) * time.Hour).UTC()
	}

	return startOfDay.Add(24*time.Hour - time.Second).UTC()
}

// GetByID backs the confirm screen. It is public on purpose: the id is
// the citizen's own handle and carries no PII beyond what they typed.
func (s *BookingService) GetByID(ctx context.Context, id string) (*dto.BookingDetailResponse, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrBookingNotFound
	}

	return ToDetailResponse(d), nil
}

// ToDetailResponse maps a joined booking row to its wire shape.
func ToDetailResponse(d *domain.Detail) *dto.BookingDetailResponse {
	return &dto.BookingDetailResponse{
		ID:          d.ID,
		Status:      d.Status,
		Tanggal:     d.Tanggal.Format(DateLayout),
		Channel:     d.Channel,
		QRToken:     d.QRToken,
		QRExpiresAt: formatTime(d.QRExpiresAt),
		CheckedInAt: formatTime(d.CheckedInAt),
		PemohonName: d.PemohonName,
		Instansi:    dto.CatalogRef{ID: d.InstansiID, Name: d.InstansiName, Prefix: d.InstansiPrefix},
		Layanan:     dto.CatalogRef{ID: d.JenisLayananID, Name: d.LayananName},
		CreatedAt:   d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
