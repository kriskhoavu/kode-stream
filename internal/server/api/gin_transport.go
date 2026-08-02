package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kode-stream/internal/common/httpx"
	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

const (
	requestIDHeader = "X-Request-ID"
	requestTimeout  = 30 * time.Second
)

func newTransport(config system.RuntimeConfig, register func(*gin.RouterGroup)) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	engine.HandleMethodNotAllowed = false
	engine.Use(gin.Recovery(), localExtensionCORSMiddleware(config), requestIDMiddleware(), timeoutMiddleware(requestTimeout))
	api := engine.Group("/api")
	if register != nil {
		register(api)
	}
	return engine
}

func localExtensionCORSMiddleware(config system.RuntimeConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if config.Mode == models.RuntimeModeLocal && strings.HasPrefix(origin, "chrome-extension://") {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
			if c.Request.Method == http.MethodOptions {
				c.Status(http.StatusNoContent)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID != "" {
			c.Writer.Header().Set(requestIDHeader, requestID)
		}
		c.Next()
	}
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func ginJSON(c *gin.Context, status int, data any) {
	c.Header("Content-Type", "application/json")
	c.JSON(status, data)
}

func ginAppError(c *gin.Context, err error) {
	status, message, code := httpx.MapError(err)
	payload := map[string]string{"error": message}
	if code != "" {
		payload["code"] = string(code)
	}
	c.Header("Content-Type", "application/json")
	c.JSON(status, payload)
}

func ginHTTPHandler(handler http.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, param := range c.Params {
			c.Request.SetPathValue(param.Key, param.Value)
		}
		handler(c.Writer, c.Request)
	}
}
