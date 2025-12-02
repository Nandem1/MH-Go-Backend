package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"stock-service/internal/cache"
	"stock-service/internal/models"
	"stock-service/internal/repository"
	"stock-service/internal/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// POSHandler maneja las operaciones específicas del POS
type POSHandler struct {
	productCache *cache.ProductCache
	stockService services.StockService
	productRepo  repository.ProductRepository
	logger       *zap.Logger
}

// NewPOSHandler crea una nueva instancia del handler POS
func NewPOSHandler(productCache *cache.ProductCache, stockService services.StockService, productRepo repository.ProductRepository, logger *zap.Logger) *POSHandler {
	return &POSHandler{
		productCache: productCache,
		stockService: stockService,
		productRepo:  productRepo,
		logger:       logger,
	}
}

// SearchProductByBarcode busca un producto por código de barras (ultra-rápido)
func (h *POSHandler) SearchProductByBarcode(c *gin.Context) {
	start := time.Now()
	codigoBarras := c.Param("codigo")

	if codigoBarras == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Código de barras requerido",
			"error":   "El código de barras no puede estar vacío",
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "search_product_barcode"),
		zap.String("codigo_barras", codigoBarras),
	)

	logger.Info("Buscando producto por código de barras")

	// 0. Validar versiones globales (solo consulta a Redis, ultra-rápida)
	// Solo consulta PostgreSQL si detecta que la versión puede haber cambiado
	if err := h.validateGlobalVersion(c.Request.Context()); err != nil {
		logger.Warn("Error validando versión global de lista_precios, continuando con cache",
			zap.Error(err))
	}
	if err := h.validateProductosVersion(c.Request.Context()); err != nil {
		logger.Warn("Error validando versión global de productos, continuando con cache",
			zap.Error(err))
	}

	// 1. Buscar en caché multi-nivel (ultra-rápido)
	producto, err := h.productCache.GetProduct(c.Request.Context(), codigoBarras)
	if err == nil && producto != nil {
		// Producto encontrado en caché
		logger.Info("Producto encontrado en caché",
			zap.String("nombre", producto.Nombre),
			zap.Duration("latency", time.Since(start)))

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Producto encontrado",
			"data": gin.H{
				"producto":   producto,
				"cache_hit":  true,
				"latency_ms": time.Since(start).Milliseconds(),
			},
		})
		return
	}

	// 2. Buscar en base de datos (más lento)
	logger.Info("Producto no encontrado en caché, buscando en base de datos")

	producto, err = h.stockService.GetProductoByBarcode(c.Request.Context(), codigoBarras)
	if err != nil {
		logger.Warn("Producto no encontrado en base de datos",
			zap.String("codigo_barras", codigoBarras),
			zap.Duration("latency", time.Since(start)),
			zap.Error(err))

		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "❌ Producto no encontrado",
			"error":   "El producto no existe en el sistema",
			"data": gin.H{
				"codigo_barras": codigoBarras,
				"cache_hit":     false,
				"latency_ms":    time.Since(start).Milliseconds(),
			},
		})
		return
	}

	// Producto encontrado en base de datos, cachearlo para futuras consultas
	if err := h.productCache.SetProduct(c.Request.Context(), codigoBarras, producto); err != nil {
		logger.Error("Error cacheando producto", zap.Error(err))
	}

	logger.Info("Producto encontrado en base de datos",
		zap.String("nombre", producto.Nombre),
		zap.String("origen", producto.Origen),
		zap.Duration("latency", time.Since(start)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Producto encontrado",
		"data": gin.H{
			"producto":   producto,
			"cache_hit":  false,
			"latency_ms": time.Since(start).Milliseconds(),
		},
	})
}

// QuickSale registra una venta rápida (estilo POS)
func (h *POSHandler) QuickSale(c *gin.Context) {
	start := time.Now()

	var req models.QuickSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Error en el formato de datos",
			"error":   err.Error(),
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "quick_sale"),
		zap.Int("cantidad_items", len(req.Items)),
		zap.Int("id_local", req.IDLocal),
	)

	logger.Info("Procesando venta rápida")

	// Validar que todos los productos existan y tengan stock
	var itemsValidos []models.ProductoStock
	var errores []string

	for i, item := range req.Items {
		// Buscar producto en caché
		producto, err := h.productCache.GetProduct(c.Request.Context(), item.CodigoProducto)
		if err != nil || producto == nil {
			errorMsg := fmt.Sprintf("Item %d: Producto %s no encontrado", i+1, item.CodigoProducto)
			errores = append(errores, errorMsg)
			continue
		}

		// Verificar stock disponible
		stock, err := h.stockService.GetStockByProducto(c.Request.Context(), item.CodigoProducto, req.IDLocal)
		if err != nil || stock == nil {
			errorMsg := fmt.Sprintf("Item %d: No hay stock disponible para %s", i+1, item.CodigoProducto)
			errores = append(errores, errorMsg)
			continue
		}

		if stock.CantidadActual < item.Cantidad {
			errorMsg := fmt.Sprintf("Item %d: Stock insuficiente para %s (disponible: %d, solicitado: %d)",
				i+1, item.CodigoProducto, stock.CantidadActual, item.Cantidad)
			errores = append(errores, errorMsg)
			continue
		}

		itemsValidos = append(itemsValidos, item)
	}

	// Si hay errores, retornar lista de problemas
	if len(errores) > 0 {
		logger.Warn("Errores en venta rápida", zap.Strings("errores", errores))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Errores en la venta",
			"errors":  errores,
			"data": gin.H{
				"items_validos":   len(itemsValidos),
				"items_invalidos": len(req.Items) - len(itemsValidos),
				"latency_ms":      time.Since(start).Milliseconds(),
			},
		})
		return
	}

	// Procesar venta con items válidos
	// Convertir ProductoStock a ProductoSalida
	var productosSalida []models.ProductoSalida
	for _, item := range itemsValidos {
		productosSalida = append(productosSalida, models.ProductoSalida{
			CodigoProducto: item.CodigoProducto,
			TipoItem:       item.TipoItem,
			Cantidad:       item.Cantidad,
		})
	}

	salidaReq := &models.SalidaMultipleStockRequest{
		Productos:     productosSalida,
		Motivo:        req.Motivo,
		IDLocal:       req.IDLocal,
		Observaciones: req.Observaciones,
	}

	// TODO: Implementar autenticación cuando sea necesario
	// Por ahora usar ID por defecto
	salidaReq.IDUsuario = 1

	response, err := h.stockService.SalidaMultipleStock(c.Request.Context(), salidaReq)
	if err != nil {
		logger.Error("Error procesando venta rápida", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error procesando venta",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Venta rápida completada",
		zap.Int("productos_procesados", response.TotalProductos),
		zap.Duration("latency", time.Since(start)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Venta procesada correctamente",
		"data": gin.H{
			"venta_id":             time.Now().Unix(), // ID temporal
			"productos_procesados": response.TotalProductos,
			"total_items":          len(itemsValidos),
			"latency_ms":           time.Since(start).Milliseconds(),
			"timestamp":            time.Now().Format(time.RFC3339),
		},
	})
}

// PreloadFrequentProducts pre-carga productos frecuentes
func (h *POSHandler) PreloadFrequentProducts(c *gin.Context) {
	var req struct {
		CodigosBarras []string `json:"codigos_barras" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Error en el formato de datos",
			"error":   err.Error(),
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "preload_products"),
		zap.Int("cantidad_codigos", len(req.CodigosBarras)),
	)

	logger.Info("Pre-cargando productos frecuentes")

	// Pre-cargar productos en caché
	err := h.productCache.PreloadProducts(c.Request.Context(), req.CodigosBarras)
	if err != nil {
		logger.Error("Error pre-cargando productos", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error pre-cargando productos",
			"error":   err.Error(),
		})
		return
	}

	// Obtener estadísticas del caché
	stats := h.productCache.Stats()

	logger.Info("Productos pre-cargados exitosamente")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Productos pre-cargados correctamente",
		"data": gin.H{
			"codigos_procesados": len(req.CodigosBarras),
			"cache_stats":        stats,
			"timestamp":          time.Now().Format(time.RFC3339),
		},
	})
}

// GetCacheStats obtiene estadísticas del caché
func (h *POSHandler) GetCacheStats(c *gin.Context) {
	stats := h.productCache.Stats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Estadísticas del caché",
		"data":    stats,
	})
}

// InvalidateProductCache invalida la cache de un producto por código de barras
func (h *POSHandler) InvalidateProductCache(c *gin.Context) {
	codigoBarras := c.Param("codigo")

	if codigoBarras == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Código de barras requerido",
			"error":   "El código de barras no puede estar vacío",
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "invalidate_product_cache"),
		zap.String("codigo_barras", codigoBarras),
	)

	logger.Info("Invalidando cache de producto")

	if err := h.productCache.InvalidateProduct(c.Request.Context(), codigoBarras); err != nil {
		logger.Error("Error invalidando cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Cache invalidada correctamente",
		"data": gin.H{
			"codigo_barras": codigoBarras,
		},
	})
}

// InvalidateByCodigoTivendo invalida la cache de productos por código_tivendo
// Útil cuando se actualiza la tabla lista_precios_cantera
func (h *POSHandler) InvalidateByCodigoTivendo(c *gin.Context) {
	codigoTivendo := c.Param("codigo")

	if codigoTivendo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Código Tivendo requerido",
			"error":   "El código Tivendo no puede estar vacío",
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "invalidate_by_codigo_tivendo"),
		zap.String("codigo_tivendo", codigoTivendo),
	)

	logger.Info("Invalidando cache por código Tivendo")

	if err := h.productCache.InvalidateByCodigoTivendo(c.Request.Context(), codigoTivendo); err != nil {
		logger.Error("Error invalidando cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Cache invalidada correctamente",
		"data": gin.H{
			"codigo_tivendo": codigoTivendo,
		},
	})
}

// InvalidateAllCache invalida toda la cache de productos
// Útil cuando se actualiza masivamente la tabla lista_precios_cantera
func (h *POSHandler) InvalidateAllCache(c *gin.Context) {
	logger := h.logger.With(
		zap.String("handler", "invalidate_all_cache"),
	)

	logger.Info("Invalidando toda la cache de productos")

	if err := h.productCache.InvalidateAll(c.Request.Context()); err != nil {
		logger.Error("Error invalidando cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	stats := h.productCache.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Cache invalidada completamente",
		"data": gin.H{
			"cache_stats": stats,
		},
	})
}

// InvalidateProductsCache invalida múltiples productos por códigos de barras
func (h *POSHandler) InvalidateProductsCache(c *gin.Context) {
	var req struct {
		CodigosBarras []string `json:"codigos_barras" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Error en el formato de datos",
			"error":   err.Error(),
		})
		return
	}

	logger := h.logger.With(
		zap.String("handler", "invalidate_products_cache"),
		zap.Int("cantidad", len(req.CodigosBarras)),
	)

	logger.Info("Invalidando cache de múltiples productos")

	if err := h.productCache.InvalidateProducts(c.Request.Context(), req.CodigosBarras); err != nil {
		logger.Error("Error invalidando cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Cache invalidada correctamente",
		"data": gin.H{
			"codigos_invalidados": len(req.CodigosBarras),
		},
	})
}

// NotifyProductosUpdate notifica que se actualizaron productos/packs masivamente
// Este endpoint debe ser llamado desde el otro servidor después de actualizar productos
func (h *POSHandler) NotifyProductosUpdate(c *gin.Context) {
	logger := h.logger.With(
		zap.String("handler", "notify_productos_update"),
	)

	logger.Info("Notificación de actualización masiva de productos/packs")

	// Obtener el último timestamp de la BD
	timestamp, err := h.productRepo.GetLastProductosTimestamp(c.Request.Context())
	if err != nil {
		logger.Error("Error obteniendo timestamp de productos", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo timestamp",
			"error":   err.Error(),
		})
		return
	}

	if timestamp == nil {
		logger.Warn("No se encontró timestamp de productos")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "⚠️ No hay timestamp disponible",
		})
		return
	}

	// Convertir timestamp a string para usar como versión
	version := timestamp.Format(time.RFC3339Nano)

	// Invalidar cache si la versión cambió
	invalidated, err := h.productCache.InvalidateAllByProductosVersion(c.Request.Context(), version)
	if err != nil {
		logger.Error("Error invalidando cache por versión", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	if invalidated {
		logger.Info("Cache invalidada por actualización masiva de productos",
			zap.String("version", version))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Cache invalidada correctamente",
			"data": gin.H{
				"version":     version,
				"invalidated": true,
			},
		})
	} else {
		logger.Info("Cache ya estaba actualizada",
			zap.String("version", version))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Cache ya está actualizada",
			"data": gin.H{
				"version":     version,
				"invalidated": false,
			},
		})
	}
}

// NotifyListaPreciosUpdate notifica que se actualizó lista_precios_cantera masivamente
// Este endpoint debe ser llamado desde el otro servidor después de actualizar ~9900 filas
func (h *POSHandler) NotifyListaPreciosUpdate(c *gin.Context) {
	logger := h.logger.With(
		zap.String("handler", "notify_lista_precios_update"),
	)

	logger.Info("Notificación de actualización masiva de lista_precios_cantera")

	// Obtener el último timestamp de la BD
	timestamp, err := h.productRepo.GetLastListaPreciosTimestamp(c.Request.Context())
	if err != nil {
		logger.Error("Error obteniendo timestamp de lista_precios", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo timestamp",
			"error":   err.Error(),
		})
		return
	}

	if timestamp == nil {
		logger.Warn("No se encontró timestamp de lista_precios")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "⚠️ No hay timestamp disponible",
		})
		return
	}

	// Convertir timestamp a string para usar como versión
	version := timestamp.Format(time.RFC3339Nano)

	// Invalidar cache si la versión cambió
	invalidated, err := h.productCache.InvalidateAllByVersion(c.Request.Context(), version)
	if err != nil {
		logger.Error("Error invalidando cache por versión", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error invalidando cache",
			"error":   err.Error(),
		})
		return
	}

	if invalidated {
		logger.Info("Cache invalidada por actualización masiva",
			zap.String("version", version))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Cache invalidada correctamente",
			"data": gin.H{
				"version":     version,
				"invalidated": true,
			},
		})
	} else {
		logger.Info("Cache ya estaba actualizada",
			zap.String("version", version))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Cache ya está actualizada",
			"data": gin.H{
				"version":     version,
				"invalidated": false,
			},
		})
	}
}

// validateGlobalVersion valida la versión global de lista_precios_cantera
// Optimizado: primero consulta Redis (ultra-rápido), solo consulta BD si es necesario
func (h *POSHandler) validateGlobalVersion(ctx context.Context) error {
	// 1. Primero obtener versión actual de Redis (ultra-rápido, ~0.1ms)
	currentVersion, err := h.productCache.GetGlobalVersion(ctx)
	if err != nil {
		// Si hay error con Redis, no bloquear, continuar con cache
		return err
	}

	// 2. Si no hay versión en Redis, obtener de BD y guardar (solo primera vez)
	if currentVersion == "" {
		timestamp, err := h.productRepo.GetLastListaPreciosTimestamp(ctx)
		if err != nil {
			return err
		}
		if timestamp != nil {
			version := timestamp.Format(time.RFC3339Nano)
			return h.productCache.SetGlobalVersion(ctx, version)
		}
		return nil
	}

	// 3. Si hay versión en Redis, verificar BD solo si pasó el intervalo
	// Esto evita sobrecargar PostgreSQL con consultas en cada request
	shouldCheck, err := h.productCache.ShouldCheckDatabase(ctx)
	if err != nil {
		// Error verificando, continuar sin bloquear
		return err
	}

	if !shouldCheck {
		// Aún no es tiempo de verificar, solo usar Redis (ultra-rápido)
		return nil
	}

	// Es tiempo de verificar BD (solo cada 10 segundos aproximadamente)
	timestamp, err := h.productRepo.GetLastListaPreciosTimestamp(ctx)
	if err != nil {
		return err
	}

	// Actualizar timestamp de última verificación
	h.productCache.UpdateLastCheck(ctx)

	if timestamp == nil {
		return nil
	}

	newVersion := timestamp.Format(time.RFC3339Nano)

	// Solo invalidar si la versión cambió
	if currentVersion != newVersion {
		_, err = h.productCache.InvalidateAllByVersion(ctx, newVersion)
		return err
	}

	return nil
}

// validateProductosVersion valida la versión global de productos/packs
// Optimizado: primero consulta Redis (ultra-rápido), solo consulta BD si es necesario
func (h *POSHandler) validateProductosVersion(ctx context.Context) error {
	// 1. Primero obtener versión actual de Redis (ultra-rápido, ~0.1ms)
	currentVersion, err := h.productCache.GetProductosVersion(ctx)
	if err != nil {
		// Si hay error con Redis, no bloquear, continuar con cache
		return err
	}

	// 2. Si no hay versión en Redis, obtener de BD y guardar (solo primera vez)
	if currentVersion == "" {
		timestamp, err := h.productRepo.GetLastProductosTimestamp(ctx)
		if err != nil {
			return err
		}
		if timestamp != nil {
			version := timestamp.Format(time.RFC3339Nano)
			return h.productCache.SetProductosVersion(ctx, version)
		}
		return nil
	}

	// 3. Si hay versión en Redis, verificar BD solo si pasó el intervalo
	// Esto evita sobrecargar PostgreSQL con consultas en cada request
	shouldCheck, err := h.productCache.ShouldCheckProductosDatabase(ctx)
	if err != nil {
		// Error verificando, continuar sin bloquear
		return err
	}

	if !shouldCheck {
		// Aún no es tiempo de verificar, solo usar Redis (ultra-rápido)
		return nil
	}

	// Es tiempo de verificar BD (solo cada 10 segundos aproximadamente)
	timestamp, err := h.productRepo.GetLastProductosTimestamp(ctx)
	if err != nil {
		return err
	}

	// Actualizar timestamp de última verificación
	h.productCache.UpdateProductosLastCheck(ctx)

	if timestamp == nil {
		return nil
	}

	newVersion := timestamp.Format(time.RFC3339Nano)

	// Solo invalidar si la versión cambió
	if currentVersion != newVersion {
		_, err = h.productCache.InvalidateAllByProductosVersion(ctx, newVersion)
		return err
	}

	return nil
}
