package api

import (
	"encoding/json"
	"strings"
	"time"

	"hooty/internal/clock"
	"hooty/internal/domain"
)

// ComprobanteRow es la fila cruda de comprobantes. El DTO se construye aparte
// para no mezclar la forma de la base con el contrato de la UI.
type ComprobanteRow struct {
	HashLargo      string
	HashCorto      string
	Instancia      string
	MontoRecibo    float64
	MonedaRecibo   string
	Banco          string
	Titular        string
	Referencia     string
	URLImagen      string
	Grupo1         string
	Grupo2         string
	Caption1       string
	Caption2       string
	Socio1         string
	Socio2         string
	Tasa1          float64
	Tasa2          float64
	Monto1         float64
	Monto2         float64
	ME1            float64
	ME2            float64
	LoteTasa       string
	FechaOperacion time.Time
	CreadoEn       time.Time
	ActualizadoEn  time.Time
}

// ComprobantePatch es el payload de PUT /api/v1/comprobantes/{hash}. Los
// punteros distinguen “campo ausente” de “valor cero”: la edición es parcial.
type ComprobantePatch struct {
	Banco        *string  `json:"banco"`
	Referencia   *string  `json:"referencia"`
	Titular      *string  `json:"titular"`
	MontoRecibo  *float64 `json:"monto_recibo"`
	MonedaRecibo *string  `json:"moneda_recibo"`

	Socio1 *string  `json:"socio_1"`
	Monto1 *float64 `json:"monto_1"`
	Tasa1  *float64 `json:"tasa_1"`
	ME1    *float64 `json:"me1"`

	Socio2 *string  `json:"socio_2"`
	Monto2 *float64 `json:"monto_2"`
	Tasa2  *float64 `json:"tasa_2"`
	ME2    *float64 `json:"me2"`

	// Alias de compatibilidad con la UI histórica.
	NombreSocio1 *string  `json:"nombre_socio_1"`
	NombreSocio2 *string  `json:"nombre_socio_2"`
	Monto        *float64 `json:"monto"`
	Moneda       *string  `json:"moneda"`
}

// Perfil es la fila de la vista perfil.
type Perfil struct {
	Nombre       string
	IDGrupo      string
	Rol          string
	MonedaBase   string
	SaldoInicial float64
	Mostrar      json.RawMessage
	Monedas      json.RawMessage
	Herencia     bool
}

// Tasa es la fila vigente de la vista tasa.
type Tasa struct {
	IDTasa string
	Tasas  json.RawMessage
}

// HashOption alimenta el selector de hashes de Reportes.
type HashOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ReportFilters es la respuesta de /api/v1/reportes/filtros.
type ReportFilters struct {
	Roles     []string     `json:"roles"`
	Entidades []string     `json:"entidades"`
	Hashes    []HashOption `json:"hashes"`
}

// ComprobanteDTO conserva el contrato canónico y los aliases que la UI
// actual todavía consume.
type ComprobanteDTO struct {
	HashLargo string `json:"hash_largo"`
	HashCorto string `json:"hash_corto"`
	Instancia string `json:"instancia"`

	MontoRecibo float64 `json:"monto_recibo"`
	Monto       float64 `json:"monto"`
	MontoLocal  float64 `json:"monto_local"`

	MonedaRecibo string `json:"moneda_recibo"`
	Moneda       string `json:"moneda"`
	MonedaLocal  string `json:"moneda_local"`

	Banco      string `json:"banco"`
	Titular    string `json:"titular"`
	Referencia string `json:"referencia"`

	URLImagen        string `json:"url_imagen"`
	LinkImg          string `json:"link_img"`
	URLR2Comprobante string `json:"url_r2_comprobante"`

	Grupo1   string `json:"grupo_1"`
	Grupo2   string `json:"grupo_2"`
	Caption1 string `json:"caption_1"`
	Caption2 string `json:"caption_2"`

	Socio1       string `json:"socio_1"`
	Socio2       string `json:"socio_2"`
	NombreSocio1 string `json:"nombre_socio_1"`
	NombreSocio2 string `json:"nombre_socio_2"`

	Rol1    string `json:"rol_1"`
	Rol2    string `json:"rol_2"`
	TipoOp1 string `json:"tipo_op1"`
	TipoOp2 string `json:"tipo_op2"`

	MonedaSocio1     string `json:"moneda_socio_1"`
	MonedaSocio2     string `json:"moneda_socio_2"`
	MonedaBaseSocio1 string `json:"moneda_base_socio1"`
	MonedaBaseSocio2 string `json:"moneda_base_socio2"`

	Tasa1      float64 `json:"tasa_1"`
	Tasa2      float64 `json:"tasa_2"`
	Monto1     float64 `json:"monto_1"`
	Monto2     float64 `json:"monto_2"`
	ME1        float64 `json:"me1"`
	ME2        float64 `json:"me2"`
	M1Socio    float64 `json:"m1_socio"`
	M2Socio    float64 `json:"m2_socio"`
	ME1Firmado float64 `json:"me1_firmado"`
	ME2Firmado float64 `json:"me2_firmado"`
	Polaridad1 string  `json:"polaridad_1"`
	Polaridad2 string  `json:"polaridad_2"`

	LoteTasa         string `json:"lote_tasa"`
	LoteTasaAsignado string `json:"lote_tasa_asignado"`
	Lote             string `json:"lote"`

	EsX2 bool `json:"es_x2"`

	FechaOperacion       string `json:"fecha_operacion"`
	FechaHoraComprobante string `json:"fecha_hora_comprobante"`
	CreadoEn             string `json:"creado_en"`
	ActualizadoEn        string `json:"actualizado_en"`
}

// PerfilDTO expone la vista perfil y los aliases que usa el panel.
type PerfilDTO struct {
	Nombre        string          `json:"nombre"`
	IDGrupo       string          `json:"id_grupo"`
	WhatsApp      string          `json:"whatsapp"`
	Rol           string          `json:"rol"`
	Roles         string          `json:"roles"`
	MonedaBase    string          `json:"moneda_base"`
	MonedaSocio   string          `json:"moneda_socio"`
	SaldoInicial  float64         `json:"saldo_inicial"`
	SaldoAnterior float64         `json:"saldo_anterior"`
	Mostrar       json.RawMessage `json:"mostrar"`
	Monedas       json.RawMessage `json:"monedas"`
	Herencia      bool            `json:"herencia"`
}

// TasaDTO es el contrato de /api/v1/tasas/monitor.
type TasaDTO struct {
	IDTasa string          `json:"id_tasa"`
	Tasas  json.RawMessage `json:"tasas"`
}

// NewComprobanteDTO construye la fila de compatibilidad.
func NewComprobanteDTO(row ComprobanteRow, perfil1, perfil2 *Perfil) ComprobanteDTO {
	moneda := domain.NormalizeCurrency(row.MonedaRecibo, "USDT")
	socio1 := strings.TrimSpace(row.Socio1)
	socio2 := strings.TrimSpace(row.Socio2)
	monedaBase1 := monedaBaseDe(perfil1)
	monedaBase2 := monedaBaseDe(perfil2)
	rol1 := rolDe(perfil1)
	rol2 := rolDe(perfil2)
	tipo1 := tipoOperacion(moneda, perfil1)
	tipo2 := tipoOperacion(moneda, perfil2)
	lote := strings.TrimSpace(row.LoteTasa)
	fecha := fechaLocal(row.FechaOperacion)

	return ComprobanteDTO{
		HashLargo: row.HashLargo,
		HashCorto: row.HashCorto,
		Instancia: row.Instancia,

		MontoRecibo: row.MontoRecibo,
		Monto:       row.MontoRecibo,
		MontoLocal:  row.MontoRecibo,

		MonedaRecibo: moneda,
		Moneda:       moneda,
		MonedaLocal:  moneda,

		Banco:      row.Banco,
		Titular:    row.Titular,
		Referencia: row.Referencia,

		URLImagen:        row.URLImagen,
		LinkImg:          row.URLImagen,
		URLR2Comprobante: row.URLImagen,

		Grupo1:   row.Grupo1,
		Grupo2:   row.Grupo2,
		Caption1: row.Caption1,
		Caption2: row.Caption2,

		Socio1:       socio1,
		Socio2:       socio2,
		NombreSocio1: socio1,
		NombreSocio2: socio2,

		Rol1:    rol1,
		Rol2:    rol2,
		TipoOp1: tipo1,
		TipoOp2: tipo2,

		MonedaSocio1:     monedaBase1,
		MonedaSocio2:     monedaBase2,
		MonedaBaseSocio1: monedaBase1,
		MonedaBaseSocio2: monedaBase2,

		Tasa1:      row.Tasa1,
		Tasa2:      row.Tasa2,
		Monto1:     row.Monto1,
		Monto2:     row.Monto2,
		ME1:        row.ME1,
		ME2:        row.ME2,
		M1Socio:    row.ME1,
		M2Socio:    row.ME2,
		ME1Firmado: row.ME1,
		ME2Firmado: row.ME2,
		Polaridad1: polaridad(row.ME1),
		Polaridad2: polaridad(row.ME2),

		LoteTasa:         lote,
		LoteTasaAsignado: lote,
		Lote:             lote,

		EsX2: domain.IsRealSocio(socio2) &&
			domain.Canonical(socio1) != domain.Canonical(socio2),

		FechaOperacion:       fecha,
		FechaHoraComprobante: fecha,
		CreadoEn:             fechaLocal(row.CreadoEn),
		ActualizadoEn:        fechaLocal(row.ActualizadoEn),
	}
}

// NewPerfilDTO normaliza la fila de perfil.
func NewPerfilDTO(p Perfil) PerfilDTO {
	rol := strings.TrimSpace(p.Rol)
	if rol == "" {
		rol = "SOCIO"
	}
	moneda := domain.NormalizeCurrency(p.MonedaBase, "USDT")
	mostrar := p.Mostrar
	if len(mostrar) == 0 || string(mostrar) == "null" {
		mostrar = json.RawMessage(`{}`)
	}
	monedas := p.Monedas
	if len(monedas) == 0 || string(monedas) == "null" {
		monedas = json.RawMessage(`{}`)
	}
	return PerfilDTO{
		Nombre:        strings.TrimSpace(p.Nombre),
		IDGrupo:       strings.TrimSpace(p.IDGrupo),
		WhatsApp:      strings.TrimSpace(p.IDGrupo),
		Rol:           rol,
		Roles:         rol,
		MonedaBase:    moneda,
		MonedaSocio:   moneda,
		SaldoInicial:  p.SaldoInicial,
		SaldoAnterior: p.SaldoInicial,
		Mostrar:       mostrar,
		Monedas:       monedas,
		Herencia:      p.Herencia,
	}
}

// NewTasaDTO normaliza la tasa vigente.
func NewTasaDTO(t Tasa) TasaDTO {
	tasas := t.Tasas
	if len(tasas) == 0 || string(tasas) == "null" {
		tasas = json.RawMessage(`{}`)
	}
	return TasaDTO{IDTasa: strings.TrimSpace(t.IDTasa), Tasas: tasas}
}

// NewReportFiltersDTO normaliza listas para que nunca sean null en JSON.
func NewReportFiltersDTO(f ReportFilters) ReportFilters {
	if f.Roles == nil {
		f.Roles = []string{}
	}
	if f.Entidades == nil {
		f.Entidades = []string{}
	}
	if f.Hashes == nil {
		f.Hashes = []HashOption{}
	}
	return f
}

// MapPerfiles indexa por nombre canónico para calcular tipo_op y monedas base.
func MapPerfiles(perfiles []Perfil) map[string]*Perfil {
	index := make(map[string]*Perfil, len(perfiles)*2)
	for i := range perfiles {
		p := &perfiles[i]
		nombre := strings.TrimSpace(p.Nombre)
		if nombre == "" {
			continue
		}
		index[domain.Canonical(nombre)] = p
		index[strings.ToUpper(nombre)] = p
	}
	return index
}

func tipoOperacion(moneda string, perfil *Perfil) string {
	if perfil == nil {
		return "D"
	}
	moneda = domain.NormalizeCurrency(moneda, "")
	base := domain.NormalizeCurrency(perfil.MonedaBase, "")
	if moneda != "" && base != "" && moneda == base {
		return "A"
	}
	if len(perfil.Monedas) == 0 {
		return "D"
	}
	var reglas map[string]monedaRegla
	if err := json.Unmarshal(perfil.Monedas, &reglas); err != nil {
		return "D"
	}
	for codigo, regla := range reglas {
		if domain.NormalizeCurrency(codigo, "") != moneda {
			continue
		}
		tipo := strings.ToUpper(strings.TrimSpace(regla.Tipo))
		if tipo == "" {
			tipo = strings.ToUpper(strings.TrimSpace(regla.Def))
		}
		if tipo == "A" || tipo == "P" || tipo == "D" {
			return tipo
		}
	}
	return "D"
}

type monedaRegla struct {
	Tipo string `json:"tipo"`
	Def  string `json:"def"`
}

func rolDe(perfil *Perfil) string {
	if perfil == nil || strings.TrimSpace(perfil.Rol) == "" {
		return "SOCIO"
	}
	return strings.TrimSpace(perfil.Rol)
}

func monedaBaseDe(perfil *Perfil) string {
	if perfil == nil {
		return "USDT"
	}
	return domain.NormalizeCurrency(perfil.MonedaBase, "USDT")
}

func polaridad(value float64) string {
	if value < 0 {
		return "-"
	}
	return "+"
}

// fechaLocal serializa una columna timestamp sin zona como hora de pared de
// Caracas, sin agregar Z ni desplazar el instante.
func fechaLocal(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return clock.WallClock(t).Format("2006-01-02T15:04:05")
}
