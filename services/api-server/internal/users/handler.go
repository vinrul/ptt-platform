package users

import (
	"errors"
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
	claims, ok := requireReadRole(c)
	if !ok {
		return
	}
	_ = claims

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 50)
	if pageSize > 100 {
		pageSize = 100
	}

	role := c.Query("role")
	status := c.Query("status")
	if role != "" && !validUserRole(role) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Invalid role filter", nil)
		return
	}
	if status != "" && !validUserStatus(status) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Invalid status filter", nil)
		return
	}

	result, err := h.service.List(c.Request.Context(), ListInput{
		Page:     page,
		PageSize: pageSize,
		Role:     role,
		Status:   status,
		Query:    strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to list users", nil)
		return
	}
	c.JSON(http.StatusOK, result)
}

type createRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"fullName" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

func (h *Handler) Create(c *gin.Context) {
	claims, ok := requireManagementRole(c)
	if !ok {
		return
	}

	var request createRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "username, password, fullName, and role are required", nil)
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.FullName = strings.TrimSpace(request.FullName)
	if request.Username == "" || request.FullName == "" || len(request.Password) < 8 || !validUserRole(request.Role) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Use a valid role and a password with at least 8 characters", nil)
		return
	}
	if claims.Role != "super_admin" && request.Role != "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "Dispatcher may only create field users", nil)
		return
	}

	user, err := h.service.Create(c.Request.Context(), claims.Subject, CreateInput{
		Username: request.Username,
		Password: request.Password,
		FullName: request.FullName,
		Role:     request.Role,
	})
	if errors.Is(err, ErrUsernameConflict) {
		apiutil.Error(c, http.StatusConflict, "username_conflict", "Username already exists", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to create user", nil)
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *Handler) Get(c *gin.Context) {
	claims, ok := requireReadRole(c)
	if !ok {
		return
	}
	_ = claims

	user, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load user", nil)
		return
	}
	c.JSON(http.StatusOK, user)
}

type updateRequest struct {
	FullName *string `json:"fullName"`
	Role     *string `json:"role"`
	Status   *string `json:"status"`
}

func (h *Handler) Update(c *gin.Context) {
	claims, ok := requireManagementRole(c)
	if !ok {
		return
	}

	target, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load user", nil)
		return
	}
	if claims.Role != "super_admin" && target.Role != "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "Dispatcher may only update field users", nil)
		return
	}

	var request updateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Invalid request body", nil)
		return
	}
	if request.FullName == nil && request.Role == nil && request.Status == nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "At least one field is required", nil)
		return
	}
	if request.FullName != nil {
		trimmed := strings.TrimSpace(*request.FullName)
		if trimmed == "" {
			apiutil.Error(c, http.StatusBadRequest, "validation_error", "fullName cannot be empty", nil)
			return
		}
		request.FullName = &trimmed
	}
	if request.Role != nil && (!validUserRole(*request.Role) || claims.Role != "super_admin") {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "Only super admin may change user roles", nil)
		return
	}
	if request.Status != nil && !validUserStatus(*request.Status) {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "Invalid user status", nil)
		return
	}

	user, err := h.service.Update(c.Request.Context(), claims.Subject, target.ID, UpdateInput{
		FullName: request.FullName,
		Role:     request.Role,
		Status:   request.Status,
	})
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to update user", nil)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) Delete(c *gin.Context) {
	claims, ok := requireManagementRole(c)
	if !ok {
		return
	}

	target, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load user", nil)
		return
	}
	if claims.Subject == target.ID {
		apiutil.Error(c, http.StatusConflict, "self_disable_forbidden", "You cannot disable your own account", nil)
		return
	}
	if claims.Role != "super_admin" && target.Role != "field_user" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "Dispatcher may only disable field users", nil)
		return
	}
	if err := h.service.Disable(c.Request.Context(), claims.Subject, target.ID); err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to disable user", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}
	if claims.Role != "super_admin" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "Only super admin may reset user passwords", nil)
		return
	}

	var request resetPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.NewPassword) < 8 {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "New password must be at least 8 characters", nil)
		return
	}

	err = h.service.ResetPassword(
		c.Request.Context(),
		claims.Subject,
		c.Param("id"),
		request.NewPassword,
	)
	if errors.Is(err, ErrNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to reset user password", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func requireReadRole(c *gin.Context) (auth.Claims, bool) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return auth.Claims{}, false
	}
	if claims.Role != "super_admin" && claims.Role != "dispatcher" && claims.Role != "supervisor" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires an operator role", nil)
		return auth.Claims{}, false
	}
	return claims, true
}

func requireManagementRole(c *gin.Context) (auth.Claims, bool) {
	claims, err := auth.ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return auth.Claims{}, false
	}
	if claims.Role != "super_admin" && claims.Role != "dispatcher" {
		apiutil.Error(c, http.StatusForbidden, "forbidden", "This endpoint requires an admin or dispatcher role", nil)
		return auth.Claims{}, false
	}
	return claims, true
}

func validUserRole(role string) bool {
	switch role {
	case "super_admin", "dispatcher", "supervisor", "field_user":
		return true
	default:
		return false
	}
}

func validUserStatus(status string) bool {
	return status == "active" || status == "disabled"
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
