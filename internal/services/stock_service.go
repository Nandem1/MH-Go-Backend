package services

import (
	"context"
	"fmt"
	"time"

	"stock-service/internal/models"
	"stock-service/internal/repository"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// StockService define la interfaz para operaciones de stock
type StockService interface {
	// Operaciones básicas
	EntradaStock(ctx context.Context, req *models.EntradaStockRequest) (*models.EntradaStockResponse, error)
	SalidaStock(ctx context.Context, req *models.SalidaStockRequest) (*models.SalidaStockResponse, error)

	// Operaciones múltiples
	EntradaMultipleStock(ctx context.Context, req *models.EntradaMultipleStockRequest) (*models.EntradaMultipleStockResponse, error)
	SalidaMultipleStock(ctx context.Context, req *models.SalidaMultipleStockRequest) (*models.SalidaMultipleStockResponse, error)

	// Consultas
	GetStockByLocal(ctx context.Context, idLocal int) ([]*models.Stock, error)
	GetStockBajo(ctx context.Context, idLocal int) ([]*models.Stock, error)
	GetStockByProducto(ctx context.Context, codigoProducto string, idLocal int) (*models.Stock, error)
	GetStockCompleteByLocal(ctx context.Context, idLocal int) ([]*models.StockComplete, error)
	GetMovimientosByLocal(ctx context.Context, filter *models.MovimientoFilter) ([]*models.Movimiento, error)

	// POS - Búsqueda de productos
	GetProductoByBarcode(ctx context.Context, barcode string) (*models.ProductoCompleto, error)
}

// stockService implementa StockService
type stockService struct {
	repo        repository.StockRepository
	productRepo repository.ProductRepository
	cache       *redis.Client
	logger      *zap.Logger
}

// NewStockService crea una nueva instancia del servicio
func NewStockService(repo repository.StockRepository, productRepo repository.ProductRepository, cache *redis.Client, logger *zap.Logger) StockService {
	return &stockService{
		repo:        repo,
		productRepo: productRepo,
		cache:       cache,
		logger:      logger,
	}
}

// EntradaStock procesa la entrada de stock de un producto
func (s *stockService) EntradaStock(ctx context.Context, req *models.EntradaStockRequest) (*models.EntradaStockResponse, error) {
	logger := s.logger.With(
		zap.String("operation", "entrada_stock"),
		zap.String("codigo_producto", req.CodigoProducto),
		zap.Int("cantidad", req.Cantidad),
		zap.Int("id_local", req.IDLocal),
		zap.Int("id_usuario", req.IDUsuario),
	)

	logger.Info("🔍 [DEBUG] Iniciando entrada de stock individual")

	// Verificar que el producto existe
	logger.Info("🔍 [DEBUG] Verificando que el producto existe",
		zap.String("codigo_producto", req.CodigoProducto),
		zap.String("tipo_item", req.TipoItem))

	if err := s.verificarProductoExiste(ctx, req.CodigoProducto, req.TipoItem); err != nil {
		logger.Error("❌ [DEBUG] Producto no encontrado", zap.Error(err))
		return nil, fmt.Errorf("producto no encontrado: %w", err)
	}
	logger.Info("✅ [DEBUG] Producto verificado exitosamente")

	// Obtener stock actual
	logger.Info("🔍 [DEBUG] Obteniendo stock actual")
	stockActual, err := s.repo.GetStockByProducto(ctx, req.CodigoProducto, req.IDLocal)
	if err != nil {
		logger.Error("❌ [DEBUG] Error obteniendo stock actual", zap.Error(err))
		return nil, fmt.Errorf("error obteniendo stock actual: %w", err)
	}

	cantidadAnterior := 0
	if stockActual != nil {
		cantidadAnterior = stockActual.CantidadActual
		logger.Info("🔍 [DEBUG] Stock actual encontrado",
			zap.Int("cantidad_anterior", cantidadAnterior))
	} else {
		logger.Info("🔍 [DEBUG] No hay stock actual, creando nuevo registro")
	}

	cantidadNueva := cantidadAnterior + req.Cantidad
	logger.Info("🔍 [DEBUG] Calculando cantidad nueva",
		zap.Int("cantidad_anterior", cantidadAnterior),
		zap.Int("cantidad_entrada", req.Cantidad),
		zap.Int("cantidad_nueva", cantidadNueva))

	// Actualizar o crear stock
	if stockActual != nil {
		logger.Info("🔍 [DEBUG] Actualizando stock existente")
		stockActual.CantidadActual = cantidadNueva
		if req.CantidadMinima > 0 {
			stockActual.CantidadMinima = req.CantidadMinima
			logger.Info("🔍 [DEBUG] Actualizando cantidad mínima", zap.Int("cantidad_minima", req.CantidadMinima))
		}
		err = s.repo.UpdateStock(ctx, stockActual)
	} else {
		logger.Info("🔍 [DEBUG] Creando nuevo stock")
		stockActual = &models.Stock{
			CodigoProducto: req.CodigoProducto,
			TipoItem:       req.TipoItem,
			CantidadActual: cantidadNueva,
			CantidadMinima: req.CantidadMinima,
			IDLocal:        req.IDLocal,
		}
		err = s.repo.CreateStock(ctx, stockActual)
	}

	if err != nil {
		logger.Error("❌ [DEBUG] Error actualizando/creando stock", zap.Error(err))
		return nil, fmt.Errorf("error actualizando stock: %w", err)
	}
	logger.Info("✅ [DEBUG] Stock actualizado/creado exitosamente")

	// Registrar movimiento
	logger.Info("🔍 [DEBUG] Creando movimiento")
	movimiento := &models.Movimiento{
		CodigoProducto:   req.CodigoProducto,
		TipoItem:         req.TipoItem,
		TipoMovimiento:   "entrada",
		Cantidad:         req.Cantidad,
		CantidadAnterior: cantidadAnterior,
		CantidadNueva:    cantidadNueva,
		Motivo:           req.Motivo,
		IDUsuario:        req.IDUsuario,
		IDLocal:          req.IDLocal,
		Observaciones:    req.Observaciones,
	}

	if err := s.repo.CreateMovimiento(ctx, movimiento); err != nil {
		logger.Error("❌ [DEBUG] Error creando movimiento", zap.Error(err))
		return nil, fmt.Errorf("error creando movimiento: %w", err)
	}
	logger.Info("✅ [DEBUG] Movimiento creado exitosamente")

	// Si es un pack, procesar productos individuales
	if req.TipoItem == "pack" {
		logger.Info("🔍 [DEBUG] Procesando pack")
		if err := s.procesarPack(ctx, req.CodigoProducto, req.Cantidad, "entrada", req.IDUsuario, req.IDLocal); err != nil {
			logger.Error("❌ [DEBUG] Error procesando pack", zap.Error(err))
			return nil, fmt.Errorf("error procesando pack: %w", err)
		}
		logger.Info("✅ [DEBUG] Pack procesado exitosamente")
	}

	// Invalidar cache
	logger.Info("🔍 [DEBUG] Invalidando cache")
	s.invalidarCacheStock(req.CodigoProducto, req.IDLocal)

	logger.Info("✅ [DEBUG] Entrada de stock completada exitosamente",
		zap.Int("cantidad_nueva", cantidadNueva))

	return &models.EntradaStockResponse{
		Success: true,
		Message: "✅ Entrada de stock registrada correctamente",
		Data: struct {
			CodigoProducto string `json:"codigo_producto"`
			TipoItem       string `json:"tipo_item"`
			Cantidad       int    `json:"cantidad"`
			CantidadNueva  int    `json:"cantidad_nueva"`
			Motivo         string `json:"motivo"`
			IDLocal        int    `json:"id_local"`
			Timestamp      string `json:"timestamp"`
		}{
			CodigoProducto: req.CodigoProducto,
			TipoItem:       req.TipoItem,
			Cantidad:       req.Cantidad,
			CantidadNueva:  cantidadNueva,
			Motivo:         req.Motivo,
			IDLocal:        req.IDLocal,
			Timestamp:      time.Now().Format(time.RFC3339),
		},
	}, nil
}

// SalidaStock procesa la salida de stock de un producto
func (s *stockService) SalidaStock(ctx context.Context, req *models.SalidaStockRequest) (*models.SalidaStockResponse, error) {
	logger := s.logger.With(
		zap.String("operation", "salida_stock"),
		zap.String("codigo_producto", req.CodigoProducto),
		zap.Int("cantidad", req.Cantidad),
		zap.Int("id_local", req.IDLocal),
	)

	logger.Info("Iniciando salida de stock")

	// Verificar que el producto existe
	if err := s.verificarProductoExiste(ctx, req.CodigoProducto, req.TipoItem); err != nil {
		logger.Error("Producto no encontrado", zap.Error(err))
		return nil, fmt.Errorf("producto no encontrado: %w", err)
	}

	// Obtener stock actual
	stockActual, err := s.repo.GetStockByProducto(ctx, req.CodigoProducto, req.IDLocal)
	if err != nil {
		logger.Error("Error obteniendo stock actual", zap.Error(err))
		return nil, fmt.Errorf("error obteniendo stock actual: %w", err)
	}

	if stockActual == nil {
		logger.Error("No hay stock disponible")
		return nil, fmt.Errorf("no hay stock disponible para el producto %s", req.CodigoProducto)
	}

	cantidadAnterior := stockActual.CantidadActual
	cantidadNueva := cantidadAnterior - req.Cantidad

	// Verificar stock suficiente
	if cantidadNueva < 0 {
		logger.Error("Stock insuficiente",
			zap.Int("stock_disponible", cantidadAnterior),
			zap.Int("cantidad_solicitada", req.Cantidad))
		return nil, fmt.Errorf("stock insuficiente: disponible %d, solicitado %d", cantidadAnterior, req.Cantidad)
	}

	// Actualizar stock
	stockActual.CantidadActual = cantidadNueva
	if err := s.repo.UpdateStock(ctx, stockActual); err != nil {
		logger.Error("Error actualizando stock", zap.Error(err))
		return nil, fmt.Errorf("error actualizando stock: %w", err)
	}

	// Registrar movimiento
	movimiento := &models.Movimiento{
		CodigoProducto:   req.CodigoProducto,
		TipoItem:         req.TipoItem,
		TipoMovimiento:   "salida",
		Cantidad:         req.Cantidad,
		CantidadAnterior: cantidadAnterior,
		CantidadNueva:    cantidadNueva,
		Motivo:           req.Motivo,
		IDUsuario:        req.IDUsuario,
		IDLocal:          req.IDLocal,
		Observaciones:    req.Observaciones,
	}

	if err := s.repo.CreateMovimiento(ctx, movimiento); err != nil {
		logger.Error("Error creando movimiento", zap.Error(err))
		return nil, fmt.Errorf("error creando movimiento: %w", err)
	}

	// Si es un pack, procesar productos individuales
	if req.TipoItem == "pack" {
		if err := s.procesarPack(ctx, req.CodigoProducto, req.Cantidad, "salida", req.IDUsuario, req.IDLocal); err != nil {
			logger.Error("Error procesando pack", zap.Error(err))
			return nil, fmt.Errorf("error procesando pack: %w", err)
		}
	}

	// Invalidar cache
	s.invalidarCacheStock(req.CodigoProducto, req.IDLocal)

	logger.Info("Salida de stock completada", zap.Int("cantidad_nueva", cantidadNueva))

	return &models.SalidaStockResponse{
		Success: true,
		Message: "✅ Salida de stock registrada correctamente",
		Data: struct {
			CodigoProducto string `json:"codigo_producto"`
			TipoItem       string `json:"tipo_item"`
			Cantidad       int    `json:"cantidad"`
			CantidadNueva  int    `json:"cantidad_nueva"`
			Motivo         string `json:"motivo"`
			IDLocal        int    `json:"id_local"`
			Timestamp      string `json:"timestamp"`
		}{
			CodigoProducto: req.CodigoProducto,
			TipoItem:       req.TipoItem,
			Cantidad:       req.Cantidad,
			CantidadNueva:  cantidadNueva,
			Motivo:         req.Motivo,
			IDLocal:        req.IDLocal,
			Timestamp:      time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GetStockByProducto obtiene el stock de un producto con cache
func (s *stockService) GetStockByProducto(ctx context.Context, codigoProducto string, idLocal int) (*models.Stock, error) {
	// Intentar obtener del cache
	cacheKey := fmt.Sprintf("stock:%s:%d", codigoProducto, idLocal)
	if _, err := s.cache.Get(ctx, cacheKey).Result(); err == nil {
		// TODO: Deserializar JSON del cache
		s.logger.Debug("Cache hit", zap.String("key", cacheKey))
	}

	// Obtener de la base de datos
	stock, err := s.repo.GetStockByProducto(ctx, codigoProducto, idLocal)
	if err != nil {
		return nil, err
	}

	// Guardar en cache si existe
	if stock != nil {
		// TODO: Serializar a JSON y guardar en cache
		s.cache.Set(ctx, cacheKey, "stock_data", 5*time.Minute)
	}

	return stock, nil
}

// GetStockByLocal obtiene todo el stock de un local
func (s *stockService) GetStockByLocal(ctx context.Context, idLocal int) ([]*models.Stock, error) {
	return s.repo.GetStockByLocal(ctx, idLocal)
}

// GetStockBajo obtiene productos con stock bajo
func (s *stockService) GetStockBajo(ctx context.Context, idLocal int) ([]*models.Stock, error) {
	return s.repo.GetStockBajo(ctx, idLocal)
}

// GetStockCompleteByLocal obtiene stock con información completa del producto, categoría y local
func (s *stockService) GetStockCompleteByLocal(ctx context.Context, idLocal int) ([]*models.StockComplete, error) {
	return s.repo.GetStockCompleteByLocal(ctx, idLocal)
}

// GetMovimientosByLocal obtiene movimientos de un local
func (s *stockService) GetMovimientosByLocal(ctx context.Context, filter *models.MovimientoFilter) ([]*models.Movimiento, error) {
	return s.repo.GetMovimientosByLocal(ctx, filter)
}

// EntradaMultipleStock procesa entrada múltiple de stock
func (s *stockService) EntradaMultipleStock(ctx context.Context, req *models.EntradaMultipleStockRequest) (*models.EntradaMultipleStockResponse, error) {
	logger := s.logger.With(
		zap.String("operation", "entrada_multiple_stock"),
		zap.Int("cantidad_productos", len(req.Productos)),
		zap.Int("id_local", req.IDLocal),
		zap.Int("id_usuario", req.IDUsuario),
	)

	logger.Info("🔍 [DEBUG] Iniciando entrada múltiple de stock en service")

	resultados := []models.ProductoResultado{}
	errores := []models.ProductoError{}

	// Procesar cada producto
	for i, producto := range req.Productos {
		logger.Info("🔍 [DEBUG] Procesando producto en entrada múltiple",
			zap.Int("index", i),
			zap.String("codigo_producto", producto.CodigoProducto),
			zap.String("tipo_item", producto.TipoItem),
			zap.Int("cantidad", producto.Cantidad),
			zap.Int("cantidad_minima", producto.CantidadMinima))

		entradaReq := &models.EntradaStockRequest{
			CodigoProducto: producto.CodigoProducto,
			TipoItem:       producto.TipoItem,
			Cantidad:       producto.Cantidad,
			CantidadMinima: producto.CantidadMinima,
			Motivo:         req.Motivo,
			IDUsuario:      req.IDUsuario,
			IDLocal:        req.IDLocal,
			Observaciones:  req.Observaciones,
		}

		logger.Info("🔍 [DEBUG] Llamando a EntradaStock individual",
			zap.String("codigo_producto", entradaReq.CodigoProducto),
			zap.Int("cantidad", entradaReq.Cantidad),
			zap.Int("id_local", entradaReq.IDLocal))

		response, err := s.EntradaStock(ctx, entradaReq)
		if err != nil {
			logger.Error("❌ [DEBUG] Error procesando producto en entrada múltiple",
				zap.String("codigo_producto", producto.CodigoProducto),
				zap.Error(err))
			errores = append(errores, models.ProductoError{
				CodigoProducto: producto.CodigoProducto,
				Error:          err.Error(),
			})
		} else {
			logger.Info("✅ [DEBUG] Producto procesado exitosamente en entrada múltiple",
				zap.String("codigo_producto", producto.CodigoProducto),
				zap.Int("cantidad_nueva", response.Data.CantidadNueva))
			resultados = append(resultados, models.ProductoResultado{
				CodigoProducto: producto.CodigoProducto,
				TipoItem:       producto.TipoItem,
				Cantidad:       producto.Cantidad,
				CantidadNueva:  response.Data.CantidadNueva,
				Success:        true,
			})
		}
	}

	// Determinar si fue exitoso
	success := len(errores) == 0
	message := "✅ Entrada múltiple de stock registrada correctamente"
	if len(errores) > 0 {
		message = "Algunos productos no pudieron ser procesados"
	}

	logger.Info("✅ [DEBUG] Entrada múltiple completada en service",
		zap.Int("productos_procesados", len(resultados)),
		zap.Int("productos_fallidos", len(errores)),
		zap.Bool("success", success),
		zap.String("message", message))

	return &models.EntradaMultipleStockResponse{
		Success:        success,
		Message:        message,
		TotalProductos: len(resultados),
		Resultados:     resultados,
		Errores:        errores,
		Timestamp:      time.Now().Format(time.RFC3339),
	}, nil
}

// SalidaMultipleStock procesa salida múltiple de stock
func (s *stockService) SalidaMultipleStock(ctx context.Context, req *models.SalidaMultipleStockRequest) (*models.SalidaMultipleStockResponse, error) {
	logger := s.logger.With(
		zap.String("operation", "salida_multiple_stock"),
		zap.Int("cantidad_productos", len(req.Productos)),
		zap.Int("id_local", req.IDLocal),
		zap.Int("id_usuario", req.IDUsuario),
	)

	logger.Info("🔍 [DEBUG] Iniciando salida múltiple de stock en service")

	resultados := []models.ProductoResultado{}
	errores := []models.ProductoError{}

	// Procesar cada producto
	for i, producto := range req.Productos {
		logger.Info("🔍 [DEBUG] Procesando producto en salida múltiple",
			zap.Int("index", i),
			zap.String("codigo_producto", producto.CodigoProducto),
			zap.String("tipo_item", producto.TipoItem),
			zap.Int("cantidad", producto.Cantidad))

		salidaReq := &models.SalidaStockRequest{
			CodigoProducto: producto.CodigoProducto,
			TipoItem:       producto.TipoItem,
			Cantidad:       producto.Cantidad,
			Motivo:         req.Motivo,
			IDUsuario:      req.IDUsuario,
			IDLocal:        req.IDLocal,
			Observaciones:  req.Observaciones,
		}

		logger.Info("🔍 [DEBUG] Llamando a SalidaStock individual",
			zap.String("codigo_producto", salidaReq.CodigoProducto),
			zap.Int("cantidad", salidaReq.Cantidad),
			zap.Int("id_local", salidaReq.IDLocal))

		response, err := s.SalidaStock(ctx, salidaReq)
		if err != nil {
			logger.Error("❌ [DEBUG] Error procesando producto en salida múltiple",
				zap.String("codigo_producto", producto.CodigoProducto),
				zap.Error(err))
			errores = append(errores, models.ProductoError{
				CodigoProducto: producto.CodigoProducto,
				Error:          err.Error(),
			})
		} else {
			logger.Info("✅ [DEBUG] Producto procesado exitosamente en salida múltiple",
				zap.String("codigo_producto", producto.CodigoProducto),
				zap.Int("cantidad_nueva", response.Data.CantidadNueva))
			resultados = append(resultados, models.ProductoResultado{
				CodigoProducto: producto.CodigoProducto,
				TipoItem:       producto.TipoItem,
				Cantidad:       producto.Cantidad,
				CantidadNueva:  response.Data.CantidadNueva,
				Success:        true,
			})
		}
	}

	// Determinar si fue exitoso
	success := len(errores) == 0
	message := "✅ Salida múltiple de stock registrada correctamente"
	if len(errores) > 0 {
		message = "Algunos productos no pudieron ser procesados"
	}

	logger.Info("✅ [DEBUG] Salida múltiple completada en service",
		zap.Int("productos_procesados", len(resultados)),
		zap.Int("productos_fallidos", len(errores)),
		zap.Bool("success", success),
		zap.String("message", message))

	return &models.SalidaMultipleStockResponse{
		Success:        success,
		Message:        message,
		TotalProductos: len(resultados),
		Resultados:     resultados,
		Errores:        errores,
		Timestamp:      time.Now().Format(time.RFC3339),
	}, nil
}

// Métodos auxiliares

func (s *stockService) verificarProductoExiste(ctx context.Context, codigoProducto, tipoItem string) error {
	if tipoItem == "producto" {
		producto, err := s.repo.GetProductoByCodigo(ctx, codigoProducto)
		if err != nil {
			return err
		}
		if producto == nil {
			return fmt.Errorf("producto %s no encontrado", codigoProducto)
		}
	} else if tipoItem == "pack" {
		pack, err := s.repo.GetPackByCodigo(ctx, codigoProducto)
		if err != nil {
			return err
		}
		if pack == nil {
			return fmt.Errorf("pack %s no encontrado", codigoProducto)
		}
	}
	return nil
}

func (s *stockService) procesarPack(ctx context.Context, codigoPack string, cantidad int, operacion string, idUsuario, idLocal int) error {
	// Obtener productos del pack
	productosPack, err := s.repo.GetPacksByProducto(ctx, codigoPack)
	if err != nil {
		return err
	}

	for _, productoPack := range productosPack {
		cantidadProducto := cantidad * productoPack.CantidadArticulo

		if operacion == "entrada" {
			req := &models.EntradaStockRequest{
				CodigoProducto: productoPack.CodigoArticulo,
				TipoItem:       "producto",
				Cantidad:       cantidadProducto,
				Motivo:         fmt.Sprintf("Entrada automática desde pack %s", codigoPack),
				IDUsuario:      idUsuario,
				IDLocal:        idLocal,
				Observaciones:  fmt.Sprintf("Pack: %s", codigoPack),
			}
			_, err = s.EntradaStock(ctx, req)
		} else {
			req := &models.SalidaStockRequest{
				CodigoProducto: productoPack.CodigoArticulo,
				TipoItem:       "producto",
				Cantidad:       cantidadProducto,
				Motivo:         fmt.Sprintf("Salida automática desde pack %s", codigoPack),
				IDUsuario:      idUsuario,
				IDLocal:        idLocal,
				Observaciones:  fmt.Sprintf("Pack: %s", codigoPack),
			}
			_, err = s.SalidaStock(ctx, req)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *stockService) invalidarCacheStock(codigoProducto string, idLocal int) {
	cacheKey := fmt.Sprintf("stock:%s:%d", codigoProducto, idLocal)
	s.cache.Del(context.Background(), cacheKey)
}

// GetProductoByBarcode busca un producto por código de barras (POS)
func (s *stockService) GetProductoByBarcode(ctx context.Context, barcode string) (*models.ProductoCompleto, error) {
	logger := s.logger.With(
		zap.String("operation", "get_producto_by_barcode"),
		zap.String("barcode", barcode),
	)

	logger.Info("Buscando producto por código de barras")

	// Buscar en el repository
	producto, err := s.productRepo.GetProductoByBarcode(ctx, barcode)
	if err != nil {
		logger.Error("Error buscando producto", zap.Error(err))
		return nil, fmt.Errorf("error buscando producto: %w", err)
	}

	if producto == nil {
		logger.Warn("Producto no encontrado")
		return nil, fmt.Errorf("producto no encontrado: %s", barcode)
	}

	logger.Info("Producto encontrado",
		zap.String("nombre", producto.Nombre),
		zap.String("origen", producto.Origen))

	return producto, nil
}
