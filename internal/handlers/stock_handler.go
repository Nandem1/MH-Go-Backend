package handlers

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"stock-service/internal/models"
	"stock-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// StockHandler maneja las peticiones HTTP relacionadas con stock
type StockHandler struct {
	stockService services.StockService
	validator    *validator.Validate
	logger       *zap.Logger
}

// NewStockHandler crea una nueva instancia del handler
func NewStockHandler(stockService services.StockService, logger *zap.Logger) *StockHandler {
	return &StockHandler{
		stockService: stockService,
		validator:    validator.New(),
		logger:       logger,
	}
}

// logDebug logs solo en modo debug
func (h *StockHandler) logDebug(msg string, fields ...zap.Field) {
	h.logger.Debug("🔍 [DEBUG] "+msg, fields...)
}

// logInfo logs en todos los modos
func (h *StockHandler) logInfo(msg string, fields ...zap.Field) {
	h.logger.Info("ℹ️ "+msg, fields...)
}

// logError logs errores en todos los modos
func (h *StockHandler) logError(msg string, fields ...zap.Field) {
	h.logger.Error("❌ "+msg, fields...)
}

// logSuccess logs de éxito en todos los modos
func (h *StockHandler) logSuccess(msg string, fields ...zap.Field) {
	h.logger.Info("✅ "+msg, fields...)
}

// EntradaMultipleStock maneja la entrada múltiple de stock
func (h *StockHandler) EntradaMultipleStock(c *gin.Context) {
	start := time.Now()

	h.logDebug("Iniciando entrada múltiple de stock")

	// Log del body raw para debugging (solo en debug)
	body, _ := c.GetRawData()
	h.logDebug("Raw body recibido", zap.String("body", string(body)))

	// Restaurar el body para que gin pueda leerlo
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var req models.EntradaMultipleStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logError("Error binding JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Error en el formato de datos",
			"error":   err.Error(),
		})
		return
	}

	h.logInfo("Entrada múltiple recibida",
		zap.Int("cantidad_productos", len(req.Productos)),
		zap.Int("id_local", req.IDLocal),
		zap.String("motivo", req.Motivo))

	// Validar request
	if err := h.validator.Struct(req); err != nil {
		h.logError("Validation error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Datos de entrada inválidos",
			"error":   err.Error(),
		})
		return
	}

	h.logDebug("Validación exitosa")

	// Log detallado de productos (solo en debug)
	for i, producto := range req.Productos {
		h.logDebug("Producto en request",
			zap.Int("index", i),
			zap.String("codigo_producto", producto.CodigoProducto),
			zap.String("tipo_item", producto.TipoItem),
			zap.Int("cantidad", producto.Cantidad),
			zap.Int("cantidad_minima", producto.CantidadMinima))
	}

	// TODO: Implementar autenticación cuando sea necesario
	// Por ahora usar ID por defecto
	req.IDUsuario = 1
	h.logDebug("ID Usuario asignado", zap.Int("id_usuario", req.IDUsuario))

	h.logDebug("Llamando a stockService.EntradaMultipleStock")

	// Procesar entrada múltiple
	response, err := h.stockService.EntradaMultipleStock(c.Request.Context(), &req)
	if err != nil {
		h.logError("Error procesando entrada múltiple", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error procesando entrada múltiple de stock",
			"error":   err.Error(),
		})
		return
	}

	h.logSuccess("Entrada múltiple completada",
		zap.Int("productos_procesados", response.TotalProductos),
		zap.Int("productos_fallidos", len(response.Errores)),
		zap.Bool("success", response.Success),
		zap.Duration("latency", time.Since(start)))

	// Log detallado de resultados (solo en debug)
	for i, resultado := range response.Resultados {
		h.logDebug("Resultado producto",
			zap.Int("index", i),
			zap.String("codigo_producto", resultado.CodigoProducto),
			zap.Int("cantidad_nueva", resultado.CantidadNueva),
			zap.Bool("success", resultado.Success))
	}

	// Log de errores si los hay (siempre visible)
	for i, error := range response.Errores {
		h.logError("Error en producto",
			zap.Int("index", i),
			zap.String("codigo_producto", error.CodigoProducto),
			zap.String("error", error.Error))
	}

	c.JSON(http.StatusOK, response)
}

// SalidaMultipleStock maneja la salida múltiple de stock
func (h *StockHandler) SalidaMultipleStock(c *gin.Context) {
	start := time.Now()

	h.logDebug("Iniciando salida múltiple de stock")

	// Log del body raw para debugging (solo en debug)
	body, _ := c.GetRawData()
	h.logDebug("Raw body recibido", zap.String("body", string(body)))

	// Restaurar el body para que gin pueda leerlo
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var req models.SalidaMultipleStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logError("Error binding JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Error en el formato de datos",
			"error":   err.Error(),
		})
		return
	}

	h.logInfo("Salida múltiple recibida",
		zap.Int("cantidad_productos", len(req.Productos)),
		zap.Int("id_local", req.IDLocal),
		zap.String("motivo", req.Motivo))

	// Validar request
	if err := h.validator.Struct(req); err != nil {
		h.logError("Validation error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Datos de entrada inválidos",
			"error":   err.Error(),
		})
		return
	}

	h.logDebug("Validación exitosa")

	// Log detallado de productos (solo en debug)
	for i, producto := range req.Productos {
		h.logDebug("Producto en request",
			zap.Int("index", i),
			zap.String("codigo_producto", producto.CodigoProducto),
			zap.String("tipo_item", producto.TipoItem),
			zap.Int("cantidad", producto.Cantidad))
	}

	// TODO: Implementar autenticación cuando sea necesario
	// Por ahora usar ID por defecto
	req.IDUsuario = 1
	h.logDebug("ID Usuario asignado", zap.Int("id_usuario", req.IDUsuario))

	h.logDebug("Llamando a stockService.SalidaMultipleStock")

	// Procesar salida múltiple
	response, err := h.stockService.SalidaMultipleStock(c.Request.Context(), &req)
	if err != nil {
		h.logError("Error procesando salida múltiple", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error procesando salida múltiple de stock",
			"error":   err.Error(),
		})
		return
	}

	h.logSuccess("Salida múltiple completada",
		zap.Int("productos_procesados", response.TotalProductos),
		zap.Int("productos_fallidos", len(response.Errores)),
		zap.Bool("success", response.Success),
		zap.Duration("latency", time.Since(start)))

	// Log detallado de resultados (solo en debug)
	for i, resultado := range response.Resultados {
		h.logDebug("Resultado producto",
			zap.Int("index", i),
			zap.String("codigo_producto", resultado.CodigoProducto),
			zap.Int("cantidad_nueva", resultado.CantidadNueva),
			zap.Bool("success", resultado.Success))
	}

	// Log de errores si los hay (siempre visible)
	for i, error := range response.Errores {
		h.logError("Error en producto",
			zap.Int("index", i),
			zap.String("codigo_producto", error.CodigoProducto),
			zap.String("error", error.Error))
	}

	c.JSON(http.StatusOK, response)
}

// GetStockByLocal obtiene el stock de un local específico
func (h *StockHandler) GetStockByLocal(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_stock_by_local"))

	idLocalStr := c.Param("id")
	idLocal, err := strconv.Atoi(idLocalStr)
	if err != nil {
		logger.Error("Error parsing local ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ ID de local inválido",
			"error":   "El ID debe ser un número válido",
		})
		return
	}

	logger.Info("Obteniendo stock por local", zap.Int("id_local", idLocal))

	stock, err := h.stockService.GetStockByLocal(c.Request.Context(), idLocal)
	if err != nil {
		logger.Error("Error obteniendo stock por local", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo stock del local",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Stock obtenido exitosamente",
		zap.Int("id_local", idLocal),
		zap.Int("cantidad_productos", len(stock)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Stock obtenido correctamente",
		"data": gin.H{
			"id_local":        idLocal,
			"productos":       stock,
			"total_productos": len(stock),
		},
	})
}

// GetStockBajo obtiene productos con stock bajo
func (h *StockHandler) GetStockBajo(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_stock_bajo"))

	idLocalStr := c.Param("id")
	idLocal, err := strconv.Atoi(idLocalStr)
	if err != nil {
		logger.Error("Error parsing local ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ ID de local inválido",
			"error":   "El ID debe ser un número válido",
		})
		return
	}

	logger.Info("Obteniendo stock bajo", zap.Int("id_local", idLocal))

	stockBajo, err := h.stockService.GetStockBajo(c.Request.Context(), idLocal)
	if err != nil {
		logger.Error("Error obteniendo stock bajo", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo stock bajo",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Stock bajo obtenido exitosamente",
		zap.Int("id_local", idLocal),
		zap.Int("productos_bajo_stock", len(stockBajo)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Productos con stock bajo obtenidos",
		"data": gin.H{
			"id_local":             idLocal,
			"productos_bajo_stock": stockBajo,
			"total_productos_bajo": len(stockBajo),
		},
	})
}

// GetStockByProducto obtiene el stock de un producto específico
func (h *StockHandler) GetStockByProducto(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_stock_by_producto"))

	codigoProducto := c.Param("codigo")
	if codigoProducto == "" {
		logger.Error("Código de producto vacío")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Código de producto requerido",
			"error":   "El código de producto no puede estar vacío",
		})
		return
	}

	idLocalStr := c.Query("local")
	idLocal := 1 // Default local
	if idLocalStr != "" {
		if parsed, err := strconv.Atoi(idLocalStr); err == nil {
			idLocal = parsed
		}
	}

	logger.Info("Obteniendo stock por producto",
		zap.String("codigo_producto", codigoProducto),
		zap.Int("id_local", idLocal))

	stock, err := h.stockService.GetStockByProducto(c.Request.Context(), codigoProducto, idLocal)
	if err != nil {
		logger.Error("Error obteniendo stock por producto", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo stock del producto",
			"error":   err.Error(),
		})
		return
	}

	if stock == nil {
		logger.Info("Producto sin stock", zap.String("codigo_producto", codigoProducto))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "✅ Producto sin stock disponible",
			"data": gin.H{
				"codigo_producto": codigoProducto,
				"id_local":        idLocal,
				"stock":           nil,
			},
		})
		return
	}

	logger.Info("Stock obtenido exitosamente",
		zap.String("codigo_producto", codigoProducto),
		zap.Int("cantidad_actual", stock.CantidadActual))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Stock obtenido correctamente",
		"data": gin.H{
			"codigo_producto": codigoProducto,
			"id_local":        idLocal,
			"stock":           stock,
		},
	})
}

// GetMovimientos obtiene el historial de movimientos
func (h *StockHandler) GetMovimientos(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_movimientos"))

	// Parsear parámetros de query
	idLocalStr := c.Query("local")
	tipoMovimiento := c.Query("tipo")
	fechaDesdeStr := c.Query("fecha_desde")
	fechaHastaStr := c.Query("fecha_hasta")

	filter := &models.MovimientoFilter{}

	if idLocalStr != "" {
		if idLocal, err := strconv.Atoi(idLocalStr); err == nil {
			filter.IDLocal = &idLocal
		}
	}

	if tipoMovimiento != "" {
		filter.TipoMovimiento = &tipoMovimiento
	}

	// Parsear fechas
	if fechaDesdeStr != "" {
		if fechaDesde, err := time.Parse("2006-01-02", fechaDesdeStr); err == nil {
			filter.FechaDesde = &fechaDesde
		}
	}

	if fechaHastaStr != "" {
		if fechaHasta, err := time.Parse("2006-01-02", fechaHastaStr); err == nil {
			filter.FechaHasta = &fechaHasta
		}
	}

	logger.Info("Obteniendo movimientos",
		zap.Any("filtros", filter))

	movimientos, err := h.stockService.GetMovimientosByLocal(c.Request.Context(), filter)
	if err != nil {
		logger.Error("Error obteniendo movimientos", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo movimientos",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Movimientos obtenidos exitosamente",
		zap.Int("total_movimientos", len(movimientos)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Movimientos obtenidos correctamente",
		"data": gin.H{
			"movimientos": movimientos,
			"total":       len(movimientos),
			"filtros":     filter,
		},
	})
}

// GetMovimientosByLocal obtiene movimientos de un local específico (con parámetro en URL)
func (h *StockHandler) GetMovimientosByLocal(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "get_movimientos_by_local"))

	// Obtener ID del local desde la URL
	idLocalStr := c.Param("id")
	idLocal, err := strconv.Atoi(idLocalStr)
	if err != nil {
		logger.Error("Error parsing local ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ ID de local inválido",
			"error":   "El ID debe ser un número válido",
		})
		return
	}

	// Parsear otros parámetros de query
	tipoMovimiento := c.Query("tipo")
	fechaDesdeStr := c.Query("fecha_desde")
	fechaHastaStr := c.Query("fecha_hasta")

	filter := &models.MovimientoFilter{
		IDLocal: &idLocal,
	}

	if tipoMovimiento != "" {
		filter.TipoMovimiento = &tipoMovimiento
	}

	// Parsear fechas
	if fechaDesdeStr != "" {
		if fechaDesde, err := time.Parse("2006-01-02", fechaDesdeStr); err == nil {
			filter.FechaDesde = &fechaDesde
		}
	}

	if fechaHastaStr != "" {
		if fechaHasta, err := time.Parse("2006-01-02", fechaHastaStr); err == nil {
			filter.FechaHasta = &fechaHasta
		}
	}

	logger.Info("Obteniendo movimientos por local",
		zap.Int("id_local", idLocal),
		zap.Any("filtros", filter))

	movimientos, err := h.stockService.GetMovimientosByLocal(c.Request.Context(), filter)
	if err != nil {
		logger.Error("Error obteniendo movimientos por local", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "❌ Error obteniendo movimientos",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Movimientos por local obtenidos exitosamente",
		zap.Int("id_local", idLocal),
		zap.Int("total_movimientos", len(movimientos)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "✅ Movimientos obtenidos correctamente",
		"data": gin.H{
			"id_local":    idLocal,
			"movimientos": movimientos,
			"total":       len(movimientos),
			"filtros":     filter,
		},
	})
}
