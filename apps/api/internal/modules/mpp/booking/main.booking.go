package booking

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotarepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
)

type Module struct {
	Handler    *handler.BookingHandler
	Service    *service.BookingService
	Repository *repository.BookingRepository

	limiter *middleware.RateLimiter
}

// Initialize builds repo → service → handler. The instansi and kuota
// repositories are injected rather than rebuilt so tenancy and quota
// precedence live in exactly one implementation each.
func Initialize(
	db *pgxpool.Pool,
	companyID string,
	loc *time.Location,
	instansiRepo *instansirepo.InstansiRepository,
	kuotaRepo *kuotarepo.KuotaRepository,
	cfg *configsvc.ConfigService,
) *Module {
	repo := repository.NewBookingRepository(db, companyID)
	svc := service.NewBookingService(repo, instansiRepo, kuotaRepo, cfg, loc)

	return &Module{
		Handler:    handler.NewBookingHandler(svc),
		Service:    svc,
		Repository: repo,
		// NFR-SEC-06: public registration is throttled per source IP.
		//
		// ponytail: this limiter is per-process (in-memory). Behind more
		// than one API instance the effective ceiling multiplies by the
		// instance count — move the counter to Redis INCR when that
		// happens.
		limiter: middleware.NewRateLimiter(20, time.Minute),
	}
}

// SetupRoutes registers the public registration endpoints. There is no
// JWTAuth here by design: quota and booking state are the authority for
// who may register, not RBAC.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.POST("/booking",
		middleware.IPBasedRateLimiter(m.limiter, 5*time.Minute),
		m.Handler.Create)
	rg.GET("/booking/:id", m.Handler.GetByID)
}
