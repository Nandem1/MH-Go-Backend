package handlers

import (
	"fmt"
	"net/http"
	"time"

	"stock-service/internal/cache"
	"stock-service/internal/models"
	"stock-service/internal/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// POSHandler maneja las operaciones específicas del POS
type POSHandler struct {
	productCache *cache.ProductCache
	stockService services.StockService
	logger       *zap.Logger
}

// NewPOSHandler crea una nueva instancia del handler POS
func NewPOSHandler(productCache *cache.ProductCache, stockService services.StockService, logger *zap.Logger) *POSHandler {
	return &POSHandler{
		productCache: productCache,
		stockService: stockService,
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
