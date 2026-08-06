package service

import (
	"context"
	"time"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
)

// DateLayout is the wire format for a booking day.
const DateLayout = "2006-01-02"

type KuotaService struct {
	repo *repository.KuotaRepository
	loc  *time.Location
}

func NewKuotaService(repo *repository.KuotaRepository, loc *time.Location) *KuotaService {
	return &KuotaService{repo: repo, loc: loc}
}

// ParseDate turns a YYYY-MM-DD string into the midnight instant of that
// local operating day. Quota rows are DATE columns, so only the calendar
// part survives — parsing in the operating zone keeps "today" honest for
// a server running in UTC.
func (s *KuotaService) ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation(DateLayout, value, s.loc)
}

// Availability answers "how many seats are left on this day". A day with
// no quota row configured is reported as 0/0/0 rather than 404 — the
// citizen asked a legitimate question and the answer is "none".
func (s *KuotaService) Availability(ctx context.Context, instansiID string, layananID *string, date time.Time) (*dto.AvailabilityResponse, error) {
	slot, err := s.repo.FindSlot(ctx, instansiID, layananID, date)
	if err != nil {
		return nil, err
	}

	resp := dto.AvailabilityResponse{Date: date.Format(DateLayout)}
	if slot != nil {
		resp.Kuota = slot.Kuota
		resp.Terpakai = slot.Terpakai
		resp.Remaining = slot.Remaining()
	}

	return &resp, nil
}
