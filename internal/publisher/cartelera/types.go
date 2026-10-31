// Package cartelera contiene el generador oficial de carteleras migrado desde
// el repositorio Glauk (tasasgo). Se conserva la lógica de cálculo y el dibujo
// con fondo de constelaciones, banderas y tipografía embebida.
package cartelera

// Porcentajes son los márgenes que un socio aplica sobre la tasa triangulada.
type Porcentajes struct {
	Deposito float64 `json:"deposito"`
	Pago     float64 `json:"pago"`
}

// Moneda describe cómo publica un socio una divisa concreta.
type Moneda struct {
	Activo     *bool       `json:"activo,omitempty"`
	Porcentaje Porcentajes `json:"porcentaje"`
}

// Perfil es un socio: su identidad, el grupo donde publica y sus divisas.
type Perfil struct {
	Nombre     string             `json:"nombre"`
	IDGrupo    string             `json:"id_grupo,omitempty"`
	MonedaBase string             `json:"moneda_base,omitempty"`
	Monedas    map[string]*Moneda `json:"monedas"`
}

// Lote es un conjunto de tasas publicado en un momento dado.
type Lote struct {
	IDTasa string         `json:"id_tasa,omitempty"`
	Tasas  map[string]any `json:"tasas"`
}

// TarjetaPais es una fila de la cartelera.
type TarjetaPais struct {
	Code       string  `json:"code"`
	NombrePais string  `json:"nombre_pais"`
	Bandera    string  `json:"bandera"`
	Compra     float64 `json:"compra"`
	Venta      float64 `json:"venta"`
	Trend      string  `json:"trend"`
}

// Cartelera es el resultado del cálculo para un socio.
type Cartelera struct {
	NombreSocio string        `json:"nombre_socio"`
	IDGrupo     string        `json:"id_grupo,omitempty"`
	MonedaBase  string        `json:"moneda_base"`
	LoteTasa    string        `json:"lote_tasa"`
	Fecha       string        `json:"fecha_actualizacion"`
	Hora        string        `json:"hora_actualizacion"`
	Tarjetas    []TarjetaPais `json:"tarjetas_paises"`
}
