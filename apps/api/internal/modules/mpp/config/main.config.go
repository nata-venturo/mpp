package config

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
)

// Module exposes the system_config reader to the other MPP modules. It
// registers no routes — admin CRUD over configuration is a later phase.
type Module struct {
	Service    *service.ConfigService
	Repository *repository.ConfigRepository
}

func Initialize(db *pgxpool.Pool, loc *time.Location) *Module {
	repo := repository.NewConfigRepository(db)

	return &Module{
		Service:    service.NewConfigService(repo, loc),
		Repository: repo,
	}
}
