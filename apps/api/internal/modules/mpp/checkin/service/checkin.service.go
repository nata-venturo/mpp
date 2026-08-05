package service

import (
	"context"
	"errors"
	"time"

	antriandomain "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/domain"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/dto"
)

var (
	// ErrTokenNotFound — no booking carries this token (404).
	ErrTokenNotFound = errors.New("checkin token not found")
	// ErrTokenUsed — the booking is no longer BOOKED, so the token has
	// already been spent (or the booking was cancelled) (409).
	ErrTokenUsed = errors.New("checkin token already used")
	// ErrTokenExpired — past its window, or presented on the wrong day (410).
	ErrTokenExpired = errors.New("checkin token expired")
)

type CheckInService struct {
	bookingRepo *bookingrepo.BookingRepository
	antrian     *antriansvc.AntrianService
	loc         *time.Location
	now         func() time.Time
}

func NewCheckInService(
	bookingRepo *bookingrepo.BookingRepository,
	antrian *antriansvc.AntrianService,
	loc *time.Location,
) *CheckInService {
	return &CheckInService{
		bookingRepo: bookingRepo,
		antrian:     antrian,
		loc:         loc,
		now:         time.Now,
	}
}

// SetClock overrides the time source. Tests use it to walk past a QR
// window without sleeping; production never calls it.
func (s *CheckInService) SetClock(now func() time.Time) {
	s.now = now
}

// CheckIn validates a scanned token and, in one transaction, flips the
// booking to CHECKED_IN and issues the queue number.
//
// The single transaction is the point: a citizen must never end up
// checked in without a number, or holding a number for a booking that
// was never marked used.
func (s *CheckInService) CheckIn(ctx context.Context, token string) (*dto.CheckInResponse, error) {
	booking, err := s.bookingRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, ErrTokenNotFound
	}

	if booking.Status != "BOOKED" {
		return nil, ErrTokenUsed
	}

	now := s.now()
	if booking.QRExpiresAt != nil && now.After(*booking.QRExpiresAt) {
		return nil, ErrTokenExpired
	}
	if !s.isToday(booking.Tanggal, now) {
		// A valid token for another day is not "used" — it is simply not
		// valid here and now. Same rejection, different reason.
		return nil, ErrTokenExpired
	}

	tx, err := s.bookingRepo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// The status='BOOKED' guard inside MarkCheckedIn is what makes a
	// concurrent double-scan a 0-row result rather than two check-ins.
	updated, err := s.bookingRepo.MarkCheckedIn(ctx, tx, booking.ID)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, ErrTokenUsed
	}

	item, err := s.antrian.Enqueue(ctx, tx, antriansvc.EnqueueInput{
		BookingID:  &booking.ID,
		PemohonID:  booking.PemohonID,
		InstansiID: booking.InstansiID,
		LayananID:  booking.JenisLayananID,
		Source:     antriandomain.SourceBooking,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &dto.CheckInResponse{
		BookingID:   booking.ID,
		Status:      "CHECKED_IN",
		CheckedInAt: now.UTC().Format(time.RFC3339),
		Instansi: dto.InstansiRef{
			ID:     booking.InstansiID,
			Name:   booking.InstansiName,
			Prefix: booking.InstansiPrefix,
		},
		Layanan: dto.LayananRef{
			ID:   booking.JenisLayananID,
			Name: booking.LayananName,
		},
		AntrianID:   item.ID,
		Nomor:       item.Nomor,
		NomorSeq:    item.NomorSeq,
		QueueStatus: item.Status,
		EtaMenit:    s.antrian.EstimateWait(ctx, item),
		PemohonName: booking.PemohonName,
	}, nil
}

// isToday compares the booking's calendar date with the current local
// operating day — a QR is only good on the day it was booked for.
func (s *CheckInService) isToday(tanggal, now time.Time) bool {
	local := now.In(s.loc)
	return tanggal.Year() == local.Year() &&
		tanggal.Month() == local.Month() &&
		tanggal.Day() == local.Day()
}
