package service

import (
	"context"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
)

type LoketService struct {
	repo *repository.LoketRepository
}

func NewLoketService(repo *repository.LoketRepository) *LoketService {
	return &LoketService{repo: repo}
}

func (s *LoketService) ListByInstansi(ctx context.Context, instansiID string) ([]dto.LoketResponse, error) {
	list, err := s.repo.FindByInstansi(ctx, instansiID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.LoketResponse, 0, len(list))
	for i := range list {
		out = append(out, dto.LoketResponse{
			ID:         list[i].ID,
			InstansiID: list[i].InstansiID,
			Code:       list[i].Code,
			Name:       list[i].DisplayName(),
			Status:     list[i].Status,
			IsActive:   list[i].IsActive,
		})
	}

	return out, nil
}
