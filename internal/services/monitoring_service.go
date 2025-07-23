package services

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock-service/internal/cache"
	"stock-service/internal/config"
	"stock-service/internal/models"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type MonitoringService interface {
	GetMetrics(ctx context.Context) *models.MonitoringResponse
	RecordRequest(data models.RequestData)
	GetCacheStats() models.CacheMetrics
	GetDatabaseStats(ctx context.Context) models.DatabaseMetrics
	GetSystemStats() models.SystemMetrics
	GetRedisStats(ctx context.Context) models.RedisMetrics
}

type monitoringService struct {
	logger       *zap.Logger
	config       *config.Config
	redisClient  *redis.Client
	dbPool       *sql.DB
	productCache *cache.ProductCache

	// Métricas de requests
	requestsMutex sync.RWMutex
	requests      map[string]*models.EndpointMetrics
	slowRequests  []models.SlowRequest
	errors        []models.RequestError

	// Contadores
	totalRequests int64
	totalHits     int64
	totalMisses   int64
	totalQueries  int64

	// Timestamps
	startTime time.Time
}

func NewMonitoringService(
	logger *zap.Logger,
	config *config.Config,
	redisClient *redis.Client,
	dbPool *sql.DB,
	productCache *cache.ProductCache,
) MonitoringService {
	return &monitoringService{
		logger:       logger,
		config:       config,
		redisClient:  redisClient,
		dbPool:       dbPool,
		productCache: productCache,
		requests:     make(map[string]*models.EndpointMetrics),
		startTime:    time.Now(),
	}
}

func (s *monitoringService) RecordRequest(data models.RequestData) {
	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	// Crear key del endpoint
	endpointKey := fmt.Sprintf("%s %s", data.Method, data.Endpoint)

	// Obtener o crear métricas del endpoint
	metrics, exists := s.requests[endpointKey]
	if !exists {
		metrics = &models.EndpointMetrics{}
		s.requests[endpointKey] = metrics
	}

	// Actualizar métricas
	metrics.Count++
	durationMs := int64(data.Duration.Milliseconds())
	metrics.TotalTime += durationMs
	metrics.AvgTime = float64(metrics.TotalTime) / float64(metrics.Count)

	// Incrementar contador total
	s.totalRequests++

	// Registrar request lento (> 1000ms)
	if durationMs > 1000 {
		slowReq := models.SlowRequest{
			Endpoint:  endpointKey,
			Duration:  durationMs,
			Timestamp: data.Timestamp,
		}
		s.slowRequests = append(s.slowRequests, slowReq)

		// Mantener solo los últimos 100 requests lentos
		if len(s.slowRequests) > 100 {
			s.slowRequests = s.slowRequests[1:]
		}
	}

	// Registrar error
	if data.Error != nil || data.StatusCode >= 400 {
		errorReq := models.RequestError{
			Endpoint:   endpointKey,
			StatusCode: data.StatusCode,
			Timestamp:  data.Timestamp,
		}
		s.errors = append(s.errors, errorReq)

		// Mantener solo los últimos 100 errores
		if len(s.errors) > 100 {
			s.errors = s.errors[1:]
		}
	}
}

func (s *monitoringService) GetMetrics(ctx context.Context) *models.MonitoringResponse {
	s.requestsMutex.RLock()
	defer s.requestsMutex.RUnlock()

	// Calcular métricas de requests
	requestMetrics := s.calculateRequestMetrics()

	// Obtener métricas de otros servicios
	cacheMetrics := s.GetCacheStats()
	databaseMetrics := s.GetDatabaseStats(ctx)
	systemMetrics := s.GetSystemStats()
	redisMetrics := s.GetRedisStats(ctx)

	// Calcular métricas de rendimiento
	performanceMetrics := s.calculatePerformanceMetrics()

	return &models.MonitoringResponse{
		Requests:    requestMetrics,
		Performance: performanceMetrics,
		Cache:       cacheMetrics,
		Database:    databaseMetrics,
		System:      systemMetrics,
		Redis:       redisMetrics,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Version:     "2.0",
		GeneratedBy: "Go Monitoring Service",
	}
}

func (s *monitoringService) calculateRequestMetrics() models.RequestMetrics {
	// Crear slice de endpoints para ordenar
	var endpoints []struct {
		key     string
		metrics *models.EndpointMetrics
	}

	for key, metrics := range s.requests {
		endpoints = append(endpoints, struct {
			key     string
			metrics *models.EndpointMetrics
		}{key, metrics})
	}

	// Ordenar por count descendente
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].metrics.Count > endpoints[j].metrics.Count
	})

	// Crear top endpoints (máximo 10)
	var topEndpoints []models.TopEndpoint
	for i, endpoint := range endpoints {
		if i >= 10 {
			break
		}
		topEndpoints = append(topEndpoints, models.TopEndpoint{
			Endpoint:  endpoint.key,
			Count:     endpoint.metrics.Count,
			AvgTimeMs: fmt.Sprintf("%.2fms", endpoint.metrics.AvgTime),
		})
	}

	// Convertir map de punteros a map de valores
	byEndpoint := make(map[string]models.EndpointMetrics)
	for key, metrics := range s.requests {
		byEndpoint[key] = *metrics
	}

	return models.RequestMetrics{
		Total:             len(s.requests),
		ByEndpoint:        byEndpoint,
		SlowRequests:      s.slowRequests,
		Errors:            s.errors,
		TotalRequests:     int(s.totalRequests),
		SlowRequestsCount: len(s.slowRequests),
		ErrorsCount:       len(s.errors),
		TopEndpoints:      topEndpoints,
	}
}

func (s *monitoringService) calculatePerformanceMetrics() models.PerformanceMetrics {
	var totalTime int64
	var maxTime int64
	var minTime int64 = math.MaxInt64
	var count int

	for _, metrics := range s.requests {
		totalTime += metrics.TotalTime
		if metrics.TotalTime > maxTime {
			maxTime = metrics.TotalTime
		}
		if metrics.TotalTime < minTime {
			minTime = metrics.TotalTime
		}
		count += metrics.Count
	}

	var avgTime float64
	if count > 0 {
		avgTime = float64(totalTime) / float64(count)
	}

	if minTime == math.MaxInt64 {
		minTime = 0
	}

	return models.PerformanceMetrics{
		AvgResponseTime:   avgTime,
		MaxResponseTime:   maxTime,
		MinResponseTime:   minTime,
		AvgResponseTimeMs: fmt.Sprintf("%.2fms", avgTime),
		MaxResponseTimeMs: fmt.Sprintf("%dms", maxTime),
		MinResponseTimeMs: fmt.Sprintf("%dms", minTime),
	}
}

func (s *monitoringService) GetCacheStats() models.CacheMetrics {
	// Obtener stats del cache de productos
	cacheStats := s.productCache.GetStats()

	// Calcular hit rate
	var hitRate float64
	if cacheStats.TotalRequests > 0 {
		hitRate = float64(cacheStats.Hits) / float64(cacheStats.TotalRequests)
	}

	return models.CacheMetrics{
		Connected:         true,
		TotalKeys:         cacheStats.TotalKeys,
		ByPrefix:          map[string]int{"product": cacheStats.TotalKeys},
		HitRate:           hitRate,
		Status:            "online",
		HitRatePercentage: fmt.Sprintf("%.2f%%", hitRate*100),
		TotalHits:         cacheStats.Hits,
		TotalMisses:       cacheStats.Misses,
		TotalRequests:     cacheStats.TotalRequests,
	}
}

func (s *monitoringService) GetDatabaseStats(ctx context.Context) models.DatabaseMetrics {
	// Obtener stats de la conexión de la base de datos
	stats := s.dbPool.Stats()

	return models.DatabaseMetrics{
		ActiveConnections:      int(stats.OpenConnections),
		TotalQueries:           s.totalQueries,
		SlowQueries:            []string{}, // Por ahora vacío
		Status:                 "online",
		ActiveConnectionsCount: int(stats.OpenConnections),
	}
}

func (s *monitoringService) GetSystemStats() models.SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calcular uptime
	uptime := time.Since(s.startTime).Seconds()
	uptimeHours := uptime / 3600

	// Obtener info del sistema
	platform := runtime.GOOS
	environment := "production"
	if s.config.Server.GinMode == "debug" {
		environment = "development"
	}

	return models.SystemMetrics{
		MemoryUsage: fmt.Sprintf("%.2f", float64(m.Alloc)/1024/1024),
		CPUUsage:    "0.00", // Por ahora hardcodeado
		Uptime:      uptime,
		Memory: models.MemoryMetrics{
			HeapUsed:  fmt.Sprintf("%.2f MB", float64(m.HeapAlloc)/1024/1024),
			HeapTotal: fmt.Sprintf("%.2f MB", float64(m.HeapSys)/1024/1024),
			External:  fmt.Sprintf("%.2f MB", float64(m.OtherSys)/1024/1024),
			RSS:       fmt.Sprintf("%.2f MB", float64(m.Sys)/1024/1024),
		},
		CPU: models.CPUMetrics{
			UsagePercentage: "0.00%",
			UserTime:        "0.00s",
			SystemTime:      "0.00s",
		},
		UptimeHours: fmt.Sprintf("%.2fh", uptimeHours),
		NodeVersion: runtime.Version(),
		Platform:    platform,
		Environment: environment,
	}
}

func (s *monitoringService) GetRedisStats(ctx context.Context) models.RedisMetrics {
	// Verificar conexión
	_, err := s.redisClient.Ping(ctx).Result()
	connected := err == nil

	// Obtener info de Redis
	var keys int
	var memory string
	var memoryMB string

	if connected {
		// Obtener número de keys
		keysResult, err := s.redisClient.DBSize(ctx).Result()
		if err == nil {
			keys = int(keysResult)
		}

		// Obtener info de memoria
		info, err := s.redisClient.Info(ctx, "memory").Result()
		if err == nil {
			lines := strings.Split(info, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "used_memory:") {
					parts := strings.Split(line, ":")
					if len(parts) == 2 {
						memory = strings.TrimSpace(parts[1])
						if memBytes, err := strconv.ParseInt(memory, 10, 64); err == nil {
							memoryMB = fmt.Sprintf("%.2f MB", float64(memBytes)/1024/1024)
						}
					}
					break
				}
			}
		}
	}

	status := "offline"
	if connected {
		status = "online"
	}

	return models.RedisMetrics{
		Connected: connected,
		Keys:      keys,
		Memory:    memory,
		Status:    status,
		MemoryMB:  memoryMB,
	}
}
