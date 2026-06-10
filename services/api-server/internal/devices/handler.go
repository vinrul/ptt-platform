package devices

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/apiutil"
	"ptt-fleet/services/api-server/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	if !requireOperator(c) {
		return
	}
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to list devices", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Get(c *gin.Context) {
	if !requireOperator(c) {
		return
	}
	device, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Device not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load device", nil)
		return
	}
	c.JSON(http.StatusOK, device)
}

func requireOperator(c *gin.Context) bool {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return false
	}
	if claims.Role == "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires an operator role", nil)
		return false
	}
	return true
}

func (h *Handler) UpdatePushToken(c *gin.Context) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}

	var req struct {
		PushToken string `json:"pushToken" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "pushToken is required", nil)
		return
	}

	deviceID := c.Param("id")

	device, err := h.service.Get(c.Request.Context(), deviceID)
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Device not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load device", nil)
		return
	}

	if claims.Role == "field_user" && device.UserID != claims.Subject {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "You cannot update another user's device", nil)
		return
	}

	err = h.service.UpdatePushToken(c.Request.Context(), deviceID, req.PushToken)
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to update push token", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
