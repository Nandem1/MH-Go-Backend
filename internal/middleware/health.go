package middleware

import (
	"context"
	"net/http"
	"time"

	"stock-service/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type HealthChecker struct {
	postgresDB *database.PostgresDB
	redisDB    *database.RedisDB
	logger     *zap.Logger
}

func NewHealthChecker(postgresDB *database.PostgresDB, redisDB *database.RedisDB, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		postgresDB: postgresDB,
		redisDB:    redisDB,
		logger:     logger,
	}
}

func (h *HealthChecker) HealthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"services":  make(map[string]interface{}),
	}

	// Verificar PostgreSQL
	postgresStatus := "healthy"
	if err := h.postgresDB.Ping(); err != nil {
		postgresStatus = "unhealthy"
		status["status"] = "unhealthy"
		h.logger.Error("PostgreSQL health check failed", zap.Error(err))
	}

	// Obtener estadísticas de PostgreSQL
	postgresStats := h.postgresDB.GetStats()
	status["services"].(map[string]interface{})["postgresql"] = gin.H{
		"status": postgresStatus,
		"stats": gin.H{
			"max_open_connections": postgresStats.MaxOpenConnections,
			"open_connections":     postgresStats.OpenConnections,
			"in_use":               postgresStats.InUse,
			"idle":                 postgresStats.Idle,
		},
	}

	// Verificar Redis
	redisStatus := "healthy"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.redisDB.Ping(ctx); err != nil {
		redisStatus = "unhealthy"
		status["status"] = "unhealthy"
		h.logger.Error("Redis health check failed", zap.Error(err))
	}

	// Obtener estadísticas de Redis
	redisStats, err := h.redisDB.GetStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get Redis stats", zap.Error(err))
		redisStats = "unavailable"
	}

	status["services"].(map[string]interface{})["redis"] = gin.H{
		"status": redisStatus,
		"stats":  redisStats,
	}

	// Determinar código de respuesta HTTP
	httpStatus := http.StatusOK
	if status["status"] == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}
