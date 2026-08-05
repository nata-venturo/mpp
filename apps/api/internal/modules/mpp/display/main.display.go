package display

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/service"
)

type Module struct {
	Handler    *handler.DisplayHandler
	Service    *service.DisplayService
	Repository *repository.DisplayRepository
}

func Initialize(
	db *pgxpool.Pool,
	instansiRepo *instansirepo.InstansiRepository,
	antrian *antriansvc.AntrianService,
	cfg *configsvc.ConfigService,
) *Module {
	repo := repository.NewDisplayRepository(db)
	svc := service.NewDisplayService(db, repo, instansiRepo, antrian, cfg)

	return &Module{
		Handler:    handler.NewDisplayHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the TV snapshot. JWTAuth accepts the TV's
// X-API-Key, whose scope carries mpp.display:read.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/display",
		middleware.JWTAuth(), middleware.RequirePermission("mpp.display:read"),
		m.Handler.Snapshot)
}
