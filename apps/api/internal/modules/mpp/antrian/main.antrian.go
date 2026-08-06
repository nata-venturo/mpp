package antrian

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
)

type Module struct {
	Handler    *handler.AntrianHandler
	Service    *service.AntrianService
	Repository *repository.AntrianRepository
}

// Initialize builds the queue-numbering module. Redis is required: it is
// the authoritative counter, with the DB unique index as the backstop.
func Initialize(
	db *pgxpool.Pool,
	redis *goredis.Client,
	companyID string,
	loc *time.Location,
	instansiRepo *instansirepo.InstansiRepository,
	loketRepo *loketrepo.LoketRepository,
	bookingRepo *bookingrepo.BookingRepository,
	cfg *configsvc.ConfigService,
) *Module {
	repo := repository.NewAntrianRepository(db, redis)
	svc := service.NewAntrianService(repo, instansiRepo, loketRepo, bookingRepo, cfg, companyID, loc)

	return &Module{
		Handler:    handler.NewAntrianHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the staff queue read and the device walk-in.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/queue",
		middleware.JWTAuth(), middleware.RequirePermission("mpp.queue:read"),
		m.Handler.Queue)

	// Walk-in comes from the kiosk (X-API-Key) or a front-desk operator —
	// JWTAuth accepts both; the key carries mpp.booking:create.
	rg.POST("/walkin",
		middleware.JWTAuth(), middleware.RequirePermission("mpp.booking:create"),
		m.Handler.WalkIn)
}
