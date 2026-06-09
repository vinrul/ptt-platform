package groups

import (
	"errors"
	"net/http"
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

	var items []Group
	if claims.Role == "field_user" {
		items, err = h.service.ListForUser(c.Request.Context(), claims.Subject)
	} else {
		items, err = h.service.List(c.Request.Context())
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to list groups", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type createRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (h *Handler) Create(c *gin.Context) {
	claims, ok := requireSuperAdmin(c)
	if !ok {
		return
	}

	var request createRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "name is required", nil)
		return
	}

	group, err := h.service.Create(
		c.Request.Context(),
		claims.Subject,
		strings.TrimSpace(request.Name),
		strings.TrimSpace(request.Description),
	)
	if errors.Is(err, ErrNameConflict) {
		apiutil.Error(c, http.StatusConflict, "group_name_conflict", "Group name already exists", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to create group", nil)
		return
	}
	c.JSON(http.StatusCreated, group)
}

func (h *Handler) Get(c *gin.Context) {
	if _, ok := requireReadRole(c); !ok {
		return
	}

	group, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Group not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load group", nil)
		return
	}
	c.JSON(http.StatusOK, group)
}

type updateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *Handler) Update(c *gin.Context) {
	claims, ok := requireSuperAdmin(c)
	if !ok {
		return
	}

	var request updateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Invalid request body", nil)
		return
	}
	if request.Name == nil && request.Description == nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "At least one field is required", nil)
		return
	}
	if request.Name != nil {
		trimmed := strings.TrimSpace(*request.Name)
		if trimmed == "" {
			apiutil.Error(c, http.StatusBadRequest, "validation_error", "name cannot be empty", nil)
			return
		}
		request.Name = &trimmed
	}
	if request.Description != nil {
		trimmed := strings.TrimSpace(*request.Description)
		request.Description = &trimmed
	}

	group, err := h.service.Update(c.Request.Context(), claims.Subject, c.Param("id"), request.Name, request.Description)
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Group not found", nil)
		return
	}
	if errors.Is(err, ErrNameConflict) {
		apiutil.Error(c, http.StatusConflict, "group_name_conflict", "Group name already exists", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to update group", nil)
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *Handler) Delete(c *gin.Context) {
	claims, ok := requireSuperAdmin(c)
	if !ok {
		return
	}

	err := h.service.Delete(c.Request.Context(), claims.Subject, c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Group not found", nil)
		return
	}
	if errors.Is(err, ErrHasMembers) {
		apiutil.Error(c, http.StatusConflict, "group_has_members", "Remove all members before deleting the group", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to delete group", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

type addMemberRequest struct {
	UserID      string `json:"userId" binding:"required"`
	RoleInGroup string `json:"roleInGroup" binding:"required"`
}

func (h *Handler) AddMember(c *gin.Context) {
	claims, ok := requireSuperAdmin(c)
	if !ok {
		return
	}

	var request addMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil || !validGroupRole(request.RoleInGroup) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "userId and a valid roleInGroup are required", nil)
		return
	}

	member, err := h.service.AddMember(c.Request.Context(), claims.Subject, c.Param("id"), request.UserID, request.RoleInGroup)
	if errors.Is(err, ErrMemberConflict) {
		apiutil.Error(c, http.StatusConflict, "member_conflict", "User is already a member of this group", nil)
		return
	}
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Group or user not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to add group member", nil)
		return
	}
	c.JSON(http.StatusCreated, member)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	claims, ok := requireSuperAdmin(c)
	if !ok {
		return
	}

	err := h.service.RemoveMember(c.Request.Context(), claims.Subject, c.Param("id"), c.Param("userId"))
	if errors.Is(err, ErrMemberNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Group member not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to remove group member", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

func requireReadRole(c *gin.Context) (auth.Claims, bool) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return auth.Claims{}, false
	}
	if claims.Role == "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires an operator role", nil)
		return auth.Claims{}, false
	}
	return claims, true
}

func requireSuperAdmin(c *gin.Context) (auth.Claims, bool) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return auth.Claims{}, false
	}
	if claims.Role != "super_admin" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires super admin", nil)
		return auth.Claims{}, false
	}
	return claims, true
}

func validGroupRole(role string) bool {
	return role == "member" || role == "dispatcher" || role == "supervisor"
}
