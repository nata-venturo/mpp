package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type BookingHandler struct {
	bookingService *service.BookingService
}

func NewBookingHandler(s *service.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: s}
}

func (h *BookingHandler) Create(c *gin.Context) {
	var req dto.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err.Error())
		return
	}

	result, err := h.bookingService.Create(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLayananNotFound):
			response.Error(c, http.StatusNotFound, "Instansi atau layanan tidak ditemukan", "")
		case errors.Is(err, service.ErrQuotaFull):
			// 409, not 400: the request was valid, the world changed.
			response.Error(c, http.StatusConflict, "Kuota tanggal ini sudah penuh", "quota full")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create booking", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "Booking created", result)
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	result, err := h.bookingService.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrBookingNotFound) {
			response.Error(c, http.StatusNotFound, "Booking not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get booking", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Booking retrieved successfully", result)
}
