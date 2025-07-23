package models

import (
	"time"
)

// Movimiento representa la tabla stock_movimientos_cantera
type Movimiento struct {
	ID               int       `json:"id" db:"id"`
	CodigoProducto   string    `json:"codigo_producto" db:"codigo_producto"`
	TipoItem         string    `json:"tipo_item" db:"tipo_item"`
	TipoMovimiento   string    `json:"tipo_movimiento" db:"tipo_movimiento"`
	Cantidad         int       `json:"cantidad" db:"cantidad"`
	CantidadAnterior int       `json:"cantidad_anterior" db:"cantidad_anterior"`
	CantidadNueva    int       `json:"cantidad_nueva" db:"cantidad_nueva"`
	Motivo           string    `json:"motivo" db:"motivo"`
	IDUsuario        int       `json:"id_usuario" db:"id_usuario"`
	IDLocal          int       `json:"id_local" db:"id_local"`
	Observaciones    string    `json:"observaciones" db:"observaciones"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// MovimientoWithDetails incluye información adicional
type MovimientoWithDetails struct {
	Movimiento
	NombreProducto string `json:"nombre_producto,omitempty"`
	NombreUsuario  string `json:"nombre_usuario,omitempty"`
	NombreLocal    string `json:"nombre_local,omitempty"`
}

// MovimientoFilter filtros para consultas de movimientos
type MovimientoFilter struct {
	IDLocal        *int       `json:"id_local,omitempty"`
	TipoMovimiento *string    `json:"tipo_movimiento,omitempty"`
	TipoItem       *string    `json:"tipo_item,omitempty"`
	CodigoProducto *string    `json:"codigo_producto,omitempty"`
	FechaDesde     *time.Time `json:"fecha_desde,omitempty"`
	FechaHasta     *time.Time `json:"fecha_hasta,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
}
