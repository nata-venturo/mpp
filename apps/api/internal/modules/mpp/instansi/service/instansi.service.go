package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
)

var ErrInstansiNotFound = errors.New("instansi not found")

type InstansiService struct {
	repo *repository.InstansiRepository
}

func NewInstansiService(repo *repository.InstansiRepository) *InstansiService {
	return &InstansiService{repo: repo}
}

func (s *InstansiService) List(ctx context.Context) ([]dto.InstansiResponse, error) {
	list, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]dto.InstansiResponse, 0, len(list))
	for i := range list {
		out = append(out, ToInstansiResponse(&list[i]))
	}

	return out, nil
}

func (s *InstansiService) GetByID(ctx context.Context, id string) (*dto.InstansiResponse, error) {
	i, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, ErrInstansiNotFound
	}

	resp := ToInstansiResponse(i)
	return &resp, nil
}

// Layanan lists an agency's services. A missing agency is a 404, not an
// empty list — the citizen picked something that does not exist.
func (s *InstansiService) Layanan(ctx context.Context, instansiID string) ([]dto.LayananResponse, error) {
	i, err := s.repo.FindByID(ctx, instansiID)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, ErrInstansiNotFound
	}

	list, err := s.repo.FindLayananByInstansi(ctx, instansiID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.LayananResponse, 0, len(list))
	for idx := range list {
		out = append(out, ToLayananResponse(&list[idx]))
	}

	return out, nil
}

// ToInstansiResponse maps an agency entity to its public payload. Exported
// because other MPP modules (booking, display) embed the same shape.
func ToInstansiResponse(i *domain.Instansi) dto.InstansiResponse {
	hours := json.RawMessage(i.OperatingHours)
	if len(hours) == 0 {
		hours = json.RawMessage(`{}`)
	}

	return dto.InstansiResponse{
		ID:             i.ID,
		Name:           i.Name,
		Slug:           i.Slug,
		Prefix:         i.Prefix,
		Description:    i.Description,
		LogoURL:        i.LogoURL,
		OperatingHours: hours,
		QueueMode:      i.QueueMode,
		IsActive:       i.IsActive,
	}
}

// ToLayananResponse maps a service entity plus its requirements.
func ToLayananResponse(l *domain.Layanan) dto.LayananResponse {
	syarat := make([]dto.SyaratDokumenResponse, 0, len(l.Syarat))
	for _, s := range l.Syarat {
		syarat = append(syarat, dto.SyaratDokumenResponse{
			ID:         s.ID,
			Name:       s.Name,
			IsRequired: s.IsRequired,
			Notes:      s.Notes,
			Sort:       s.Sort,
		})
	}

	return dto.LayananResponse{
		ID:                     l.ID,
		InstansiID:             l.InstansiID,
		Name:                   l.Name,
		Description:            l.Description,
		EstimasiDurasiMenit:    l.EstimasiDurasiMenit,
		RequiresFOVerification: l.RequiresFOVerification,
		SyaratDokumen:          syarat,
	}
}
