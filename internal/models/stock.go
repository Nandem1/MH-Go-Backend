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

// StockSummary resumen de stock por local
type StockSummary struct {
	IDLocal        int    `json:"id_local"`
	NombreLocal    string `json:"nombre_local"`
	TotalProductos int    `json:"total_productos"`
	TotalPacks     int    `json:"total_packs"`
	StockBajo      int    `json:"stock_bajo"`
}
