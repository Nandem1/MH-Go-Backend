package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"stock-service/internal/models"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// CacheStats estadísticas del caché
type CacheStats struct {
	Hits          int64
	Misses        int64
	TotalRequests int64
	TotalKeys     int
}

// ProductCache implementa caché multi-nivel para productos
type ProductCache struct {
	// L1 Cache: Memoria local (más rápido)
	l1Cache map[string]*models.ProductoCompleto
	l1Mutex sync.RWMutex

	// L2 Cache: Redis (persistente)
	redisClient *redis.Client

	// Configuración
	maxL1Size int
	ttl       time.Duration

	logger *zap.Logger

	// Estadísticas
	statsMutex sync.RWMutex
	hits       int64
	misses     int64
}

// NewProductCache crea una nueva instancia del caché
func NewProductCache(redisClient *redis.Client, maxL1Size int, ttl time.Duration, logger *zap.Logger) *ProductCache {
	pc := &ProductCache{
		l1Cache:     make(map[string]*models.ProductoCompleto),
		redisClient: redisClient,
		maxL1Size:   maxL1Size,
		ttl:         ttl,
		logger:      logger,
	}

	// Iniciar limpieza periódica del L1 cache
	go pc.cleanupL1Cache()

	return pc
}

// GetStats retorna estadísticas del caché
func (pc *ProductCache) GetStats() CacheStats {
	pc.statsMutex.RLock()
	defer pc.statsMutex.RUnlock()

	pc.l1Mutex.RLock()
	totalKeys := len(pc.l1Cache)
	pc.l1Mutex.RUnlock()

	return CacheStats{
		Hits:          pc.hits,
		Misses:        pc.misses,
		TotalRequests: pc.hits + pc.misses,
		TotalKeys:     totalKeys,
	}
}

// GetProduct busca un producto con caché multi-nivel
func (pc *ProductCache) GetProduct(ctx context.Context, codigoBarras string) (*models.ProductoCompleto, error) {
	start := time.Now()

	// 1. L1 Cache (Memoria local) - Más rápido
	if producto := pc.getFromL1(codigoBarras); producto != nil {
		pc.recordHit()
		pc.logger.Debug("L1 cache hit",
			zap.String("codigo_barras", codigoBarras),
			zap.Duration("latency", time.Since(start)))
		return producto, nil
	}

	// 2. L2 Cache (Redis) - Medio
	if producto, err := pc.getFromL2(ctx, codigoBarras); err == nil && producto != nil {
		// Mover a L1 cache para futuras consultas
		pc.setToL1(codigoBarras, producto)
		pc.recordHit()
		pc.logger.Debug("L2 cache hit",
			zap.String("codigo_barras", codigoBarras),
			zap.Duration("latency", time.Since(start)))
		return producto, nil
	}

	// 3. Database - Más lento (se implementará en el service)
	pc.recordMiss()
	pc.logger.Debug("Cache miss",
		zap.String("codigo_barras", codigoBarras),
		zap.Duration("latency", time.Since(start)))

	return nil, fmt.Errorf("producto no encontrado en caché")
}

// recordHit registra un hit en el caché
func (pc *ProductCache) recordHit() {
	pc.statsMutex.Lock()
	pc.hits++
	pc.statsMutex.Unlock()
}

// recordMiss registra un miss en el caché
func (pc *ProductCache) recordMiss() {
	pc.statsMutex.Lock()
	pc.misses++
	pc.statsMutex.Unlock()
}

// SetProduct almacena un producto en ambos niveles de caché
func (pc *ProductCache) SetProduct(ctx context.Context, codigoBarras string, producto *models.ProductoCompleto) error {
	// 1. L1 Cache (memoria local)
	pc.setToL1(codigoBarras, producto)

	// 2. L2 Cache (Redis)
	return pc.setToL2(ctx, codigoBarras, producto)
}

// InvalidateProduct invalida un producto en ambos cachés
func (pc *ProductCache) InvalidateProduct(ctx context.Context, codigoBarras string) error {
	// 1. L1 Cache
	pc.l1Mutex.Lock()
	delete(pc.l1Cache, codigoBarras)
	pc.l1Mutex.Unlock()

	// 2. L2 Cache
	return pc.redisClient.Del(ctx, fmt.Sprintf("product:%s", codigoBarras)).Err()
}

// PreloadProducts pre-carga productos frecuentes
func (pc *ProductCache) PreloadProducts(ctx context.Context, codigosBarras []string) error {
	for _, codigo := range codigosBarras {
		// Intentar obtener del caché primero
		if _, err := pc.GetProduct(ctx, codigo); err != nil {
			pc.logger.Debug("Producto no encontrado para preload", zap.String("codigo", codigo))
		}
	}
	return nil
}

// getFromL1 obtiene un producto del L1 cache (memoria local)
func (pc *ProductCache) getFromL1(codigoBarras string) *models.ProductoCompleto {
	pc.l1Mutex.RLock()
	defer pc.l1Mutex.RUnlock()
	return pc.l1Cache[codigoBarras]
}

// setToL1 almacena un producto en el L1 cache
func (pc *ProductCache) setToL1(codigoBarras string, producto *models.ProductoCompleto) {
	pc.l1Mutex.Lock()
	defer pc.l1Mutex.Unlock()

	// Verificar si necesitamos evictar
	if len(pc.l1Cache) >= pc.maxL1Size {
		pc.evictLRU()
	}

	pc.l1Cache[codigoBarras] = producto
}

// evictLRU elimina el elemento menos usado recientemente
func (pc *ProductCache) evictLRU() {
	// Implementación simple: eliminar el primer elemento
	for key := range pc.l1Cache {
		delete(pc.l1Cache, key)
		break
	}
}

// getFromL2 obtiene un producto del L2 cache (Redis)
func (pc *ProductCache) getFromL2(ctx context.Context, codigoBarras string) (*models.ProductoCompleto, error) {
	key := fmt.Sprintf("product:%s", codigoBarras)
	data, err := pc.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var producto models.ProductoCompleto
	if err := json.Unmarshal([]byte(data), &producto); err != nil {
		return nil, err
	}

	return &producto, nil
}

// setToL2 almacena un producto en el L2 cache (Redis)
func (pc *ProductCache) setToL2(ctx context.Context, codigoBarras string, producto *models.ProductoCompleto) error {
	key := fmt.Sprintf("product:%s", codigoBarras)
	data, err := json.Marshal(producto)
	if err != nil {
		return err
	}

	return pc.redisClient.Set(ctx, key, data, pc.ttl).Err()
}

// cleanupL1Cache limpia el L1 cache periódicamente
func (pc *ProductCache) cleanupL1Cache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		pc.l1Mutex.Lock()
		// Por ahora solo log, en el futuro podríamos implementar limpieza más inteligente
		pc.logger.Debug("L1 cache cleanup", zap.Int("items", len(pc.l1Cache)))
		pc.l1Mutex.Unlock()
	}
}

// Stats retorna estadísticas del caché (método legacy)
func (pc *ProductCache) Stats() map[string]interface{} {
	stats := pc.GetStats()
	return map[string]interface{}{
		"hits":           stats.Hits,
		"misses":         stats.Misses,
		"total_requests": stats.TotalRequests,
		"total_keys":     stats.TotalKeys,
		"hit_rate":       float64(stats.Hits) / float64(stats.TotalRequests),
	}
}
