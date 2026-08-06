package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type AntrianHandler struct {
	antrianService *service.AntrianService
}

func NewAntrianHandler(s *service.AntrianService) *AntrianHandler {
	return &AntrianHandler{antrianService: s}
}

func (h *AntrianHandler) WalkIn(c *gin.Context) {
	var req dto.WalkInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err.Error())
		return
	}

	result, err := h.antrianService.WalkIn(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrLayananNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi atau layanan tidak ditemukan", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to register walk-in", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Registered", result)
}

func (h *AntrianHandler) Queue(c *gin.Context) {
	var query dto.QueueQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	page, limit := query.Page, query.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	result, total, err := h.antrianService.Queue(c.Request.Context(), query.LayananID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to read queue", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Queue retrieved successfully", result, page, limit, total)
}
