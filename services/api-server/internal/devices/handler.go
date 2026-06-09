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
