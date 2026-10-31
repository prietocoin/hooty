package cartelera

// Tarjeta es una fila de la cartelera: lo que devuelve /calcular y lo que el
// dibujo lee para pintar cada tarjeta.
//
// Se declara aquí, y no reutilizando TarjetaPais, para que un cambio en el
// vocabulario interno no altere sin querer el JSON público ni la imagen: el
// compilador no protege un contrato HTTP.
//
// Compra y Venta son punteros porque el cálculo puede producir NaN —un defecto
// latente del original, ver calculator/precision.go— y JSON no lo admite: el
// servicio Node habría emitido null, y eso es lo que emite este tipo.
type Tarjeta struct {
	Code       string   `json:"code"`
	Pais       string   `json:"pais"`
	NombrePais string   `json:"nombre_pais"`
	Bandera    string   `json:"bandera"`
	Compra     *float64 `json:"compra"`
	Venta      *float64 `json:"venta"`
	Trend      string   `json:"trend"`
}

// Payload es el objeto que devuelve /calcular y la vista que consume el dibujo.
// Los nombres JSON son los que el panel ya usa: cambiarlos rompe al cliente en
// silencio, sin error de compilación.
//
// `id_grupo` y `moneda_base` no los lee el dibujo, pero se mantienen porque el
// JSON normalizado del origen los incluía y el panel podría necesitarlos.
type Payload struct {
	NombreSocio string    `json:"nombre_socio"`
	IDGrupo     string    `json:"id_grupo,omitempty"`
	MonedaBase  string    `json:"moneda_base"`
	LoteTasa    string    `json:"lote_tasa"`
	Fecha       string    `json:"fecha_actualizacion"`
	Hora        string    `json:"hora_actualizacion"`
	Tarjetas    []Tarjeta `json:"tarjetas_paises"`
}
