package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type LoketHandler struct {
	loketService *service.LoketService
}

func NewLoketHandler(s *service.LoketService) *LoketHandler {
	return &LoketHandler{loketService: s}
}

func (h *LoketHandler) List(c *gin.Context) {
	var query dto.LoketQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	result, err := h.loketService.ListByInstansi(c.Request.Context(), query.InstansiID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list loket", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Loket retrieved successfully", result)
}
