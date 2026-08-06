package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type InstansiHandler struct {
	instansiService *service.InstansiService
}

func NewInstansiHandler(s *service.InstansiService) *InstansiHandler {
	return &InstansiHandler{instansiService: s}
}

func (h *InstansiHandler) List(c *gin.Context) {
	result, err := h.instansiService.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list instansi", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Instansi retrieved successfully", result)
}

func (h *InstansiHandler) GetByID(c *gin.Context) {
	result, err := h.instansiService.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrInstansiNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get instansi", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Instansi retrieved successfully", result)
}

func (h *InstansiHandler) Layanan(c *gin.Context) {
	result, err := h.instansiService.Layanan(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrInstansiNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get layanan", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Layanan retrieved successfully", result)
}
