package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type KuotaHandler struct {
	kuotaService *service.KuotaService
}

func NewKuotaHandler(s *service.KuotaService) *KuotaHandler {
	return &KuotaHandler{kuotaService: s}
}

func (h *KuotaHandler) Availability(c *gin.Context) {
	var query dto.AvailabilityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	date, err := h.kuotaService.ParseDate(query.Date)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid date", err.Error())
		return
	}

	var layananID *string
	if query.LayananID != "" {
		layananID = &query.LayananID
	}

	result, err := h.kuotaService.Availability(c.Request.Context(), query.InstansiID, layananID, date)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to read availability", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Availability retrieved successfully", result)
}
