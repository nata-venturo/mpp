package kuota

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/service"
)

type Module struct {
	Handler    *handler.KuotaHandler
	Service    *service.KuotaService
	Repository *repository.KuotaRepository
}

func Initialize(db *pgxpool.Pool, companyID string, loc *time.Location) *Module {
	repo := repository.NewKuotaRepository(db, companyID)
	svc := service.NewKuotaService(repo, loc)

	return &Module{
		Handler:    handler.NewKuotaHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the public availability read — a citizen must be
// able to see remaining seats before committing to anything.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/availability", m.Handler.Availability)
}
