package routes

import (
	"stock-service/internal/handlers"
	"stock-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configura todas las rutas de la aplicación
func SetupRoutes(router *gin.Engine, stockHandler *handlers.StockHandler, posHandler *handlers.POSHandler, monitoringHandler *handlers.MonitoringHandler, healthChecker *middleware.HealthChecker) {
	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// Stock routes
		stock := v1.Group("/stock")
		{
			// Operaciones múltiples (las más importantes)
			stock.POST("/entrada-multiple", stockHandler.EntradaMultipleStock)
			stock.POST("/salida-multiple", stockHandler.SalidaMultipleStock)

			// Consultas
			stock.GET("/local/:id", stockHandler.GetStockByLocal)
			stock.GET("/local-completo/:id", stockHandler.GetStockCompleteByLocal)
			stock.GET("/bajo/:id", stockHandler.GetStockBajo)
			stock.GET("/bajo-stock/:id", stockHandler.GetStockBajo) // Alias para compatibilidad
			stock.GET("/producto/:codigo", stockHandler.GetStockByProducto)
			stock.GET("/movimientos/:id", stockHandler.GetMovimientosByLocal) // Movimientos por local
			stock.GET("/reporte/:id", stockHandler.GetStockByLocal)           // Alias para reporte
		}

		// Movimientos routes (mantener para compatibilidad)
		movimientos := v1.Group("/movimientos")
		{
			movimientos.GET("", stockHandler.GetMovimientos)
		}

		// POS routes (ultra-rápido)
		pos := v1.Group("/pos")
		{
			pos.GET("/producto/:codigo", posHandler.SearchProductByBarcode)
			pos.POST("/venta-rapida", posHandler.QuickSale)
			pos.POST("/preload", posHandler.PreloadFrequentProducts)
			pos.GET("/cache-stats", posHandler.GetCacheStats)
			
			// Endpoints para invalidar cache
			pos.DELETE("/cache/producto/:codigo", posHandler.InvalidateProductCache)
			pos.DELETE("/cache/codigo-tivendo/:codigo", posHandler.InvalidateByCodigoTivendo)
			pos.DELETE("/cache/all", posHandler.InvalidateAllCache)
			pos.POST("/cache/invalidate", posHandler.InvalidateProductsCache)
			
			// Endpoint para notificar actualización masiva de lista_precios_cantera
			// Llamar desde el otro servidor después de actualizar ~9900 filas
			pos.POST("/cache/notify-lista-precios-update", posHandler.NotifyListaPreciosUpdate)
		}

		// Monitoring routes
		monitoring := v1.Group("/monitoring")
		{
			monitoring.GET("/metrics", monitoringHandler.GetMetrics)
			monitoring.GET("/metrics/summary", monitoringHandler.GetMetricsSummary)
			monitoring.GET("/ws", monitoringHandler.WebSocketMetrics)
		}
	}

	// Health check (mantener en raíz para compatibilidad)
	router.GET("/health", healthChecker.HealthCheck)
	router.GET("/health/monitoring", monitoringHandler.HealthCheck)

	// API info en raíz
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Stock Service API",
			"version": "1.0.0",
			"status":  "running",
			"endpoints": gin.H{
				"health": "/health",
				"api":    "/api/v1",
				"stock": gin.H{
					"entrada_multiple": "POST /api/v1/stock/entrada-multiple",
					"salida_multiple":  "POST /api/v1/stock/salida-multiple",
					"stock_local":      "GET /api/v1/stock/local/:id",
					"stock_bajo":       "GET /api/v1/stock/bajo/:id",
					"stock_producto":   "GET /api/v1/stock/producto/:codigo",
				},
				"movimientos": "GET /api/v1/movimientos",
			},
		})
	})
}
