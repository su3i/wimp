package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler registers the /metrics endpoint for Prometheus
func MetricsHandler(router *gin.Engine) {
	// Wrap the promhttp.Handler() in a Gin handler
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
