package audit

import (
	"net/http"
	"strconv"
	"strings"

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
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}
	if claims.Role == "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires an operator role", nil)
		return
	}

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 50)
	if pageSize > 100 {
		pageSize = 100
	}
	result, err := h.service.List(c.Request.Context(), ListInput{
		Page:        page,
		PageSize:    pageSize,
		Action:      strings.TrimSpace(c.Query("action")),
		ActorUserID: strings.TrimSpace(c.Query("actorUserId")),
	})
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to list audit logs", nil)
		return
	}
	c.JSON(http.StatusOK, result)
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
