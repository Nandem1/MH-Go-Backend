package models

import (
	"time"
)

// Producto representa un producto básico
type Producto struct {
	ID                  int       `json:"id" db:"id"`
	Codigo              string    `json:"codigo" db:"codigo"`
	Nombre              string    `json:"nombre" db:"nombre"`
	Unidad              *string   `json:"unidad" db:"unidad"`
	Precio              *float64  `json:"precio" db:"precio"`
	CodigoBarraInterno  *string   `json:"codigo_barra_interno" db:"codigo_barra_interno"`
	CodigoBarraExterno  *string   `json:"codigo_barra_externo" db:"codigo_barra_externo"`
	Descripcion         *string   `json:"descripcion" db:"descripcion"`
	EsServicio          bool      `json:"es_servicio" db:"es_servicio"`
	EsExento            bool      `json:"es_exento" db:"es_exento"`
	ImpuestoEspecifico  *float64  `json:"impuesto_especifico" db:"impuesto_especifico"`
	IDCategoria         *int      `json:"id_categoria" db:"id_categoria"`
	DisponibleParaVenta bool      `json:"disponible_para_venta" db:"disponible_para_venta"`
	Activo              bool      `json:"activo" db:"activo"`
	Utilidad            *float64  `json:"utilidad" db:"utilidad"`
	TipoUtilidad        *string   `json:"tipo_utilidad" db:"tipo_utilidad"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// Pack representa un pack de productos
type Pack struct {
	ID               int       `json:"id" db:"id"`
	CodigoPack       string    `json:"codigo_pack" db:"codigo_pack"`
	NombrePack       string    `json:"nombre_pack" db:"nombre_pack"`
	PrecioBase       float64   `json:"precio_base" db:"precio_base"`
	CantidadArticulo int       `json:"cantidad_articulo" db:"cantidad_articulo"`
	CodigoArticulo   string    `json:"codigo_articulo" db:"codigo_articulo"`
	CodBarraArticulo string    `json:"cod_barra_articulo" db:"cod_barra_articulo"`
	NombreArticulo   string    `json:"nombre_articulo" db:"nombre_articulo"`
	CodBarraPack     string    `json:"cod_barra_pack" db:"cod_barra_pack"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Local representa una local/sucursal
type Local struct {
	ID        int       `json:"id" db:"id"`
	Nombre    string    `json:"nombre" db:"nombre"`
	Direccion string    `json:"direccion" db:"direccion"`
	Activo    bool      `json:"activo" db:"activo"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Usuario representa un usuario del sistema
type Usuario struct {
	ID        int       `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	Rol       string    `json:"rol" db:"rol"`
	Activo    bool      `json:"activo" db:"activo"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ProductoCompleto representa un producto con toda la información completa
// Incluye datos de productos, packs, precios y vencimientos
type ProductoCompleto struct {
	// Campos básicos del producto
	ID                  *int     `json:"id,omitempty" db:"id"`
	Codigo              string   `json:"codigo" db:"codigo"`
	Nombre              string   `json:"nombre" db:"nombre"`
	Unidad              *string  `json:"unidad,omitempty" db:"unidad"`
	Precio              *float64 `json:"precio,omitempty" db:"precio"`
	CodigoBarraInterno  *string  `json:"codigo_barra_interno,omitempty" db:"codigo_barra_interno"`
	CodigoBarraExterno  *string  `json:"codigo_barra_externo,omitempty" db:"codigo_barra_externo"`
	Descripcion         *string  `json:"descripcion,omitempty" db:"descripcion"`
	EsServicio          *bool    `json:"es_servicio,omitempty" db:"es_servicio"`
	EsExento            *bool    `json:"es_exento,omitempty" db:"es_exento"`
	ImpuestoEspecifico  *float64 `json:"impuesto_especifico,omitempty" db:"impuesto_especifico"`
	IDCategoria         *int     `json:"id_categoria,omitempty" db:"id_categoria"`
	DisponibleParaVenta *bool    `json:"disponible_para_venta,omitempty" db:"disponible_para_venta"`
	Activo              *bool    `json:"activo,omitempty" db:"activo"`
	Utilidad            *float64 `json:"utilidad,omitempty" db:"utilidad"`
	TipoUtilidad        *string  `json:"tipo_utilidad,omitempty" db:"tipo_utilidad"`

	// Campo origen (producto o pack)
	Origen      string `json:"origen" db:"origen"`
	CodigoFinal string `json:"codigo_final" db:"codigo_final"`

	// Campos específicos de pack
	CodigoPack       *string  `json:"codigo_pack,omitempty" db:"codigo_pack"`
	NombrePack       *string  `json:"nombre_pack,omitempty" db:"nombre_pack"`
	PrecioBase       *float64 `json:"precio_base,omitempty" db:"precio_base"`
	CantidadArticulo *int     `json:"cantidad_articulo,omitempty" db:"cantidad_articulo"`
	CodigoArticulo   *string  `json:"codigo_articulo,omitempty" db:"codigo_articulo"`
	CodBarraArticulo *string  `json:"cod_barra_articulo,omitempty" db:"cod_barra_articulo"`
	NombreArticulo   *string  `json:"nombre_articulo,omitempty" db:"nombre_articulo"`

	// Campos de lista de precios
	ListaPrecioDetalle   *float64   `json:"lista_precio_detalle,omitempty" db:"lista_precio_detalle"`
	ListaPrecioMayorista *float64   `json:"lista_precio_mayorista,omitempty" db:"lista_precio_mayorista"`
	ListaUpdatedAt       *time.Time `json:"lista_updated_at,omitempty" db:"lista_updated_at"`

	// Fechas de vencimiento (se procesará como JSON)
	FechasVencimiento []FechaVencimiento `json:"fechas_vencimiento,omitempty"`
}

// ToProductoPOSResponse convierte ProductoCompleto a ProductoPOSResponse
func (p *ProductoCompleto) ToProductoPOSResponse() ProductoPOSResponse {
	response := ProductoPOSResponse{
		Nombre: p.Nombre,
		EsPack: p.Origen == "pack",
	}

	// Determinar código y código de barras según el origen
	if p.Origen == "pack" {
		// Es un pack
		if p.CodigoPack != nil {
			response.Codigo = *p.CodigoPack
		}
		if p.CodigoBarraInterno != nil {
			response.CodigoBarras = *p.CodigoBarraInterno
		}
		if p.CantidadArticulo != nil {
			response.CantidadPack = *p.CantidadArticulo
		}
	} else {
		// Es un producto
		response.Codigo = p.Codigo
		if p.CodigoBarraExterno != nil {
			response.CodigoBarras = *p.CodigoBarraExterno
		} else if p.CodigoBarraInterno != nil {
			response.CodigoBarras = *p.CodigoBarraInterno
		}
		response.CantidadPack = 1 // Productos individuales
	}

	return response
}

// FechaVencimiento representa una fecha de vencimiento de un producto
type FechaVencimiento struct {
	FechaVencimiento time.Time `json:"fecha_vencimiento"`
	Cantidad         int       `json:"cantidad"`
	Lote             string    `json:"lote"`
}
