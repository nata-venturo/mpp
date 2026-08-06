package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type CheckInHandler struct {
	checkInService *service.CheckInService
}

func NewCheckInHandler(s *service.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkInService: s}
}

func (h *CheckInHandler) CheckIn(c *gin.Context) {
	var req dto.CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Token tidak valid", err.Error())
		return
	}

	result, err := h.checkInService.CheckIn(c.Request.Context(), req.Token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTokenNotFound):
			response.Error(c, http.StatusNotFound, "QR tidak dikenali", "")
		case errors.Is(err, service.ErrTokenUsed):
			response.Error(c, http.StatusConflict, "QR sudah dipakai", "token already used")
		case errors.Is(err, service.ErrTokenExpired):
			// 410 Gone: the token existed and is permanently past its window.
			response.Error(c, http.StatusGone, "QR sudah kedaluwarsa atau bukan untuk hari ini", "token expired")
		default:
			response.Error(c, http.StatusInternalServerError, "Check-in gagal", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Checked in", result)
}
