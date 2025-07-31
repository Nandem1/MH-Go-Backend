package models

import (
	"time"
)

// Stock representa la tabla stock_bodega_cantera
type Stock struct {
	ID             int       `json:"id" db:"id"`
	CodigoProducto string    `json:"codigo_producto" db:"codigo_producto"`
	TipoItem       string    `json:"tipo_item" db:"tipo_item"`
	CantidadActual int       `json:"cantidad_actual" db:"cantidad_actual"`
	CantidadMinima int       `json:"cantidad_minima" db:"cantidad_minima"`
	IDLocal        int       `json:"id_local" db:"id_local"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// StockWithDetails incluye información adicional del producto/pack
type StockWithDetails struct {
	Stock
	NombreProducto string `json:"nombre_producto,omitempty"`
	NombreLocal    string `json:"nombre_local,omitempty"`
}

// StockComplete incluye información completa del producto, categoría y local
type StockComplete struct {
	// Campos del stock
	ID             int       `json:"id" db:"id"`
	CodigoProducto string    `json:"codigo_producto" db:"codigo_producto"`
	TipoItem       string    `json:"tipo_item" db:"tipo_item"`
	CantidadActual int       `json:"cantidad_actual" db:"cantidad_actual"`
	CantidadMinima int       `json:"cantidad_minima" db:"cantidad_minima"`
	IDLocal        int       `json:"id_local" db:"id_local"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	
	// Campos del producto (JOIN con productos)
	NombreProducto     *string  `json:"nombre_producto,omitempty" db:"nombre_producto"`
	CodigoBarraInterno *string  `json:"codigo_barra_interno,omitempty" db:"codigo_barra_interno"`
	CodigoBarraExterno *string  `json:"codigo_barra_externo,omitempty" db:"codigo_barra_externo"`
	Descripcion        *string  `json:"descripcion,omitempty" db:"descripcion"`
	Precio             *float64 `json:"precio,omitempty" db:"precio"`
	Unidad             *string  `json:"unidad,omitempty" db:"unidad"`
	IDCategoria        *int     `json:"id_categoria,omitempty" db:"id_categoria"`
	EsServicio         *bool    `json:"es_servicio,omitempty" db:"es_servicio"`
	EsExento           *bool    `json:"es_exento,omitempty" db:"es_exento"`
	ImpuestoEspecifico *float64 `json:"impuesto_especifico,omitempty" db:"impuesto_especifico"`
	DisponibleVenta    *bool    `json:"disponible_para_venta,omitempty" db:"disponible_para_venta"`
	Activo             *bool    `json:"activo,omitempty" db:"activo"`
	Utilidad           *float64 `json:"utilidad,omitempty" db:"utilidad"`
	TipoUtilidad       *string  `json:"tipo_utilidad,omitempty" db:"tipo_utilidad"`
	
	// Campos de la categoría (JOIN con categorias)
	NombreCategoria    *string `json:"nombre_categoria,omitempty" db:"nombre_categoria"`
	
	// Campos del local (JOIN con locales)
	NombreLocal        *string `json:"nombre_local,omitempty" db:"nombre_local"`
}

// StockSummary resumen de stock por local
type StockSummary struct {
	IDLocal        int    `json:"id_local"`
	NombreLocal    string `json:"nombre_local"`
	TotalProductos int    `json:"total_productos"`
	TotalPacks     int    `json:"total_packs"`
	StockBajo      int    `json:"stock_bajo"`
}
