package loket_ops

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/ws"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"
)

type Module struct {
	Handler    *handler.LoketOpsHandler
	Service    *service.LoketOpsService
	Repository *repository.LoketOpsRepository
}

func Initialize(
	db *pgxpool.Pool,
	companyID string,
	loketRepo *loketrepo.LoketRepository,
	antrianRepo *antrianrepo.AntrianRepository,
	instansiRepo *instansirepo.InstansiRepository,
	antrian *antriansvc.AntrianService,
	cfg *configsvc.ConfigService,
	hub *ws.Hub,
) *Module {
	repo := repository.NewLoketOpsRepository(db)
	svc := service.NewLoketOpsService(repo, loketRepo, antrianRepo, instansiRepo, antrian, cfg, hub, companyID)

	return &Module{
		Handler:    handler.NewLoketOpsHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the operator actions.
//
// Every queue transition is guarded by mpp.antrian:update — the CRUD
// verb the permission-level vocabulary actually grants (see
// docs/06-security/rbac-matrix.md). Do not invent call/skip verbs.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(middleware.JWTAuth())
	{
		g.POST("/loket/:id/session", middleware.RequirePermission("mpp.queue:update"), m.Handler.Session)
		g.POST("/queue/next", middleware.RequirePermission("mpp.antrian:update"), m.Handler.CallNext)
		g.POST("/antrian/:id/recall", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Recall)
		g.POST("/antrian/:id/start", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Start)
		g.POST("/antrian/:id/skip", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Skip)
		g.POST("/antrian/:id/done", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Done)
	}
}
