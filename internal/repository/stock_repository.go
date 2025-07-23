package repository

import (
	"context"
	"database/sql"
	"fmt"

	"stock-service/internal/models"
)

// StockRepository define la interfaz para operaciones de stock
type StockRepository interface {
	// Operaciones básicas de stock
	GetStockByProducto(ctx context.Context, codigoProducto string, idLocal int) (*models.Stock, error)
	UpdateStock(ctx context.Context, stock *models.Stock) error
	CreateStock(ctx context.Context, stock *models.Stock) error
	GetStockByLocal(ctx context.Context, idLocal int) ([]*models.Stock, error)
	GetStockBajo(ctx context.Context, idLocal int) ([]*models.Stock, error)

	// Operaciones de movimientos
	CreateMovimiento(ctx context.Context, movimiento *models.Movimiento) error
	GetMovimientosByLocal(ctx context.Context, filter *models.MovimientoFilter) ([]*models.Movimiento, error)

	// Operaciones batch
	BatchUpdateStock(ctx context.Context, stocks []*models.Stock) error

	// Operaciones de productos y packs
	GetProductoByCodigo(ctx context.Context, codigo string) (*models.Producto, error)
	GetPackByCodigo(ctx context.Context, codigo string) (*models.Pack, error)
	GetPacksByProducto(ctx context.Context, codigoProducto string) ([]*models.Pack, error)
}

// stockRepository implementa StockRepository
type stockRepository struct {
	db    *sql.DB
	stmts map[string]*sql.Stmt
}

// NewStockRepository crea una nueva instancia del repository
func NewStockRepository(db *sql.DB) (StockRepository, error) {
	repo := &stockRepository{
		db:    db,
		stmts: make(map[string]*sql.Stmt),
	}

	if err := repo.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return repo, nil
}

// prepareStatements prepara todas las consultas SQL para mejor rendimiento
func (r *stockRepository) prepareStatements() error {
	statements := map[string]string{
		"get_stock": `
			SELECT id, codigo_producto, tipo_item, cantidad_actual, cantidad_minima, 
				   id_local, created_at, updated_at
			FROM stock_bodega_cantera 
			WHERE codigo_producto = $1 AND id_local = $2
		`,
		"update_stock": `
			UPDATE stock_bodega_cantera 
			SET cantidad_actual = $1, cantidad_minima = $2, updated_at = NOW()
			WHERE codigo_producto = $3 AND id_local = $4
		`,
		"create_stock": `
			INSERT INTO stock_bodega_cantera 
			(codigo_producto, tipo_item, cantidad_actual, cantidad_minima, id_local)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at, updated_at
		`,
		"get_stock_by_local": `
			SELECT id, codigo_producto, tipo_item, cantidad_actual, cantidad_minima, 
				   id_local, created_at, updated_at
			FROM stock_bodega_cantera 
			WHERE id_local = $1
			ORDER BY codigo_producto
		`,
		"get_stock_bajo": `
			SELECT id, codigo_producto, tipo_item, cantidad_actual, cantidad_minima, 
				   id_local, created_at, updated_at
			FROM stock_bodega_cantera 
			WHERE id_local = $1 AND cantidad_actual <= cantidad_minima
			ORDER BY cantidad_actual ASC
		`,
		"create_movimiento": `
			INSERT INTO stock_movimientos_cantera 
			(codigo_producto, tipo_item, tipo_movimiento, cantidad, cantidad_anterior, 
			 cantidad_nueva, motivo, id_usuario, id_local, observaciones)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, created_at
		`,
		"get_producto": `
			SELECT id, codigo, nombre, unidad, precio, codigo_barra_interno, 
				   codigo_barra_externo, descripcion, es_servicio, es_exento,
				   impuesto_especifico, id_categoria, disponible_para_venta, 
				   activo, utilidad, tipo_utilidad
			FROM productos 
			WHERE codigo = $1 AND activo = true
		`,
		"get_pack": `
			SELECT id, codigo_pack, cod_barra_pack, nombre_pack, precio_base,
				   cantidad_articulo, codigo_articulo, cod_barra_articulo, nombre_articulo
			FROM pack_listados 
			WHERE codigo_pack = $1
		`,
		"get_packs_by_producto": `
			SELECT id, codigo_pack, cod_barra_pack, nombre_pack, precio_base,
				   cantidad_articulo, codigo_articulo, cod_barra_articulo, nombre_articulo
			FROM pack_listados 
			WHERE codigo_articulo = $1
		`,
	}

	for name, query := range statements {
		stmt, err := r.db.Prepare(query)
		if err != nil {
			return fmt.Errorf("failed to prepare %s: %w", name, err)
		}
		r.stmts[name] = stmt
	}

	return nil
}

// GetStockByProducto obtiene el stock de un producto específico
func (r *stockRepository) GetStockByProducto(ctx context.Context, codigoProducto string, idLocal int) (*models.Stock, error) {
	var stock models.Stock
	err := r.stmts["get_stock"].QueryRowContext(ctx, codigoProducto, idLocal).Scan(
		&stock.ID, &stock.CodigoProducto, &stock.TipoItem, &stock.CantidadActual,
		&stock.CantidadMinima, &stock.IDLocal, &stock.CreatedAt, &stock.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stock: %w", err)
	}

	return &stock, nil
}

// UpdateStock actualiza el stock de un producto
func (r *stockRepository) UpdateStock(ctx context.Context, stock *models.Stock) error {
	result, err := r.stmts["update_stock"].ExecContext(ctx,
		stock.CantidadActual, stock.CantidadMinima, stock.CodigoProducto, stock.IDLocal,
	)
	if err != nil {
		return fmt.Errorf("failed to update stock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no stock record found for product %s in local %d", stock.CodigoProducto, stock.IDLocal)
	}

	return nil
}

// CreateStock crea un nuevo registro de stock
func (r *stockRepository) CreateStock(ctx context.Context, stock *models.Stock) error {
	err := r.stmts["create_stock"].QueryRowContext(ctx,
		stock.CodigoProducto, stock.TipoItem, stock.CantidadActual, stock.CantidadMinima, stock.IDLocal,
	).Scan(&stock.ID, &stock.CreatedAt, &stock.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create stock: %w", err)
	}

	return nil
}

// GetStockByLocal obtiene todo el stock de un local
func (r *stockRepository) GetStockByLocal(ctx context.Context, idLocal int) ([]*models.Stock, error) {
	rows, err := r.stmts["get_stock_by_local"].QueryContext(ctx, idLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock by local: %w", err)
	}
	defer rows.Close()

	var stocks []*models.Stock
	for rows.Next() {
		var stock models.Stock
		err := rows.Scan(
			&stock.ID, &stock.CodigoProducto, &stock.TipoItem, &stock.CantidadActual,
			&stock.CantidadMinima, &stock.IDLocal, &stock.CreatedAt, &stock.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stock: %w", err)
		}
		stocks = append(stocks, &stock)
	}

	return stocks, nil
}

// GetStockBajo obtiene productos con stock bajo
func (r *stockRepository) GetStockBajo(ctx context.Context, idLocal int) ([]*models.Stock, error) {
	rows, err := r.stmts["get_stock_bajo"].QueryContext(ctx, idLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock bajo: %w", err)
	}
	defer rows.Close()

	var stocks []*models.Stock
	for rows.Next() {
		var stock models.Stock
		err := rows.Scan(
			&stock.ID, &stock.CodigoProducto, &stock.TipoItem, &stock.CantidadActual,
			&stock.CantidadMinima, &stock.IDLocal, &stock.CreatedAt, &stock.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stock: %w", err)
		}
		stocks = append(stocks, &stock)
	}

	return stocks, nil
}

// CreateMovimiento crea un nuevo movimiento de stock
func (r *stockRepository) CreateMovimiento(ctx context.Context, movimiento *models.Movimiento) error {
	err := r.stmts["create_movimiento"].QueryRowContext(ctx,
		movimiento.CodigoProducto, movimiento.TipoItem, movimiento.TipoMovimiento,
		movimiento.Cantidad, movimiento.CantidadAnterior, movimiento.CantidadNueva,
		movimiento.Motivo, movimiento.IDUsuario, movimiento.IDLocal, movimiento.Observaciones,
	).Scan(&movimiento.ID, &movimiento.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create movimiento: %w", err)
	}

	return nil
}

// GetMovimientosByLocal obtiene movimientos de un local con filtros
func (r *stockRepository) GetMovimientosByLocal(ctx context.Context, filter *models.MovimientoFilter) ([]*models.Movimiento, error) {
	// TODO: Implementar consulta dinámica con filtros
	// Por ahora retornamos una implementación básica
	return []*models.Movimiento{}, nil
}

// BatchUpdateStock actualiza múltiples stocks en una transacción
func (r *stockRepository) BatchUpdateStock(ctx context.Context, stocks []*models.Stock) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE stock_bodega_cantera 
		SET cantidad_actual = $1, cantidad_minima = $2, updated_at = NOW()
		WHERE codigo_producto = $3 AND id_local = $4
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch update statement: %w", err)
	}
	defer stmt.Close()

	for _, stock := range stocks {
		_, err := stmt.ExecContext(ctx,
			stock.CantidadActual, stock.CantidadMinima, stock.CodigoProducto, stock.IDLocal,
		)
		if err != nil {
			return fmt.Errorf("failed to update stock %s: %w", stock.CodigoProducto, err)
		}
	}

	return tx.Commit()
}

// GetProductoByCodigo obtiene un producto por código
func (r *stockRepository) GetProductoByCodigo(ctx context.Context, codigo string) (*models.Producto, error) {
	var producto models.Producto
	err := r.stmts["get_producto"].QueryRowContext(ctx, codigo).Scan(
		&producto.ID, &producto.Codigo, &producto.Nombre, &producto.Unidad, &producto.Precio,
		&producto.CodigoBarraInterno, &producto.CodigoBarraExterno, &producto.Descripcion,
		&producto.EsServicio, &producto.EsExento, &producto.ImpuestoEspecifico,
		&producto.IDCategoria, &producto.DisponibleParaVenta, &producto.Activo,
		&producto.Utilidad, &producto.TipoUtilidad,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get producto: %w", err)
	}

	return &producto, nil
}

// GetPackByCodigo obtiene un pack por código
func (r *stockRepository) GetPackByCodigo(ctx context.Context, codigo string) (*models.Pack, error) {
	var pack models.Pack
	err := r.stmts["get_pack"].QueryRowContext(ctx, codigo).Scan(
		&pack.ID, &pack.CodigoPack, &pack.CodBarraPack, &pack.NombrePack, &pack.PrecioBase,
		&pack.CantidadArticulo, &pack.CodigoArticulo, &pack.CodBarraArticulo, &pack.NombreArticulo,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pack: %w", err)
	}

	return &pack, nil
}

// GetPacksByProducto obtiene todos los packs que contienen un producto
func (r *stockRepository) GetPacksByProducto(ctx context.Context, codigoProducto string) ([]*models.Pack, error) {
	rows, err := r.stmts["get_packs_by_producto"].QueryContext(ctx, codigoProducto)
	if err != nil {
		return nil, fmt.Errorf("failed to get packs by producto: %w", err)
	}
	defer rows.Close()

	var packs []*models.Pack
	for rows.Next() {
		var pack models.Pack
		err := rows.Scan(
			&pack.ID, &pack.CodigoPack, &pack.CodBarraPack, &pack.NombrePack, &pack.PrecioBase,
			&pack.CantidadArticulo, &pack.CodigoArticulo, &pack.CodBarraArticulo, &pack.NombreArticulo,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pack: %w", err)
		}
		packs = append(packs, &pack)
	}

	return packs, nil
}
