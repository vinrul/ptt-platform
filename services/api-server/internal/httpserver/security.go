package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"ptt-fleet/services/api-server/internal/apiutil"
)

type loginRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

type redisLoginRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

var incrementRateLimit = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func (l redisLoginRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	count, err := incrementRateLimit.Run(
		ctx,
		l.client,
		[]string{"rate:login:" + key},
		int64(l.window/time.Second),
	).Int64()
	if err != nil {
		return false, err
	}
	return count <= int64(l.limit), nil
}

func loginRateLimit(limiter loginRateLimiter, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := loginRateLimitKey(c)
		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			log.Printf("login rate limiter unavailable: %v", err)
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			apiutil.Error(c, http.StatusTooManyRequests, "rate_limited", "Too many login attempts. Try again later", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func loginRateLimitKey(c *gin.Context) string {
	ip := net.ParseIP(c.ClientIP())
	clientIP := c.ClientIP()
	if ip != nil {
		clientIP = ip.String()
	}
	username := strings.ToLower(strings.TrimSpace(loginUsername(c)))
	sum := sha256.Sum256([]byte(clientIP + "\x00" + username))
	return fmt.Sprintf("%x", sum[:16])
}

func loginUsername(c *gin.Context) string {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var request struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return ""
	}
	return request.Username
}

func cors(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSuffix(c.GetHeader("Origin"), "/")
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			apiutil.Error(c, http.StatusForbidden, "origin_not_allowed", "Request origin is not allowed", nil)
			c.Abort()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "600")
		c.Header("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func requireHTTPS(appEnv string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appEnv != "production" || c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Next()
			return
		}
		if c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/readyz" {
			c.Next()
			return
		}
		apiutil.Error(c, http.StatusUpgradeRequired, "https_required", "HTTPS is required", nil)
		c.Abort()
	}
}

func requestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		path := param.Path
		if queryIndex := strings.IndexByte(path, '?'); queryIndex >= 0 {
			path = path[:queryIndex]
		}
		return fmt.Sprintf(
			"[GIN] %s | %3d | %13v | %15s | %-7s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			path,
		)
	})
}
