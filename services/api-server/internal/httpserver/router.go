package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/config"
	"ptt-fleet/services/api-server/internal/db"
	"ptt-fleet/services/api-server/internal/gps"
	"ptt-fleet/services/api-server/internal/groups"
	"ptt-fleet/services/api-server/internal/sos"
	"ptt-fleet/services/api-server/internal/users"
	realtime "ptt-fleet/services/api-server/internal/ws"
)

func NewRouter(cfg config.Config, store *db.Store, hub *realtime.Hub) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	if hub == nil {
		hub = realtime.NewHub()
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"env":    cfg.AppEnv,
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	router.GET("/readyz", func(c *gin.Context) {
		if err := store.Ready(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	api := router.Group("/api")
	api.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "ptt-fleet-api",
			"version": "0.0.0",
		})
	})

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL())
	authService := auth.NewService(store, tokenManager, cfg.RefreshTTL())
	authHandler := auth.NewHandler(authService)
	userHandler := users.NewHandler(users.NewService(store))
	groupHandler := groups.NewHandler(groups.NewService(store))
	websocketHandler := realtime.NewHandler(
		tokenManager,
		realtime.NewRepository(store),
		gps.NewService(store),
		sos.NewService(store),
		hub,
	)

	router.GET("/ws", websocketHandler.Connect)

	authRoutes := api.Group("/auth")
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)

	protected := api.Group("")
	protected.Use(auth.Middleware(tokenManager))
	protected.POST("/auth/logout", authHandler.Logout)
	protected.GET("/auth/me", authHandler.Me)

	protected.GET("/users", userHandler.List)
	protected.POST("/users", userHandler.Create)
	protected.GET("/users/:id", userHandler.Get)
	protected.PATCH("/users/:id", userHandler.Update)
	protected.DELETE("/users/:id", userHandler.Delete)

	protected.GET("/groups", groupHandler.List)
	protected.POST("/groups", groupHandler.Create)
	protected.GET("/groups/:id", groupHandler.Get)
	protected.PATCH("/groups/:id", groupHandler.Update)
	protected.DELETE("/groups/:id", groupHandler.Delete)
	protected.POST("/groups/:id/members", groupHandler.AddMember)
	protected.DELETE("/groups/:id/members/:userId", groupHandler.RemoveMember)

	return router
}
