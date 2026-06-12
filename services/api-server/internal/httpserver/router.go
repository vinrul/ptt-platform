package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/audit"
	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/config"
	"ptt-fleet/services/api-server/internal/db"
	"ptt-fleet/services/api-server/internal/devices"
	"ptt-fleet/services/api-server/internal/firebase"
	"ptt-fleet/services/api-server/internal/geo"
	"ptt-fleet/services/api-server/internal/gps"
	"ptt-fleet/services/api-server/internal/groups"
	"ptt-fleet/services/api-server/internal/ptt"
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
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		panic(err)
	}
	router.Use(requestLogger(), gin.Recovery(), requireHTTPS(cfg.AppEnv), cors(cfg.AllowedOrigins))

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
	gpsHandler := gps.NewHandler(gps.NewService(store))
	groupHandler := groups.NewHandler(groups.NewService(store))
	deviceHandler := devices.NewHandler(devices.NewService(store))
	auditHandler := audit.NewHandler(audit.NewService(store))
	geoHandler := geo.NewHandler(cfg.ReverseGeocodeURL, cfg.RouteServiceURL)
	firebaseClient, err := firebase.NewClient(cfg.FirebaseCredentialsPath)
	if err != nil {
		panic(err)
	}
	websocketHandler := realtime.NewHandler(
		tokenManager,
		realtime.NewRepository(store),
		gps.NewService(store),
		sos.NewService(store),
		ptt.NewManager(ptt.NewRepository(store)),
		hub,
		firebaseClient,
		cfg.AllowedOrigins,
	)

	router.GET("/ws", websocketHandler.Connect)

	authRoutes := api.Group("/auth")
	if store != nil && store.Redis != nil {
		authRoutes.POST("/login", loginRateLimit(redisLoginRateLimiter{
			client: store.Redis,
			limit:  cfg.LoginRateLimit,
			window: cfg.LoginRateWindow,
		}, cfg.LoginRateWindow), authHandler.Login)
	} else {
		authRoutes.POST("/login", authHandler.Login)
	}
	authRoutes.POST("/refresh", authHandler.Refresh)

	protected := api.Group("")
	protected.Use(auth.Middleware(tokenManager))
	protected.POST("/auth/logout", authHandler.Logout)
	protected.GET("/auth/me", authHandler.Me)
	protected.POST("/auth/change-password", authHandler.ChangePassword)

	protected.GET("/users", userHandler.List)
	protected.POST("/users", userHandler.Create)
	protected.GET("/users/:id", userHandler.Get)
	protected.PATCH("/users/:id", userHandler.Update)
	protected.DELETE("/users/:id", userHandler.Delete)
	protected.POST("/users/:id/reset-password", userHandler.ResetPassword)
	protected.GET("/users/:id/gps-history", gpsHandler.History)

	protected.GET("/groups", groupHandler.List)
	protected.POST("/groups", groupHandler.Create)
	protected.GET("/groups/:id", groupHandler.Get)
	protected.PATCH("/groups/:id", groupHandler.Update)
	protected.DELETE("/groups/:id", groupHandler.Delete)
	protected.POST("/groups/:id/members", groupHandler.AddMember)
	protected.DELETE("/groups/:id/members/:userId", groupHandler.RemoveMember)
	protected.GET("/groups/:id/locations", gpsHandler.LatestForGroup)
	protected.GET("/geocode/reverse", geoHandler.Reverse)
	protected.POST("/routes/line", geoHandler.RouteLine)
	protected.GET("/devices", deviceHandler.List)
	protected.GET("/devices/:id", deviceHandler.Get)
	protected.PUT("/devices/:id/push-token", deviceHandler.UpdatePushToken)
	protected.GET("/audit-logs", auditHandler.List)

	return router
}
