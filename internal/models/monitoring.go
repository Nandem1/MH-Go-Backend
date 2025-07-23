package models

import "time"

// MonitoringResponse respuesta completa del sistema de monitoring
type MonitoringResponse struct {
	Requests    RequestMetrics     `json:"requests"`
	Performance PerformanceMetrics `json:"performance"`
	Cache       CacheMetrics       `json:"cache"`
	Database    DatabaseMetrics    `json:"database"`
	System      SystemMetrics      `json:"system"`
	Redis       RedisMetrics       `json:"redis"`
	Timestamp   string             `json:"timestamp"`
	Version     string             `json:"version"`
	GeneratedBy string             `json:"generated_by"`
}

// RequestMetrics métricas de requests
type RequestMetrics struct {
	Total             int                        `json:"total"`
	ByEndpoint        map[string]EndpointMetrics `json:"byEndpoint"`
	SlowRequests      []SlowRequest              `json:"slowRequests"`
	Errors            []RequestError             `json:"errors"`
	TotalRequests     int                        `json:"total_requests"`
	SlowRequestsCount int                        `json:"slow_requests_count"`
	ErrorsCount       int                        `json:"errors_count"`
	TopEndpoints      []TopEndpoint              `json:"top_endpoints"`
}

// EndpointMetrics métricas por endpoint
type EndpointMetrics struct {
	Count     int     `json:"count"`
	AvgTime   float64 `json:"avgTime"`
	TotalTime int64   `json:"totalTime"`
}

// SlowRequest request lento
type SlowRequest struct {
	Endpoint  string    `json:"endpoint"`
	Duration  int64     `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
}

// RequestError error de request
type RequestError struct {
	Endpoint   string    `json:"endpoint"`
	StatusCode int       `json:"statusCode"`
	Timestamp  time.Time `json:"timestamp"`
}

// TopEndpoint endpoint más usado
type TopEndpoint struct {
	Endpoint  string `json:"endpoint"`
	Count     int    `json:"count"`
	AvgTimeMs string `json:"avg_time_ms"`
}

// PerformanceMetrics métricas de rendimiento
type PerformanceMetrics struct {
	AvgResponseTime   float64 `json:"avgResponseTime"`
	MaxResponseTime   int64   `json:"maxResponseTime"`
	MinResponseTime   int64   `json:"minResponseTime"`
	AvgResponseTimeMs string  `json:"avg_response_time_ms"`
	MaxResponseTimeMs string  `json:"max_response_time_ms"`
	MinResponseTimeMs string  `json:"min_response_time_ms"`
}

// CacheMetrics métricas de cache
type CacheMetrics struct {
	Connected         bool           `json:"connected"`
	TotalKeys         int            `json:"totalKeys"`
	ByPrefix          map[string]int `json:"byPrefix"`
	HitRate           float64        `json:"hitRate"`
	Status            string         `json:"status"`
	HitRatePercentage string         `json:"hit_rate_percentage"`
	TotalHits         int64          `json:"total_hits"`
	TotalMisses       int64          `json:"total_misses"`
	TotalRequests     int64          `json:"total_requests"`
}

// DatabaseMetrics métricas de base de datos
type DatabaseMetrics struct {
	ActiveConnections      int      `json:"activeConnections"`
	TotalQueries           int64    `json:"totalQueries"`
	SlowQueries            []string `json:"slowQueries"`
	Status                 string   `json:"status"`
	ActiveConnectionsCount int      `json:"active_connections"`
}

// SystemMetrics métricas del sistema
type SystemMetrics struct {
	MemoryUsage string        `json:"memoryUsage"`
	CPUUsage    string        `json:"cpuUsage"`
	Uptime      float64       `json:"uptime"`
	Memory      MemoryMetrics `json:"memory"`
	CPU         CPUMetrics    `json:"cpu"`
	UptimeHours string        `json:"uptime_hours"`
	NodeVersion string        `json:"node_version"`
	Platform    string        `json:"platform"`
	Environment string        `json:"environment"`
}

// MemoryMetrics métricas de memoria
type MemoryMetrics struct {
	HeapUsed  string `json:"heapUsed"`
	HeapTotal string `json:"heapTotal"`
	External  string `json:"external"`
	RSS       string `json:"rss"`
}

// CPUMetrics métricas de CPU
type CPUMetrics struct {
	UsagePercentage string `json:"usage_percentage"`
	UserTime        string `json:"user_time"`
	SystemTime      string `json:"system_time"`
}

// RedisMetrics métricas de Redis
type RedisMetrics struct {
	Connected bool   `json:"connected"`
	Keys      int    `json:"keys"`
	Memory    string `json:"memory"`
	Status    string `json:"status"`
	MemoryMB  string `json:"memory_mb"`
}

// RequestData datos de un request individual
type RequestData struct {
	Endpoint   string
	Method     string
	Duration   time.Duration
	StatusCode int
	Timestamp  time.Time
	Error      error
}
