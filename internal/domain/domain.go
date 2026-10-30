// Package domain contiene las entidades y los invariantes contables de Hooty.
//
// El dominio no conoce PostgreSQL, HTTP ni proveedores de IA. Los adaptadores
// traducen sus errores a respuestas y a SQL, pero nunca deciden si un binomio
// es contablemente válido: esa decisión vive aquí y en los CHECK de la base.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Estados de una entrega al webhook.
const (
	StatusWaitingPair = "waiting_pair"
	StatusPairReady   = "pair_ready"
	StatusDuplicate   = "duplicate"
	StatusProcessed   = "processed"
)

// ErrValidation identifica un fallo de reglas de negocio, no de infraestructura.
var ErrValidation = errors.New("validación de dominio")

// Validationf construye un error de validación con causa inspeccionable.
func Validationf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// Impacto es la evidencia mínima que deja un mensaje en la sala de espera.
//
// El primer mensaje de un hash se persiste sin descargar ni subir su imagen:
// solo texto, identidad y marcas de tiempo. La imagen del segundo mensaje es
// la única que se procesa.
type Impacto struct {
	ID         int64     `json:"id"`
	HashLargo  string    `json:"hash_largo"`
	HashCorto  string    `json:"hash_corto"`
	IDGrupo    string    `json:"id_grupo"`
	Caption    string    `json:"caption"`
	Instancia  string    `json:"instancia"`
	Procesado  bool      `json:"procesado"`
	RecibidoEn time.Time `json:"recibido_en"`
	ParejaID   *int64    `json:"pareja_id,omitempty"`
}

// Registro es el resultado transaccional de RegistrarImpacto.
//
// Pair solo viaja completo cuando Status es pair_ready. Waiting y duplicate no
// autorizan a descargar media.
type Registro struct {
	Status  string    `json:"status"`
	Impacto Impacto   `json:"impacto"`
	Pair    []Impacto `json:"-"`
	Conteo  int       `json:"conteo"`
}

// Comprobante es el snapshot contable 2x que vive en la tabla comprobantes.
//
// Los importes firmados conservan la polaridad del contrato; el frontend no
// debe recalcularlos. La base refuerza los mismos invariantes con CHECK.
type Comprobante struct {
	HashLargo string `json:"hash_largo"`
	HashCorto string `json:"hash_corto"`
	Instancia string `json:"instancia"`

	MontoRecibo  float64 `json:"monto_recibo"`
	MonedaRecibo string  `json:"moneda_recibo"`
	Banco        string  `json:"banco"`
	Titular      string  `json:"titular"`
	Referencia   string  `json:"referencia"`
	URLImagen    string  `json:"url_imagen"`

	Grupo1   string `json:"grupo_1"`
	Grupo2   string `json:"grupo_2"`
	Caption1 string `json:"caption_1"`
	Caption2 string `json:"caption_2"`

	Socio1 string  `json:"socio_1"`
	Socio2 string  `json:"socio_2"`
	Tasa1  float64 `json:"tasa_1"`
	Tasa2  float64 `json:"tasa_2"`
	Monto1 float64 `json:"monto_1"`
	Monto2 float64 `json:"monto_2"`
	ME1    float64 `json:"me1"`
	ME2    float64 `json:"me2"`

	LoteTasa       string    `json:"lote_tasa"`
	FechaOperacion time.Time `json:"fecha_operacion"`
	CreadoEn       time.Time `json:"creado_en"`
	ActualizadoEn  time.Time `json:"actualizado_en"`
}

// Extraccion es el contrato que devuelve el extractor de IA.
type Extraccion struct {
	Monto      float64 `json:"monto"`
	Moneda     string  `json:"moneda"`
	Banco      string  `json:"banco"`
	Titular    string  `json:"titular"`
	Referencia string  `json:"referencia"`
}

// LiquidacionInput reúne el comprobante crudo y el binomio que lo justifica.
type LiquidacionInput struct {
	HashLargo  string
	HashCorto  string
	Instancia  string
	Extraccion Extraccion
	URLImagen  string

	Grupo1   string
	Grupo2   string
	Caption1 string
	Caption2 string

	FechaOperacion time.Time
	RecibidoEn     time.Time
}

// Perfil es la porción de perfiles_glaukov que necesita la liquidación base.
type Perfil struct {
	Nombre     string          `json:"nombre"`
	MonedaBase string          `json:"moneda_base"`
	Monedas    json.RawMessage `json:"monedas,omitempty"`
}

// LoteTasas es el lote congelado que se usará para ambos lados.
type LoteTasas struct {
	ID    string             `json:"id_tasa"`
	Tasas map[string]float64 `json:"tasas"`
}

// Decision es la respuesta inmediata del webhook. No incluye captions ni
// credenciales; solo el estado y la identidad del binomio.
type Decision struct {
	Status string `json:"status"`
	Hash   string `json:"hash"`
	Conteo int    `json:"conteo"`
}

// NormalizeImpacto limpia la identidad antes de persistir.
func NormalizeImpacto(i Impacto) Impacto {
	i.HashLargo = NormalizeHash(i.HashLargo)
	if strings.TrimSpace(i.HashCorto) == "" {
		i.HashCorto = ShortHash(i.HashLargo)
	}
	i.HashCorto = strings.ToUpper(strings.TrimSpace(i.HashCorto))
	i.IDGrupo = strings.TrimSpace(i.IDGrupo)
	i.Caption = CleanText(i.Caption, 2000)
	i.Instancia = strings.TrimSpace(i.Instancia)
	return i
}

// NormalizeComprobante limpia textos y normaliza identidades.
func NormalizeComprobante(c Comprobante) Comprobante {
	c.HashLargo = NormalizeHash(c.HashLargo)
	if strings.TrimSpace(c.HashCorto) == "" {
		c.HashCorto = ShortHash(c.HashLargo)
	}
	c.HashCorto = strings.ToUpper(strings.TrimSpace(c.HashCorto))
	c.Instancia = strings.TrimSpace(c.Instancia)
	c.MontoRecibo = Round2(math.Abs(c.MontoRecibo))
	c.MonedaRecibo = NormalizeCurrency(c.MonedaRecibo, "USDT")
	c.Tasa1 = RoundRate(c.Tasa1)
	c.Tasa2 = RoundRate(c.Tasa2)
	c.Monto1 = Truncar2(c.Monto1)
	c.Monto2 = Truncar2(c.Monto2)
	c.ME1 = Truncar2(c.ME1)
	c.ME2 = Truncar2(c.ME2)
	c.Banco = CleanText(c.Banco, 80)
	c.Titular = CleanText(c.Titular, 120)
	c.Referencia = CleanText(c.Referencia, 80)
	c.URLImagen = strings.TrimSpace(c.URLImagen)
	c.Grupo1 = strings.TrimSpace(c.Grupo1)
	c.Grupo2 = strings.TrimSpace(c.Grupo2)
	c.Caption1 = CleanText(c.Caption1, 2000)
	c.Caption2 = CleanText(c.Caption2, 2000)
	c.Socio1 = CleanText(c.Socio1, 120)
	c.Socio2 = CleanText(c.Socio2, 120)
	c.LoteTasa = strings.TrimSpace(c.LoteTasa)
	return c
}

// Validate comprueba el invariante 2x antes de escribir.
func (i Impacto) Validate() error {
	if strings.TrimSpace(i.HashLargo) == "" {
		return Validationf("hash_largo es obligatorio")
	}
	if strings.TrimSpace(i.HashCorto) == "" {
		return Validationf("hash_corto es obligatorio")
	}
	if !IsRealGroup(i.IDGrupo) {
		return Validationf("id_grupo no puede ser vacío ni un placeholder")
	}
	if strings.TrimSpace(i.Instancia) == "" {
		return Validationf("instancia es obligatoria")
	}
	if i.RecibidoEn.IsZero() {
		return Validationf("recibido_en es obligatorio")
	}
	return nil
}

// Validate comprueba el invariante contable del snapshot 2x.
func (c Comprobante) Validate() error {
	if strings.TrimSpace(c.HashLargo) == "" {
		return Validationf("hash_largo es obligatorio")
	}
	if strings.TrimSpace(c.HashCorto) == "" {
		return Validationf("hash_corto es obligatorio")
	}
	if strings.TrimSpace(c.Instancia) == "" {
		return Validationf("instancia es obligatoria")
	}
	if strings.TrimSpace(c.URLImagen) == "" {
		return Validationf("url_imagen es obligatoria")
	}
	if c.MontoRecibo <= 0 {
		return Validationf("monto_recibo debe ser positivo")
	}
	if strings.TrimSpace(c.MonedaRecibo) == "" {
		return Validationf("moneda_recibo es obligatoria")
	}
	if !IsRealGroup(c.Grupo1) || !IsRealGroup(c.Grupo2) {
		return Validationf("grupo_1 y grupo_2 deben ser grupos reales")
	}
	if Canonical(c.Grupo1) == Canonical(c.Grupo2) {
		return Validationf("grupo_1 y grupo_2 deben ser distintos")
	}
	if !IsRealSocio(c.Socio1) || !IsRealSocio(c.Socio2) {
		return Validationf("socio_1 y socio_2 deben ser socios reales")
	}
	if Canonical(c.Socio1) == Canonical(c.Socio2) {
		return Validationf("socio_1 y socio_2 deben ser distintos")
	}
	if c.Tasa1 < 0 || c.Tasa2 < 0 {
		return Validationf("las tasas no pueden ser negativas")
	}
	if err := validarLadoContable("1", c.Tasa1, c.Monto1, c.ME1); err != nil {
		return err
	}
	if err := validarLadoContable("2", c.Tasa2, c.Monto2, c.ME2); err != nil {
		return err
	}
	if strings.TrimSpace(c.LoteTasa) == "" {
		return Validationf("lote_tasa es obligatorio")
	}
	if c.FechaOperacion.IsZero() {
		return Validationf("fecha_operacion es obligatoria")
	}
	if c.CreadoEn.IsZero() {
		return Validationf("creado_en es obligatorio")
	}
	for name, value := range map[string]float64{
		"monto_recibo": c.MontoRecibo,
		"tasa_1":       c.Tasa1,
		"tasa_2":       c.Tasa2,
		"monto_1":      c.Monto1,
		"monto_2":      c.Monto2,
		"me1":          c.ME1,
		"me2":          c.ME2,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return Validationf("%s no es finito", name)
		}
	}
	return nil
}

// validarLadoContable exige importes cuando hay tasa y ceros cuando la tasa es
// 0, estado válido para un socio sin regla de conversión en esa divisa.
func validarLadoContable(label string, tasa, monto, me float64) error {
	if tasa == 0 {
		if monto != 0 || me != 0 {
			return Validationf("monto_%s y me%s deben ser cero cuando la tasa es cero", label, label)
		}
		return nil
	}
	if math.Abs(monto) < 0.01 {
		return Validationf("ambos lados deben tener importe contable de al menos 0.01")
	}
	if math.Abs(me) < 0.01 {
		return Validationf("ambos equivalentes deben tener importe contable de al menos 0.01")
	}
	return nil
}

// Validate rechaza extracciones sin monto o divisa.
func (e Extraccion) Validate() error {
	if e.Monto <= 0 || math.IsNaN(e.Monto) || math.IsInf(e.Monto, 0) {
		return Validationf("el comprobante no tiene un monto legible")
	}
	if strings.TrimSpace(e.Moneda) == "" {
		return Validationf("la extracción no identificó la moneda")
	}
	return nil
}

// Normalize limpia y normaliza una extracción.
func (e Extraccion) Normalize() Extraccion {
	return Extraccion{
		Monto:      Round2(e.Monto),
		Moneda:     NormalizeCurrency(e.Moneda, ""),
		Banco:      CleanText(e.Banco, 80),
		Titular:    CleanText(e.Titular, 120),
		Referencia: CleanText(e.Referencia, 80),
	}
}

// NormalizeHash fija la identidad canónica: minúsculas y sin espacios.
func NormalizeHash(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// ShortHash devuelve los primeros ocho caracteres en mayúsculas.
func ShortHash(hash string) string {
	clean := strings.ToUpper(strings.TrimSpace(hash))
	if len(clean) <= 8 {
		return clean
	}
	return clean[:8]
}

// IsPlaceholder identifica valores que nunca representan un socio o grupo.
func IsPlaceholder(value string) bool {
	switch Canonical(value) {
	case "", "GENERAL", "NO DEFINIDO", "S/D", "N/A", "NA", "NULL", "UNDEFINED", "-":
		return true
	}
	return false
}

// IsRealSocio indica si el nombre puede participar en un binomio 2x.
func IsRealSocio(value string) bool {
	return !IsPlaceholder(value)
}

// IsRealGroup es el equivalente para JID o identificadores de grupo.
func IsRealGroup(value string) bool {
	return !IsPlaceholder(value)
}

var accentReplacer = strings.NewReplacer(
	"Á", "A", "À", "A", "Ä", "A", "Â", "A", "Ã", "A", "Å", "A",
	"É", "E", "È", "E", "Ë", "E", "Ê", "E",
	"Í", "I", "Ì", "I", "Ï", "I", "Î", "I",
	"Ó", "O", "Ò", "O", "Ö", "O", "Ô", "O", "Õ", "O", "Ø", "O",
	"Ú", "U", "Ù", "U", "Ü", "U", "Û", "U",
	"Ñ", "N", "Ç", "C",
)

// Canonical normaliza mayúsculas, espacios y diacríticos para comparar
// identidades sin depender de la presentación.
func Canonical(value string) string {
	clean := strings.ToUpper(strings.TrimSpace(value))
	clean = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, clean)
	clean = accentReplacer.Replace(clean)
	return strings.Join(strings.Fields(clean), " ")
}

// NormalizeCurrency devuelve el código ISO en mayúsculas o el fallback.
func NormalizeCurrency(value, fallback string) string {
	clean := strings.ToUpper(strings.TrimSpace(value))
	if clean == "" {
		return strings.ToUpper(strings.TrimSpace(fallback))
	}
	return clean
}

// CleanText colapsa espacios y recorta por runas para no partir UTF-8.
func CleanText(value string, limit int) string {
	clean := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len([]rune(clean)) <= limit {
		return clean
	}
	return string([]rune(clean)[:limit])
}

// Truncar2 corta a dos decimales exactos usando la representación decimal
// más corta del float. No multiplica por 100: 1.15 -> 1.15, 2.675 -> 2.67,
// 1.005 -> 1.00. Es la regla contable de montos.
func Truncar2(value float64) float64 {
	return truncarDecimales(value, 2)
}

// Truncar6 corta a seis decimales exactos para tasas.
func Truncar6(value float64) float64 {
	return truncarDecimales(value, 6)
}

// Round2 conserva el nombre histórico y delega en el truncamiento exacto.
func Round2(value float64) float64 {
	return Truncar2(value)
}

// RoundRate conserva el nombre histórico y delega en el truncamiento exacto.
func RoundRate(value float64) float64 {
	return Truncar6(value)
}

func truncarDecimales(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if decimals < 0 {
		decimals = 0
	}
	// 'f' con precisión -1 produce la representación decimal más corta que
	// round-trips al mismo float64, evitando el error de value*100.
	text := strconv.FormatFloat(value, 'f', -1, 64)
	dot := strings.IndexByte(text, '.')
	if dot < 0 {
		return value
	}
	limit := dot + 1 + decimals
	if len(text) > limit {
		text = text[:limit]
	}
	truncated, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return truncated
}
