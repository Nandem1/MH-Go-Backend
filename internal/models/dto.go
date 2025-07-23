package models

// ===== REQUEST DTOs =====

// EntradaStockRequest DTO para entrada de stock
type EntradaStockRequest struct {
	CodigoProducto string `json:"codigo_producto" validate:"required"`
	TipoItem       string `json:"tipo_item" validate:"required,oneof=producto pack"`
	Cantidad       int    `json:"cantidad" validate:"required,gt=0"`
	Motivo         string `json:"motivo" validate:"required"`
	IDLocal        int    `json:"id_local" validate:"required,gt=0"`
	Observaciones  string `json:"observaciones"`
	CantidadMinima int    `json:"cantidad_minima" validate:"gte=0"`
	IDUsuario      int    `json:"-"` // Se obtiene del contexto de autenticación
}

// SalidaStockRequest DTO para salida de stock
type SalidaStockRequest struct {
	CodigoProducto string `json:"codigo_producto" validate:"required"`
	TipoItem       string `json:"tipo_item" validate:"required,oneof=producto pack"`
	Cantidad       int    `json:"cantidad" validate:"required,gt=0"`
	Motivo         string `json:"motivo" validate:"required"`
	IDLocal        int    `json:"id_local" validate:"required,gt=0"`
	Observaciones  string `json:"observaciones"`
	IDUsuario      int    `json:"-"` // Se obtiene del contexto de autenticación
}

// ProductoEntrada representa un producto en entrada múltiple (con cantidad_minima)
type ProductoEntrada struct {
	CodigoProducto string `json:"codigo_producto" validate:"required"`
	TipoItem       string `json:"tipo_item" validate:"required,oneof=producto pack"`
	Cantidad       int    `json:"cantidad" validate:"required,gt=0"`
	CantidadMinima int    `json:"cantidad_minima" validate:"gte=0"`
}

// ProductoSalida representa un producto en salida múltiple (sin cantidad_minima)
type ProductoSalida struct {
	CodigoProducto string `json:"codigo_producto" validate:"required"`
	TipoItem       string `json:"tipo_item" validate:"required,oneof=producto pack"`
	Cantidad       int    `json:"cantidad" validate:"required,gt=0"`
}

// EntradaMultipleStockRequest DTO para entrada múltiple de stock
type EntradaMultipleStockRequest struct {
	Productos     []ProductoEntrada `json:"productos" validate:"required,dive"`
	Motivo        string            `json:"motivo" validate:"required"`
	IDLocal       int               `json:"id_local" validate:"required,gt=0"`
	Observaciones string            `json:"observaciones"`
	IDUsuario     int               `json:"-"` // Se obtiene del contexto de autenticación
}

// SalidaMultipleStockRequest DTO para salida múltiple de stock
type SalidaMultipleStockRequest struct {
	Productos     []ProductoSalida `json:"productos" validate:"required,dive"`
	Motivo        string           `json:"motivo" validate:"required"`
	IDLocal       int              `json:"id_local" validate:"required,gt=0"`
	Observaciones string           `json:"observaciones"`
	IDUsuario     int              `json:"-"` // Se obtiene del contexto de autenticación
}

// ===== RESPONSE DTOs =====

// EntradaStockResponse respuesta para entrada de stock
type EntradaStockResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		CodigoProducto string `json:"codigo_producto"`
		TipoItem       string `json:"tipo_item"`
		Cantidad       int    `json:"cantidad"`
		CantidadNueva  int    `json:"cantidad_nueva"`
		Motivo         string `json:"motivo"`
		IDLocal        int    `json:"id_local"`
		Timestamp      string `json:"timestamp"`
	} `json:"data"`
}

// SalidaStockResponse respuesta para salida de stock
type SalidaStockResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		CodigoProducto string `json:"codigo_producto"`
		TipoItem       string `json:"tipo_item"`
		Cantidad       int    `json:"cantidad"`
		CantidadNueva  int    `json:"cantidad_nueva"`
		Motivo         string `json:"motivo"`
		IDLocal        int    `json:"id_local"`
		Timestamp      string `json:"timestamp"`
	} `json:"data"`
}

// EntradaMultipleStockResponse respuesta para entrada múltiple
type EntradaMultipleStockResponse struct {
	Success        bool                `json:"success"`
	Message        string              `json:"message"`
	TotalProductos int                 `json:"total_productos"`
	Resultados     []ProductoResultado `json:"resultados"`
	Errores        []ProductoError     `json:"errores,omitempty"`
	Timestamp      string              `json:"timestamp"`
}

// ProductoResultado resultado de procesamiento de un producto
type ProductoResultado struct {
	CodigoProducto string `json:"codigo_producto"`
	TipoItem       string `json:"tipo_item"`
	Cantidad       int    `json:"cantidad"`
	CantidadNueva  int    `json:"cantidad_nueva"`
	Success        bool   `json:"success"`
}

// ProductoError error de procesamiento de un producto
type ProductoError struct {
	CodigoProducto string `json:"codigo_producto"`
	Error          string `json:"error"`
}

// SalidaMultipleStockResponse respuesta para salida múltiple
type SalidaMultipleStockResponse struct {
	Success        bool                `json:"success"`
	Message        string              `json:"message"`
	TotalProductos int                 `json:"total_productos"`
	Resultados     []ProductoResultado `json:"resultados"`
	Errores        []ProductoError     `json:"errores,omitempty"`
	Timestamp      string              `json:"timestamp"`
}

// ===== POS DTOs =====

// QuickSaleRequest DTO para venta rápida (POS)
type QuickSaleRequest struct {
	Items         []ProductoStock `json:"items" validate:"required,dive"`
	Motivo        string          `json:"motivo" validate:"required"`
	IDLocal       int             `json:"id_local" validate:"required,gt=0"`
	Observaciones string          `json:"observaciones"`
	IDUsuario     int             `json:"-"` // Se obtiene del contexto JWT
}

// ProductoStock representa un producto en operaciones de stock
type ProductoStock struct {
	CodigoProducto string `json:"codigo_producto" validate:"required"`
	TipoItem       string `json:"tipo_item" validate:"required,oneof=producto pack"`
	Cantidad       int    `json:"cantidad" validate:"required,gt=0"`
	CantidadMinima int    `json:"cantidad_minima" validate:"gte=0"`
}

// ===== POS Response DTOs =====

// ProductoPOSResponse respuesta optimizada para POS
type ProductoPOSResponse struct {
	Codigo       string `json:"codigo"`
	Nombre       string `json:"nombre"`
	CodigoBarras string `json:"codigo_barras"`
	EsPack       bool   `json:"es_pack"`
	CantidadPack int    `json:"cantidad_pack,omitempty"`
}

// StockResponse respuesta para consultas de stock
type StockResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		IDLocal    int                 `json:"id_local"`
		TotalItems int                 `json:"total_items"`
		Productos  int                 `json:"productos"`
		Packs      int                 `json:"packs"`
		Stock      []*StockWithDetails `json:"stock"`
		Timestamp  string              `json:"timestamp"`
	} `json:"data"`
}

// MovimientosResponse respuesta para consultas de movimientos
type MovimientosResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		IDLocal          int                      `json:"id_local"`
		TotalMovimientos int                      `json:"total_movimientos"`
		Movimientos      []*MovimientoWithDetails `json:"movimientos"`
		Timestamp        string                   `json:"timestamp"`
	} `json:"data"`
}
