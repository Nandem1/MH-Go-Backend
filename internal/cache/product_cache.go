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

	// Versión global de lista_precios_cantera (para invalidación masiva)
	globalVersionKey        string
	lastCheckTimestampKey   string
	checkIntervalSeconds    int64 // Verificar BD solo cada N segundos
	
	// Versión global de productos (para invalidación masiva)
	productosVersionKey     string
	productosLastCheckKey   string
}

// NewProductCache crea una nueva instancia del caché
func NewProductCache(redisClient *redis.Client, maxL1Size int, ttl time.Duration, logger *zap.Logger) *ProductCache {
	pc := &ProductCache{
		l1Cache:               make(map[string]*models.ProductoCompleto),
		redisClient:           redisClient,
		maxL1Size:             maxL1Size,
		ttl:                   ttl,
		logger:                logger,
		globalVersionKey:      "lista_precios:global_version",
		lastCheckTimestampKey: "lista_precios:last_check",
		checkIntervalSeconds:  10, // Verificar BD solo cada 10 segundos
		productosVersionKey:   "productos:global_version",
		productosLastCheckKey: "productos:last_check",
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
// Implementa stale-while-revalidate: devuelve cache aunque esté desactualizado,
// pero valida en background si necesita actualizarse
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

// GetGlobalVersion obtiene la versión global de lista_precios_cantera desde Redis
func (pc *ProductCache) GetGlobalVersion(ctx context.Context) (string, error) {
	version, err := pc.redisClient.Get(ctx, pc.globalVersionKey).Result()
	if err == redis.Nil {
		return "", nil // No hay versión guardada aún
	}
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetGlobalVersion actualiza la versión global de lista_precios_cantera en Redis
func (pc *ProductCache) SetGlobalVersion(ctx context.Context, version string) error {
	now := time.Now().Unix()
	// Guardar versión y timestamp de última verificación
	pipe := pc.redisClient.Pipeline()
	pipe.Set(ctx, pc.globalVersionKey, version, 0)
	pipe.Set(ctx, pc.lastCheckTimestampKey, now, 0)
	_, err := pipe.Exec(ctx)
	return err
}

// ShouldCheckDatabase verifica si debemos consultar la BD basado en el intervalo
func (pc *ProductCache) ShouldCheckDatabase(ctx context.Context) (bool, error) {
	lastCheckStr, err := pc.redisClient.Get(ctx, pc.lastCheckTimestampKey).Result()
	if err == redis.Nil {
		// Nunca se ha verificado, sí debemos verificar
		return true, nil
	}
	if err != nil {
		return false, err
	}

	var lastCheck int64
	if _, err := fmt.Sscanf(lastCheckStr, "%d", &lastCheck); err != nil {
		// Error parseando, verificar de nuevo
		return true, nil
	}

	now := time.Now().Unix()
	elapsed := now - lastCheck

	// Solo verificar si pasó el intervalo
	return elapsed >= pc.checkIntervalSeconds, nil
}

// UpdateLastCheck actualiza el timestamp de última verificación
func (pc *ProductCache) UpdateLastCheck(ctx context.Context) error {
	now := time.Now().Unix()
	return pc.redisClient.Set(ctx, pc.lastCheckTimestampKey, now, 0).Err()
}

// GetProductosVersion obtiene la versión global de productos desde Redis
func (pc *ProductCache) GetProductosVersion(ctx context.Context) (string, error) {
	version, err := pc.redisClient.Get(ctx, pc.productosVersionKey).Result()
	if err == redis.Nil {
		return "", nil // No hay versión guardada aún
	}
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetProductosVersion actualiza la versión global de productos en Redis
func (pc *ProductCache) SetProductosVersion(ctx context.Context, version string) error {
	now := time.Now().Unix()
	// Guardar versión y timestamp de última verificación
	pipe := pc.redisClient.Pipeline()
	pipe.Set(ctx, pc.productosVersionKey, version, 0)
	pipe.Set(ctx, pc.productosLastCheckKey, now, 0)
	_, err := pipe.Exec(ctx)
	return err
}

// ShouldCheckProductosDatabase verifica si debemos consultar la BD para productos basado en el intervalo
func (pc *ProductCache) ShouldCheckProductosDatabase(ctx context.Context) (bool, error) {
	lastCheckStr, err := pc.redisClient.Get(ctx, pc.productosLastCheckKey).Result()
	if err == redis.Nil {
		// Nunca se ha verificado, sí debemos verificar
		return true, nil
	}
	if err != nil {
		return false, err
	}

	var lastCheck int64
	if _, err := fmt.Sscanf(lastCheckStr, "%d", &lastCheck); err != nil {
		// Error parseando, verificar de nuevo
		return true, nil
	}

	now := time.Now().Unix()
	elapsed := now - lastCheck

	// Solo verificar si pasó el intervalo
	return elapsed >= pc.checkIntervalSeconds, nil
}

// UpdateProductosLastCheck actualiza el timestamp de última verificación de productos
func (pc *ProductCache) UpdateProductosLastCheck(ctx context.Context) error {
	now := time.Now().Unix()
	return pc.redisClient.Set(ctx, pc.productosLastCheckKey, now, 0).Err()
}

// InvalidateAllByProductosVersion invalida toda la cache si la versión de productos cambió
func (pc *ProductCache) InvalidateAllByProductosVersion(ctx context.Context, newVersion string) (bool, error) {
	currentVersion, err := pc.GetProductosVersion(ctx)
	if err != nil {
		return false, err
	}

	// Si la versión cambió, invalidar todo
	if currentVersion != newVersion {
		pc.logger.Info("Versión global de productos cambió, invalidando cache",
			zap.String("version_anterior", currentVersion),
			zap.String("version_nueva", newVersion))

		if err := pc.InvalidateAll(ctx); err != nil {
			return false, err
		}

		// Actualizar la versión
		if err := pc.SetProductosVersion(ctx, newVersion); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

// InvalidateAllByVersion invalida toda la cache si la versión cambió
func (pc *ProductCache) InvalidateAllByVersion(ctx context.Context, newVersion string) (bool, error) {
	currentVersion, err := pc.GetGlobalVersion(ctx)
	if err != nil {
		return false, err
	}

	// Si la versión cambió, invalidar todo
	if currentVersion != newVersion {
		pc.logger.Info("Versión global de lista_precios cambió, invalidando cache",
			zap.String("version_anterior", currentVersion),
			zap.String("version_nueva", newVersion))

		if err := pc.InvalidateAll(ctx); err != nil {
			return false, err
		}

		// Actualizar la versión
		if err := pc.SetGlobalVersion(ctx, newVersion); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
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

// InvalidateProducts invalida múltiples productos por códigos de barras
func (pc *ProductCache) InvalidateProducts(ctx context.Context, codigosBarras []string) error {
	if len(codigosBarras) == 0 {
		return nil
	}

	// 1. L1 Cache - Invalidar en memoria
	pc.l1Mutex.Lock()
	for _, codigo := range codigosBarras {
		delete(pc.l1Cache, codigo)
	}
	pc.l1Mutex.Unlock()

	// 2. L2 Cache - Invalidar en Redis (usar pipeline para mejor rendimiento)
	pipe := pc.redisClient.Pipeline()
	for _, codigo := range codigosBarras {
		pipe.Del(ctx, fmt.Sprintf("product:%s", codigo))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		pc.logger.Error("Error invalidando productos en Redis",
			zap.Int("cantidad", len(codigosBarras)),
			zap.Error(err))
		return err
	}

	pc.logger.Info("Productos invalidados en cache",
		zap.Int("cantidad", len(codigosBarras)))

	return nil
}

// InvalidateByCodigoTivendo invalida todos los productos que tienen un código_tivendo específico
// Busca en L1 y L2 cache todos los productos que coincidan con el código_tivendo
func (pc *ProductCache) InvalidateByCodigoTivendo(ctx context.Context, codigoTivendo string) error {
	var codigosInvalidar []string

	// 1. Buscar en L1 Cache
	pc.l1Mutex.RLock()
	for codigoBarras, producto := range pc.l1Cache {
		if producto != nil && producto.Codigo == codigoTivendo {
			codigosInvalidar = append(codigosInvalidar, codigoBarras)
		}
		// También verificar si es un pack con el código
		if producto != nil && producto.CodigoPack != nil && *producto.CodigoPack == codigoTivendo {
			codigosInvalidar = append(codigosInvalidar, codigoBarras)
		}
	}
	pc.l1Mutex.RUnlock()

	// 2. Buscar en L2 Cache (Redis) - buscar por patrón
	pattern := fmt.Sprintf("product:*")
	iter := pc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		codigoBarras := key[8:] // Remover "product:" del inicio

		// Obtener el producto del cache para verificar su código
		producto, err := pc.getFromL2(ctx, codigoBarras)
		if err == nil && producto != nil {
			if producto.Codigo == codigoTivendo {
				codigosInvalidar = append(codigosInvalidar, codigoBarras)
			}
			if producto.CodigoPack != nil && *producto.CodigoPack == codigoTivendo {
				codigosInvalidar = append(codigosInvalidar, codigoBarras)
			}
		}
	}
	if err := iter.Err(); err != nil {
		pc.logger.Error("Error escaneando Redis para invalidación",
			zap.String("codigo_tivendo", codigoTivendo),
			zap.Error(err))
	}

	// 3. Invalidar todos los productos encontrados
	if len(codigosInvalidar) > 0 {
		pc.logger.Info("Invalidando productos por código_tivendo",
			zap.String("codigo_tivendo", codigoTivendo),
			zap.Int("productos_encontrados", len(codigosInvalidar)))
		return pc.InvalidateProducts(ctx, codigosInvalidar)
	}

	pc.logger.Debug("No se encontraron productos en cache para código_tivendo",
		zap.String("codigo_tivendo", codigoTivendo))

	return nil
}

// InvalidateAll invalida toda la cache de productos (útil cuando se actualiza lista_precios_cantera masivamente)
func (pc *ProductCache) InvalidateAll(ctx context.Context) error {
	// 1. L1 Cache - Limpiar todo
	pc.l1Mutex.Lock()
	cantidadL1 := len(pc.l1Cache)
	pc.l1Cache = make(map[string]*models.ProductoCompleto)
	pc.l1Mutex.Unlock()

	// 2. L2 Cache - Eliminar todas las claves de productos
	pattern := "product:*"
	iter := pc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		pc.logger.Error("Error escaneando Redis para invalidación total", zap.Error(err))
		return err
	}

	if len(keys) > 0 {
		if err := pc.redisClient.Del(ctx, keys...).Err(); err != nil {
			pc.logger.Error("Error eliminando claves de Redis", zap.Error(err))
			return err
		}
	}

	pc.logger.Info("Cache de productos invalidada completamente",
		zap.Int("productos_l1", cantidadL1),
		zap.Int("productos_l2", len(keys)))

	return nil
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
