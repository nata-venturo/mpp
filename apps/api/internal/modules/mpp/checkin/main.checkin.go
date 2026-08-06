package checkin

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/service"
)

type Module struct {
	Handler *handler.CheckInHandler
	Service *service.CheckInService
}

// Initialize wires check-in on top of the booking repository (token
// lookup + status guard) and the antrian service (numbering), rather
// than duplicating either.
func Initialize(
	bookingRepo *bookingrepo.BookingRepository,
	antrian *antriansvc.AntrianService,
	loc *time.Location,
) *Module {
	svc := service.NewCheckInService(bookingRepo, antrian, loc)

	return &Module{
		Handler: handler.NewCheckInHandler(svc),
		Service: svc,
	}
}

// SetupRoutes registers the device check-in. JWTAuth accepts the kiosk's
// X-API-Key, whose scope carries mpp.checkin:create.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.POST("/checkin",
		middleware.JWTAuth(), middleware.RequirePermission("mpp.checkin:create"),
		m.Handler.CheckIn)
}
