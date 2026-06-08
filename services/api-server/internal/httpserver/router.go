package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/config"
	"ptt-fleet/services/api-server/internal/db"
)

func NewRouter(cfg config.Config, store *db.Store) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
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

	return router
}
