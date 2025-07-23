package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"stock-service/internal/models"

	"go.uber.org/zap"
)

// ProductRepository interface para operaciones de productos
type ProductRepository interface {
	GetProductoByBarcode(ctx context.Context, barcode string) (*models.ProductoCompleto, error)
	GetProductosFrecuentes(ctx context.Context, limit int) ([]*models.ProductoCompleto, error)
	UpdateProducto(ctx context.Context, producto *models.ProductoCompleto) error
}

// productRepository implementación del repository
type productRepository struct {
	db     *sql.DB
	stmts  map[string]*sql.Stmt
	logger *zap.Logger
}

// NewProductRepository crea una nueva instancia del repository
func NewProductRepository(db *sql.DB, logger *zap.Logger) (ProductRepository, error) {
	repo := &productRepository{
		db:     db,
		stmts:  make(map[string]*sql.Stmt),
		logger: logger,
	}

	if err := repo.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return repo, nil
}

// prepareStatements prepara todas las queries SQL
func (r *productRepository) prepareStatements() error {
	// Query para buscar producto por código de barras
	queryProducto := `
		SELECT
			p.id,
			p.codigo,
			p.nombre,
			p.unidad,
			p.precio,
			p.codigo_barra_interno,
			p.codigo_barra_externo,
			p.descripcion,
			p.es_servicio,
			p.es_exento,
			p.impuesto_especifico,
			p.id_categoria,
			p.disponible_para_venta,
			p.activo,
			p.utilidad,
			p.tipo_utilidad,
			'producto' AS origen,
			p.codigo AS codigo_final,
			NULL AS codigo_pack,
			NULL AS nombre_pack,
			NULL AS precio_base,
			NULL AS cantidad_articulo,
			NULL AS codigo_articulo,
			NULL AS cod_barra_articulo,
			NULL AS nombre_articulo,
			lp.precio_detalle AS lista_precio_detalle,
			lp.precio_mayorista AS lista_precio_mayorista,
			lp.updated_at AS lista_updated_at,
			ARRAY_AGG(
				CASE 
					WHEN cvc.fecha_vencimiento IS NOT NULL 
					THEN json_build_object(
						'fecha_vencimiento', cvc.fecha_vencimiento,
						'cantidad', cvc.cantidad,
						'lote', cvc.lote
					)
				END
			) FILTER (WHERE cvc.fecha_vencimiento IS NOT NULL) AS fechas_vencimiento
		FROM productos p
		LEFT JOIN lista_precios_cantera lp ON p.codigo = lp.codigo_tivendo
		LEFT JOIN control_vencimientos_cantera cvc ON p.codigo_barra_interno = cvc.codigo_barras
		WHERE p.codigo_barra_externo = $1 OR p.codigo_barra_interno = $1
		GROUP BY 
			p.id, p.codigo, p.nombre, p.unidad, p.precio, p.codigo_barra_interno,
			p.codigo_barra_externo, p.descripcion, p.es_servicio, p.es_exento,
			p.impuesto_especifico, p.id_categoria, p.disponible_para_venta,
			p.activo, p.utilidad, p.tipo_utilidad,
			lp.precio_detalle, lp.precio_mayorista, lp.updated_at
		LIMIT 1;
	`

	// Query para buscar pack por código de barras
	queryPack := `
		SELECT
			NULL AS id,
			pl.codigo_pack AS codigo,
			pl.nombre_pack AS nombre,
			NULL AS unidad,
			pl.precio_base AS precio,
			pl.cod_barra_pack AS codigo_barra_interno,
			pl.cod_barra_pack AS codigo_barra_externo,
			NULL AS descripcion,
			false AS es_servicio,
			false AS es_exento,
			NULL AS impuesto_especifico,
			NULL AS id_categoria,
			true AS disponible_para_venta,
			true AS activo,
			NULL AS utilidad,
			NULL AS tipo_utilidad,
			'pack' AS origen,
			pl.codigo_pack AS codigo_final,
			pl.codigo_pack,
			pl.nombre_pack,
			pl.precio_base,
			pl.cantidad_articulo,
			pl.codigo_articulo,
			pl.cod_barra_articulo,
			pl.nombre_articulo,
			lp.precio_detalle AS lista_precio_detalle,
			lp.precio_mayorista AS lista_precio_mayorista,
			lp.updated_at AS lista_updated_at,
			ARRAY_AGG(
				CASE 
					WHEN cvc.fecha_vencimiento IS NOT NULL 
					THEN json_build_object(
						'fecha_vencimiento', cvc.fecha_vencimiento,
						'cantidad', cvc.cantidad,
						'lote', cvc.lote
					)
				END
			) FILTER (WHERE cvc.fecha_vencimiento IS NOT NULL) AS fechas_vencimiento
		FROM pack_listados pl
		LEFT JOIN lista_precios_cantera lp ON pl.codigo_pack = lp.codigo_tivendo
		LEFT JOIN control_vencimientos_cantera cvc ON pl.cod_barra_pack = cvc.codigo_barras
		WHERE pl.cod_barra_pack = $1 OR pl.codigo_pack = $1
		GROUP BY 
			pl.codigo_pack, pl.nombre_pack, pl.precio_base, pl.cantidad_articulo,
			pl.codigo_articulo, pl.cod_barra_articulo, pl.nombre_articulo,
			pl.cod_barra_pack,
			lp.precio_detalle, lp.precio_mayorista, lp.updated_at
		LIMIT 1;
	`

	// Query para productos frecuentes
	queryFrecuentes := `
		SELECT
			p.id,
			p.codigo,
			p.nombre,
			p.unidad,
			p.precio,
			p.codigo_barra_interno,
			p.codigo_barra_externo,
			p.descripcion,
			p.es_servicio,
			p.es_exento,
			p.impuesto_especifico,
			p.id_categoria,
			p.disponible_para_venta,
			p.activo,
			p.utilidad,
			p.tipo_utilidad,
			'producto' AS origen,
			p.codigo AS codigo_final,
			NULL AS codigo_pack,
			NULL AS nombre_pack,
			NULL AS precio_base,
			NULL AS cantidad_articulo,
			NULL AS codigo_articulo,
			NULL AS cod_barra_articulo,
			NULL AS nombre_articulo,
			lp.precio_detalle AS lista_precio_detalle,
			lp.precio_mayorista AS lista_precio_mayorista,
			lp.updated_at AS lista_updated_at,
			ARRAY_AGG(
				CASE 
					WHEN cvc.fecha_vencimiento IS NOT NULL 
					THEN json_build_object(
						'fecha_vencimiento', cvc.fecha_vencimiento,
						'cantidad', cvc.cantidad,
						'lote', cvc.lote
					)
				END
			) FILTER (WHERE cvc.fecha_vencimiento IS NOT NULL) AS fechas_vencimiento
		FROM productos p
		LEFT JOIN lista_precios_cantera lp ON p.codigo = lp.codigo_tivendo
		LEFT JOIN control_vencimientos_cantera cvc ON p.codigo_barra_interno = cvc.codigo_barras
		WHERE p.activo = true AND p.disponible_para_venta = true
		GROUP BY 
			p.id, p.codigo, p.nombre, p.unidad, p.precio, p.codigo_barra_interno,
			p.codigo_barra_externo, p.descripcion, p.es_servicio, p.es_exento,
			p.impuesto_especifico, p.id_categoria, p.disponible_para_venta,
			p.activo, p.utilidad, p.tipo_utilidad,
			lp.precio_detalle, lp.precio_mayorista, lp.updated_at
		ORDER BY p.nombre
		LIMIT $1;
	`

	// Preparar statements
	statements := map[string]string{
		"get_producto_by_barcode":  queryProducto,
		"get_pack_by_barcode":      queryPack,
		"get_productos_frecuentes": queryFrecuentes,
	}

	for name, query := range statements {
		stmt, err := r.db.Prepare(query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement %s: %w", name, err)
		}
		r.stmts[name] = stmt
	}

	return nil
}

// GetProductoByBarcode busca un producto o pack por código de barras
func (r *productRepository) GetProductoByBarcode(ctx context.Context, barcode string) (*models.ProductoCompleto, error) {
	start := time.Now()

	// 1. Buscar en productos
	row := r.stmts["get_producto_by_barcode"].QueryRowContext(ctx, barcode)
	producto, err := r.scanProductoCompleto(row)
	if err == nil && producto != nil {
		r.logger.Debug("Producto encontrado en tabla productos",
			zap.String("codigo_barras", barcode),
			zap.String("nombre", producto.Nombre),
			zap.Duration("latency", time.Since(start)))
		return producto, nil
	}

	// 2. Buscar en packs
	row = r.stmts["get_pack_by_barcode"].QueryRowContext(ctx, barcode)
	pack, err := r.scanProductoCompleto(row)
	if err == nil && pack != nil {
		r.logger.Debug("Pack encontrado en tabla pack_listados",
			zap.String("codigo_barras", barcode),
			zap.String("nombre", pack.Nombre),
			zap.Duration("latency", time.Since(start)))
		return pack, nil
	}

	r.logger.Debug("Producto/Pack no encontrado",
		zap.String("codigo_barras", barcode),
		zap.Duration("latency", time.Since(start)))

	return nil, fmt.Errorf("producto no encontrado: %s", barcode)
}

// GetProductosFrecuentes obtiene productos frecuentes para pre-carga
func (r *productRepository) GetProductosFrecuentes(ctx context.Context, limit int) ([]*models.ProductoCompleto, error) {
	rows, err := r.stmts["get_productos_frecuentes"].QueryContext(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query productos frecuentes: %w", err)
	}
	defer rows.Close()

	var productos []*models.ProductoCompleto
	for rows.Next() {
		producto, err := r.scanProductoCompleto(rows)
		if err != nil {
			r.logger.Error("Error scanning producto frecuente", zap.Error(err))
			continue
		}
		productos = append(productos, producto)
	}

	return productos, nil
}

// UpdateProducto actualiza un producto (placeholder para futuras implementaciones)
func (r *productRepository) UpdateProducto(ctx context.Context, producto *models.ProductoCompleto) error {
	// TODO: Implementar actualización de producto
	return fmt.Errorf("not implemented yet")
}

// scanProductoCompleto escanea una fila de la base de datos
func (r *productRepository) scanProductoCompleto(row interface{}) (*models.ProductoCompleto, error) {
	var producto models.ProductoCompleto
	var fechasVencimientoJSON []byte
	var listaUpdatedAt sql.NullTime

	// Determinar el tipo de row (Row o Rows)
	switch r := row.(type) {
	case *sql.Row:
		err := r.Scan(
			&producto.ID,
			&producto.Codigo,
			&producto.Nombre,
			&producto.Unidad,
			&producto.Precio,
			&producto.CodigoBarraInterno,
			&producto.CodigoBarraExterno,
			&producto.Descripcion,
			&producto.EsServicio,
			&producto.EsExento,
			&producto.ImpuestoEspecifico,
			&producto.IDCategoria,
			&producto.DisponibleParaVenta,
			&producto.Activo,
			&producto.Utilidad,
			&producto.TipoUtilidad,
			&producto.Origen,
			&producto.CodigoFinal,
			&producto.CodigoPack,
			&producto.NombrePack,
			&producto.PrecioBase,
			&producto.CantidadArticulo,
			&producto.CodigoArticulo,
			&producto.CodBarraArticulo,
			&producto.NombreArticulo,
			&producto.ListaPrecioDetalle,
			&producto.ListaPrecioMayorista,
			&listaUpdatedAt,
			&fechasVencimientoJSON,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
	case *sql.Rows:
		err := r.Scan(
			&producto.ID,
			&producto.Codigo,
			&producto.Nombre,
			&producto.Unidad,
			&producto.Precio,
			&producto.CodigoBarraInterno,
			&producto.CodigoBarraExterno,
			&producto.Descripcion,
			&producto.EsServicio,
			&producto.EsExento,
			&producto.ImpuestoEspecifico,
			&producto.IDCategoria,
			&producto.DisponibleParaVenta,
			&producto.Activo,
			&producto.Utilidad,
			&producto.TipoUtilidad,
			&producto.Origen,
			&producto.CodigoFinal,
			&producto.CodigoPack,
			&producto.NombrePack,
			&producto.PrecioBase,
			&producto.CantidadArticulo,
			&producto.CodigoArticulo,
			&producto.CodBarraArticulo,
			&producto.NombreArticulo,
			&producto.ListaPrecioDetalle,
			&producto.ListaPrecioMayorista,
			&listaUpdatedAt,
			&fechasVencimientoJSON,
		)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported row type")
	}

	// Procesar fechas de vencimiento
	if len(fechasVencimientoJSON) > 0 {
		// TODO: Implementar parsing de JSON para fechas de vencimiento
		// producto.FechasVencimiento = parseFechasVencimiento(fechasVencimientoJSON)
	}

	// Procesar lista updated at
	if listaUpdatedAt.Valid {
		producto.ListaUpdatedAt = &listaUpdatedAt.Time
	}

	return &producto, nil
}
