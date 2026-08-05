package instansi

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/service"
)

type Module struct {
	Handler    *handler.InstansiHandler
	Service    *service.InstansiService
	Repository *repository.InstansiRepository
}

// Initialize builds repo → service → handler for the catalog module.
func Initialize(db *pgxpool.Pool, companyID string) *Module {
	repo := repository.NewInstansiRepository(db, companyID)
	svc := service.NewInstansiService(repo)

	return &Module{
		Handler:    handler.NewInstansiHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the public catalog reads. Catalog data is public
// by design (citizens browse it before any auth), so no JWTAuth here.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/instansi", m.Handler.List)
	rg.GET("/instansi/:id", m.Handler.GetByID)
	rg.GET("/instansi/:id/layanan", m.Handler.Layanan)
}
