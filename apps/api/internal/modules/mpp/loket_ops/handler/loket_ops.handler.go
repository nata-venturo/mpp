package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type LoketOpsHandler struct {
	opsService *service.LoketOpsService
}

func NewLoketOpsHandler(s *service.LoketOpsService) *LoketOpsHandler {
	return &LoketOpsHandler{opsService: s}
}

func (h *LoketOpsHandler) Session(c *gin.Context) {
	var req dto.SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err.Error())
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	result, err := h.opsService.Session(c.Request.Context(), c.Param("id"), userID, req.Action)
	if err != nil {
		writeError(c, err, "Gagal mengubah sesi loket")
		return
	}

	response.Success(c, http.StatusOK, "Loket session updated", result)
}

func (h *LoketOpsHandler) CallNext(c *gin.Context) {
	var req dto.CallNextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err.Error())
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	result, err := h.opsService.CallNext(c.Request.Context(), req.LoketID, userID)
	if err != nil {
		writeError(c, err, "Gagal memanggil antrian")
		return
	}
	if result == nil {
		// An empty stream is a normal outcome, not an error — the panel
		// shows "tidak ada antrian" rather than a failure toast.
		response.Success(c, http.StatusOK, "Tidak ada antrian menunggu", nil)
		return
	}

	response.Success(c, http.StatusOK, "Called", result)
}

func (h *LoketOpsHandler) Recall(c *gin.Context) { h.act(c, h.opsService.Recall, "Recalled") }
func (h *LoketOpsHandler) Start(c *gin.Context)  { h.act(c, h.opsService.Start, "Serving") }
func (h *LoketOpsHandler) Skip(c *gin.Context)   { h.act(c, h.opsService.Skip, "Skipped") }
func (h *LoketOpsHandler) Done(c *gin.Context)   { h.act(c, h.opsService.Done, "Done") }

// act is the shared shape of every per-item action: resolve the caller,
// run the transition, map sentinels to status codes.
func (h *LoketOpsHandler) act(
	c *gin.Context,
	fn func(ctx context.Context, antrianID, userID string) (*dto.AntrianActionResponse, error),
	message string,
) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
		return
	}

	result, err := fn(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeError(c, err, "Aksi antrian gagal")
		return
	}

	response.Success(c, http.StatusOK, message, result)
}

func writeError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrLoketNotFound):
		response.Error(c, http.StatusNotFound, "Loket tidak ditemukan", "")
	case errors.Is(err, service.ErrAntrianNotFound):
		response.Error(c, http.StatusNotFound, "Antrian tidak ditemukan", "")
	case errors.Is(err, service.ErrNotYourLoket):
		response.Error(c, http.StatusForbidden, "Loket ini dipegang petugas lain", "")
	case errors.Is(err, service.ErrNoTransition):
		response.Error(c, http.StatusConflict, "Status antrian tidak memungkinkan aksi ini", "illegal transition")
	default:
		response.Error(c, http.StatusInternalServerError, fallback, err.Error())
	}
}
