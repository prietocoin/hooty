package cartelera

// punto es una coordenada del viewBox 24x24 que usan los iconos de tendencia.
type punto struct{ x, y float64 }

// trazosTendencia reproduce las polilíneas del SVG original. Se declaran como
// datos y no como código para que un cambio de icono sea una edición del mapa y
// no del dibujo. El estado "equal" es además el respaldo: el servicio original
// sustituía una tendencia ausente por "equal".
var trazosTendencia = map[string][][]punto{
	"up": {
		{{23, 6}, {13.5, 15.5}, {8.5, 10.5}, {1, 18}},
		{{17, 6}, {23, 6}, {23, 12}},
	},
	"down": {
		{{23, 18}, {13.5, 8.5}, {8.5, 13.5}, {1, 6}},
		{{17, 18}, {23, 18}, {23, 12}},
	},
	"equal": {
		{{4, 12}, {20, 12}},
		{{14, 6}, {20, 12}, {14, 18}},
	},
}

// trazos devuelve las polilíneas de la tendencia pedida, con "equal" de respaldo.
func trazos(tendencia string) [][]punto {
	if polilineas, ok := trazosTendencia[tendencia]; ok {
		return polilineas
	}
	return trazosTendencia["equal"]
}
