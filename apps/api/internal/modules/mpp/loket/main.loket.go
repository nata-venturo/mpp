package loket

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/service"
)

type Module struct {
	Handler    *handler.LoketHandler
	Service    *service.LoketService
	Repository *repository.LoketRepository
}

func Initialize(db *pgxpool.Pool, companyID string) *Module {
	repo := repository.NewLoketRepository(db, companyID)
	svc := service.NewLoketService(repo)

	return &Module{
		Handler:    handler.NewLoketHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers staff-only loket reads.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/loket", middleware.JWTAuth(), middleware.RequirePermission("mpp.loket:read"), m.Handler.List)
}
