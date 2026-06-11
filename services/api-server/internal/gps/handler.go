package gps

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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

func (h *Handler) History(c *gin.Context) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}
	if claims.Role != "super_admin" && claims.Role != "dispatcher" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "GPS history requires super admin or dispatcher role", nil)
		return
	}

	to, err := queryTime(c, "to", time.Now().UTC())
	if err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "to must use RFC3339 format", nil)
		return
	}
	from, err := queryTime(c, "from", to.Add(-24*time.Hour))
	if err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "from must use RFC3339 format", nil)
		return
	}
	if !from.Before(to) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "from must be earlier than to", nil)
		return
	}

	limit := queryLimit(c.Query("limit"))
	result, err := h.service.History(c.Request.Context(), c.Param("id"), from, to, limit)
	if errors.Is(err, ErrUserNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load GPS history", nil)
		return
	}
	c.JSON(http.StatusOK, result)
}

func queryTime(c *gin.Context, name string, fallback time.Time) (time.Time, error) {
	value := c.Query(name)
	if value == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339, value)
}

func queryLimit(value string) int {
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
