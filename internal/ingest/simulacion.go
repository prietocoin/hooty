package ingest

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"hooty/internal/domain"
)

// SimulacionInput es el payload de POST /api/v1/comprobantes/simular.
type SimulacionInput struct {
	MonedaRecibo string  `json:"moneda_recibo"`
	MontoRecibo  float64 `json:"monto_recibo"`
	Socio1       string  `json:"socio_1"`
	Socio2       string  `json:"socio_2"`
	LoteTasa     string  `json:"lote_tasa"`
	Tasa1        float64 `json:"tasa_1"`
	Tasa2        float64 `json:"tasa_2"`
}

// SimulacionResult es el contrato de POST /api/v1/comprobantes/simular.
type SimulacionResult struct {
	Tasa1  float64 `json:"tasa_1"`
	Tasa2  float64 `json:"tasa_2"`
	Monto1 float64 `json:"monto_1"`
	Monto2 float64 `json:"monto_2"`
	ME1    float64 `json:"me1"`
	ME2    float64 `json:"me2"`
}

// numeroFlexible acepta números o cadenas numéricas del JSONB de perfiles.
type numeroFlexible struct {
	Valor    float64
	Presente bool
}

func (n *numeroFlexible) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		n.Valor = 0
		n.Presente = false
		return nil
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
	if err != nil {
		n.Valor = 0
		n.Presente = false
		return nil
	}
	n.Valor = value
	n.Presente = true
	return nil
}

// monedaRegla replica el contrato JSONB de perfiles.monedas.
type monedaRegla struct {
	Activo     bool           `json:"activo"`
	Tipo       string         `json:"tipo"`
	Def        string         `json:"def"`
	PolarD     string         `json:"polar_D"`
	PolarP     string         `json:"polar_P"`
	Polaridad  string         `json:"polaridad"`
	Deposito   numeroFlexible `json:"deposito"`
	Pago       numeroFlexible `json:"pago"`
	Porcentaje struct {
		Deposito numeroFlexible `json:"deposito"`
		Pago     numeroFlexible `json:"pago"`
	} `json:"porcentaje"`
}

// SimularLiquidacion reproduce el motor canónico de ingesta sin persistir:
// resuelve cada lado contra el perfil y el lote reales, aplica el spread y la
// polaridad del perfil, y trunca los importes. Si un socio no tiene la divisa
// o falta una tasa, ese lado queda en 0 sin afectar al otro.
func SimularLiquidacion(input SimulacionInput, perfil1, perfil2, fundda domain.Perfil,
	lote domain.LoteTasas) (SimulacionResult, error) {
	if input.MontoRecibo <= 0 || math.IsNaN(input.MontoRecibo) || math.IsInf(input.MontoRecibo, 0) {
		return SimulacionResult{}, domain.Validationf("monto_recibo debe ser positivo")
	}
	if strings.TrimSpace(input.MonedaRecibo) == "" {
		return SimulacionResult{}, domain.Validationf("moneda_recibo es obligatoria")
	}
	if !domain.IsRealSocio(perfil1.Nombre) || !domain.IsRealSocio(perfil2.Nombre) {
		return SimulacionResult{}, domain.Validationf("ambos grupos deben resolver a socios reales")
	}
	if domain.Canonical(perfil1.Nombre) == domain.Canonical(perfil2.Nombre) {
		return SimulacionResult{}, domain.Validationf("los dos grupos resolvieron al mismo socio")
	}
	if strings.TrimSpace(lote.ID) == "" {
		return SimulacionResult{}, domain.Validationf("no hay lote de tasas vigente")
	}
	if tasaInvalida(input.Tasa1) || tasaInvalida(input.Tasa2) {
		return SimulacionResult{}, domain.Validationf("las tasas explícitas deben ser finitas y no negativas")
	}

	currency := domain.NormalizeCurrency(input.MonedaRecibo, "")
	if currency == "" {
		return SimulacionResult{}, domain.Validationf("moneda_recibo es obligatoria")
	}
	efectivo1 := perfilEfectivo(perfil1, fundda)
	efectivo2 := perfilEfectivo(perfil2, fundda)
	tipoOp := tipoOperacion(efectivo1, currency)

	tasa1 := input.Tasa1
	if tasa1 <= 0 {
		tasa1 = tasaDeSocio(efectivo1, currency, tipoOp, lote)
	}
	tasa2 := input.Tasa2
	if tasa2 <= 0 {
		tasa2 = tasaDeSocio(efectivo2, currency, tipoOp, lote)
	}

	signo1, signo2 := signosLado(perfil1, perfil2, efectivo1, efectivo2, currency, tipoOp)
	montoRecibo := math.Abs(input.MontoRecibo)
	base1, ok1 := loteRate(lote, monedaBase(efectivo1))
	base2, ok2 := loteRate(lote, monedaBase(efectivo2))
	if !ok1 {
		base1 = 0
	}
	if !ok2 {
		base2 = 0
	}

	monto1, me1 := calcularMontoLado(signo1, montoRecibo, tasa1, base1)
	monto2, me2 := calcularMontoLado(signo2, montoRecibo, tasa2, base2)

	return SimulacionResult{
		Tasa1:  tasa1,
		Tasa2:  tasa2,
		Monto1: monto1,
		Monto2: monto2,
		ME1:    me1,
		ME2:    me2,
	}, nil
}

func tasaInvalida(tasa float64) bool {
	return tasa < 0 || math.IsNaN(tasa) || math.IsInf(tasa, 0)
}

func monedaBase(perfil domain.Perfil) string {
	return domain.NormalizeCurrency(perfil.MonedaBase, "USDT")
}

// perfilEfectivo aplica la herencia de FUNDDA sobre el catálogo de monedas.
func perfilEfectivo(perfil, fundda domain.Perfil) domain.Perfil {
	if perfil.Herencia && strings.TrimSpace(fundda.Nombre) != "" {
		perfil.Monedas = fundda.Monedas
	}
	return perfil
}

func reglasDe(perfil domain.Perfil) map[string]monedaRegla {
	reglas := map[string]monedaRegla{}
	if len(perfil.Monedas) == 0 {
		return reglas
	}
	var raw map[string]monedaRegla
	if err := json.Unmarshal(perfil.Monedas, &raw); err != nil {
		return reglas
	}
	for codigo, regla := range raw {
		code := domain.NormalizeCurrency(codigo, "")
		if code == "" {
			continue
		}
		reglas[code] = regla
	}
	return reglas
}

func buscarRegla(reglas map[string]monedaRegla, code string) (monedaRegla, bool) {
	regla, ok := reglas[code]
	return regla, ok
}

// tipoOperacion replica la regla canónica: A si la divisa es la base del socio
// 1; si no, el tipo contractual de su perfil; por defecto P.
func tipoOperacion(perfil1 domain.Perfil, currency string) string {
	if currency == monedaBase(perfil1) {
		return "A"
	}
	regla, ok := buscarRegla(reglasDe(perfil1), currency)
	if !ok {
		return "P"
	}
	for _, valor := range []string{regla.Tipo, regla.Def} {
		tipo := strings.ToUpper(strings.TrimSpace(valor))
		if tipo != "" {
			return tipo[:1]
		}
	}
	return "P"
}

// tasaDeSocio calcula la tasa comercial de un lado con su propio spread:
//
//	tasa = (tasa_divisa / tasa_moneda_base) * factor_spread
//
// Sin regla para la divisa, sin tasa de la divisa o sin tasa de su base, el
// resultado es 0. Nunca reutiliza la tasa del otro socio.
func tasaDeSocio(perfil domain.Perfil, currency, tipoOp string, lote domain.LoteTasas) float64 {
	base := monedaBase(perfil)
	regla, tieneRegla := buscarRegla(reglasDe(perfil), currency)
	if tipoOp != "A" && !tieneRegla {
		return 0
	}
	baseDivisa, ok := loteRate(lote, currency)
	if !ok {
		return 0
	}
	baseSocio, ok := loteRate(lote, base)
	if !ok || baseSocio <= 0 {
		return 0
	}
	factor := factorSpread(regla, tipoOp)
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return 0
	}
	return truncarTasaSnapshot((baseDivisa / baseSocio) * factor)
}

// factorSpread aplica el porcentaje de depósito o pago del perfil.
func factorSpread(regla monedaRegla, tipoOp string) float64 {
	deposito := regla.Porcentaje.Deposito
	if !deposito.Presente {
		deposito = regla.Deposito
	}
	pago := regla.Porcentaje.Pago
	if !pago.Presente {
		pago = regla.Pago
	}
	if tipoOp == "D" || tipoOp == "A" {
		return 1 + math.Abs(deposito.Valor)/100
	}
	return 1 - math.Abs(pago.Valor)/100
}

// signosLado resuelve el signo de cada socio con su propia polaridad. Solo la
// regla de herencia o el abono (tipo A) pueden invertir un lado.
func signosLado(perfil1, perfil2, efectivo1, efectivo2 domain.Perfil, currency, tipoOp string) (int, int) {
	if tipoOp == "A" {
		if strings.EqualFold(strings.TrimSpace(perfil1.Nombre), "FUNDDA") {
			return -1, 1
		}
		return 1, -1
	}
	regla1, _ := buscarRegla(reglasDe(efectivo1), currency)
	regla2, _ := buscarRegla(reglasDe(efectivo2), currency)
	signo1 := signoNatural(regla1, tipoOp)
	signo2 := signoNatural(regla2, tipoOp)
	if perfil1.Herencia && !perfil2.Herencia {
		signo1 = -signo2
	} else if perfil2.Herencia && !perfil1.Herencia {
		signo2 = -signo1
	}
	return signo1, signo2
}

func signoNatural(regla monedaRegla, tipoOp string) int {
	if polaridadEfectiva(regla, tipoOp) == "-" {
		return -1
	}
	return 1
}

// polaridadEfectiva lee polar_D / polar_P; polaridad es el respaldo histórico
// del depósito y el pago es su contrapartida cuando falta polar_P.
func polaridadEfectiva(regla monedaRegla, tipoOp string) string {
	deposito := strings.TrimSpace(regla.PolarD)
	if deposito == "" {
		deposito = strings.TrimSpace(regla.Polaridad)
	}
	if deposito == "" {
		deposito = "+"
	}
	switch tipoOp {
	case "P":
		if pago := strings.TrimSpace(regla.PolarP); pago != "" {
			return pago
		}
		if deposito == "-" {
			return "+"
		}
		return "-"
	case "D":
		return deposito
	default:
		return "+"
	}
}

func calcularMontoLado(signo int, montoRecibo, tasa, baseSocio float64) (float64, float64) {
	if tasa <= 0 {
		return 0, 0
	}
	monto := domain.Truncar2(float64(signo) * (montoRecibo / tasa))
	if monto == 0 {
		return 0, 0
	}
	if baseSocio <= 0 {
		return monto, 0
	}
	return monto, domain.Truncar2(monto / baseSocio)
}

// loteRate lee la tasa genérica del lote para una divisa. USD/USDT valen 1.
func loteRate(lote domain.LoteTasas, currency string) (float64, bool) {
	code := domain.NormalizeCurrency(currency, "")
	if code == "" {
		return 0, false
	}
	if rate, ok := lote.Tasas[code]; ok && rate > 0 && !math.IsInf(rate, 0) && !math.IsNaN(rate) {
		return rate, true
	}
	if code == "USD" || code == "USDT" {
		return 1, true
	}
	return 0, false
}

const epsilonTasa = 0.0000001

// truncarTasaSnapshot replica la precisión adaptativa del motor canónico.
func truncarTasaSnapshot(value float64) float64 {
	absolute := math.Abs(value)
	if absolute == 0 || math.IsNaN(absolute) || math.IsInf(absolute, 0) {
		return 1
	}
	rounded := math.Round(absolute*1e8) / 1e8
	var result float64
	switch {
	case rounded > 99.99:
		result = math.Trunc(rounded)
	case rounded >= 10:
		result = math.Trunc((rounded+epsilonTasa)*100) / 100
	default:
		magnitude := math.Floor(math.Log10(rounded))
		factor := math.Pow(10, 2-magnitude)
		result = math.Trunc((rounded+epsilonTasa)*factor) / factor
	}
	return result
}
