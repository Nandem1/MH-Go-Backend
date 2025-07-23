package handlers

import (
	"context"
	"net/http"
	"time"

	"stock-service/internal/models"
	"stock-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type MonitoringHandler struct {
	monitoringService services.MonitoringService
	logger            *zap.Logger
}

func NewMonitoringHandler(monitoringService services.MonitoringService, logger *zap.Logger) *MonitoringHandler {
	return &MonitoringHandler{
		monitoringService: monitoringService,
		logger:            logger,
	}
}

// GetMetrics maneja la petición HTTP para obtener métricas
func (h *MonitoringHandler) GetMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_metrics"))

	ctx := c.Request.Context()
	metrics := h.monitoringService.GetMetrics(ctx)

	logger.Info("Métricas obtenidas exitosamente",
		zap.Int("total_requests", metrics.Requests.TotalRequests),
		zap.Int("total_endpoints", metrics.Requests.Total),
		zap.String("avg_response_time", metrics.Performance.AvgResponseTimeMs))

	c.JSON(http.StatusOK, metrics)
}

// WebSocketUpgrader configuración para WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Permitir todas las conexiones para desarrollo
	},
}

// WebSocketMetrics maneja la conexión WebSocket para métricas en tiempo real
func (h *MonitoringHandler) WebSocketMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "websocket_metrics"))

	// Actualizar a WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Error actualizando a WebSocket", zap.Error(err))
		return
	}
	defer conn.Close()

	logger.Info("Conexión WebSocket establecida")

	// Configurar ping/pong
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Enviar métricas cada 10 segundos
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Obtener métricas
			metrics := h.monitoringService.GetMetrics(context.Background())

			// Enviar por WebSocket
			if err := conn.WriteJSON(metrics); err != nil {
				logger.Error("Error enviando métricas por WebSocket", zap.Error(err))
				return
			}

			logger.Debug("Métricas enviadas por WebSocket",
				zap.Int("total_requests", metrics.Requests.TotalRequests),
				zap.String("timestamp", metrics.Timestamp))

		case <-c.Request.Context().Done():
			logger.Info("Conexión WebSocket cerrada por contexto")
			return
		}
	}
}

// RecordRequestMiddleware middleware para registrar requests
func (h *MonitoringHandler) RecordRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Procesar request
		c.Next()

		// Calcular duración
		duration := time.Since(start)

		// Filtrar endpoints de monitoring (igual que en Node.js)
		path := c.Request.URL.Path
		if h.shouldSkipMonitoring(path) {
			return
		}

		// Registrar request
		requestData := models.RequestData{
			Endpoint:   path,
			Method:     c.Request.Method,
			Duration:   duration,
			StatusCode: c.Writer.Status(),
			Timestamp:  time.Now(),
			Error:      nil, // TODO: Capturar errores específicos
		}

		h.monitoringService.RecordRequest(requestData)
	}
}

// shouldSkipMonitoring determina si un endpoint debe ser excluido del monitoring
func (h *MonitoringHandler) shouldSkipMonitoring(path string) bool {
	// Lista de endpoints a excluir (igual que en Node.js)
	excludedPaths := []string{
		"/api/v1/monitoring/metrics",
		"/api/v1/monitoring/metrics/summary",
		"/api/v1/monitoring/ws",
		"/health/monitoring",
		"/health",
		"/",
	}

	for _, excludedPath := range excludedPaths {
		if path == excludedPath {
			return true
		}
	}

	return false
}

// HealthCheck endpoint de health check
func (h *MonitoringHandler) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	// Verificar servicios básicos
	health := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "2.0",
		"services": gin.H{
			"database": "online",
			"redis":    "online",
			"cache":    "online",
		},
	}

	// Verificar Redis
	redisMetrics := h.monitoringService.GetRedisStats(ctx)
	if !redisMetrics.Connected {
		health["services"].(gin.H)["redis"] = "offline"
		health["status"] = "degraded"
	}

	// Verificar Database
	dbMetrics := h.monitoringService.GetDatabaseStats(ctx)
	if dbMetrics.Status != "online" {
		health["services"].(gin.H)["database"] = "offline"
		health["status"] = "degraded"
	}

	c.JSON(http.StatusOK, health)
}

// GetMetricsSummary endpoint para métricas resumidas
func (h *MonitoringHandler) GetMetricsSummary(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_metrics_summary"))

	ctx := c.Request.Context()
	metrics := h.monitoringService.GetMetrics(ctx)

	// Crear resumen
	summary := gin.H{
		"requests": gin.H{
			"total":         metrics.Requests.TotalRequests,
			"endpoints":     metrics.Requests.Total,
			"errors":        metrics.Requests.ErrorsCount,
			"slow_requests": metrics.Requests.SlowRequestsCount,
		},
		"performance": gin.H{
			"avg_response_time": metrics.Performance.AvgResponseTimeMs,
			"max_response_time": metrics.Performance.MaxResponseTimeMs,
			"min_response_time": metrics.Performance.MinResponseTimeMs,
		},
		"cache": gin.H{
			"hit_rate":   metrics.Cache.HitRatePercentage,
			"total_keys": metrics.Cache.TotalKeys,
			"status":     metrics.Cache.Status,
		},
		"database": gin.H{
			"active_connections": metrics.Database.ActiveConnections,
			"total_queries":      metrics.Database.TotalQueries,
			"status":             metrics.Database.Status,
		},
		"system": gin.H{
			"memory_usage": metrics.System.MemoryUsage,
			"uptime":       metrics.System.UptimeHours,
			"platform":     metrics.System.Platform,
		},
		"redis": gin.H{
			"connected": metrics.Redis.Connected,
			"keys":      metrics.Redis.Keys,
			"memory":    metrics.Redis.MemoryMB,
			"status":    metrics.Redis.Status,
		},
		"timestamp": metrics.Timestamp,
	}

	logger.Info("Resumen de métricas generado",
		zap.Int("total_requests", metrics.Requests.TotalRequests),
		zap.String("avg_response_time", metrics.Performance.AvgResponseTimeMs))

	c.JSON(http.StatusOK, summary)
}
