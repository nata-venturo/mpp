package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type DisplayHandler struct {
	displayService *service.DisplayService
}

func NewDisplayHandler(s *service.DisplayService) *DisplayHandler {
	return &DisplayHandler{displayService: s}
}

func (h *DisplayHandler) Snapshot(c *gin.Context) {
	var query dto.DisplayQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	result, err := h.displayService.Snapshot(c.Request.Context(), query.InstansiID)
	if err != nil {
		if errors.Is(err, service.ErrInstansiNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi tidak ditemukan", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to build display snapshot", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Display snapshot retrieved successfully", result)
}
