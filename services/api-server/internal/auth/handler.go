package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/apiutil"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type loginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceName string `json:"deviceName" binding:"required"`
	ClientType string `json:"clientType" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "username, password, deviceName, and clientType are required", nil)
		return
	}
	if request.ClientType != "android" && request.ClientType != "web" {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "clientType must be android or web", nil)
		return
	}

	session, err := h.service.Login(
		c.Request.Context(),
		strings.TrimSpace(request.Username),
		request.Password,
		strings.TrimSpace(request.DeviceName),
		request.ClientType,
	)
	if errors.Is(err, ErrInvalidCredentials) {
		apiutil.Error(c, http.StatusUnauthorized, "invalid_credentials", "Username or password is invalid", nil)
		return
	}
	if errors.Is(err, ErrUserDisabled) {
		apiutil.Error(c, http.StatusForbidden, "user_disabled", "User account is disabled", nil)
		return
	}
	if err != nil {
		log.Printf("auth login failed to create session: %v", err)
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to create session", nil)
		return
	}

	c.JSON(http.StatusOK, session)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *Handler) Refresh(c *gin.Context) {
	var request refreshRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "refreshToken is required", nil)
		return
	}

	session, err := h.service.Refresh(c.Request.Context(), request.RefreshToken)
	if errors.Is(err, ErrInvalidRefreshToken) {
		apiutil.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token is invalid or expired", nil)
		return
	}
	if errors.Is(err, ErrUserDisabled) {
		apiutil.Error(c, http.StatusForbidden, "user_disabled", "User account is disabled", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to refresh session", nil)
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *Handler) Logout(c *gin.Context) {
	claims, err := ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}

	var request refreshRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "refreshToken is required", nil)
		return
	}

	err = h.service.Logout(c.Request.Context(), claims.Subject, request.RefreshToken)
	if errors.Is(err, ErrInvalidRefreshToken) {
		apiutil.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token is invalid or already revoked", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to logout", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) Me(c *gin.Context) {
	claims, err := ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}

	user, err := h.service.Me(c.Request.Context(), claims.Subject)
	if errors.Is(err, ErrUserNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "User not found", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to load user", nil)
		return
	}
	if user.Status != "active" {
		apiutil.Error(c, http.StatusForbidden, "user_disabled", "User account is disabled", nil)
		return
	}

	c.JSON(http.StatusOK, user)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	claims, err := ClaimsFromContext(c)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}

	var request changePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "currentPassword and newPassword are required", nil)
		return
	}
	if len(request.NewPassword) < 8 {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "New password must be at least 8 characters", nil)
		return
	}

	err = h.service.ChangePassword(
		c.Request.Context(),
		claims.Subject,
		request.CurrentPassword,
		request.NewPassword,
	)
	if errors.Is(err, ErrCurrentPassword) {
		apiutil.Error(c, http.StatusUnauthorized, "invalid_current_password", "Current password is invalid", nil)
		return
	}
	if errors.Is(err, ErrPasswordUnchanged) {
		apiutil.Error(c, http.StatusConflict, "password_unchanged", "New password must be different", nil)
		return
	}
	if errors.Is(err, ErrUserNotFound) {
		apiutil.Error(c, http.StatusNotFound, "not_found", "Active user not found", nil)
		return
	}
	if err != nil {
		log.Printf("auth change password failed: %v", err)
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Unable to change password", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
