package ingest

import (
	"fmt"
	"math"
	"strings"
	"time"

	"hooty/internal/domain"
)

// CalcularLiquidacion congela el snapshot base 2x.
//
// Fórmula v2 inicial: cada tasa de lado es la razón entre la tasa del recibo y
// la tasa de la moneda base del socio; el importe del lado se obtiene dividiendo
// el monto del recibo por esa tasa, y el equivalente ME se obtiene dividiendo el
// importe del lado por la tasa de su moneda base. Las tasas del lote se asumen
// expresadas contra la misma referencia (USDT/USD = 1).
//
// La polaridad, el spread y la herencia de perfiles son reglas P1: se
// incorporarán cuando el contrato financiero quede aprobado. Los CHECK de la
// base y Comprobante.Validate siguen siendo la última defensa.
func CalcularLiquidacion(input domain.LiquidacionInput, perfil1, perfil2 domain.Perfil,
	lote domain.LoteTasas, now time.Time) (domain.Comprobante, error) {
	if !domain.IsRealGroup(input.Grupo1) || !domain.IsRealGroup(input.Grupo2) {
		return domain.Comprobante{}, domain.Validationf("el binomio requiere dos grupos reales")
	}
	if domain.Canonical(input.Grupo1) == domain.Canonical(input.Grupo2) {
		return domain.Comprobante{}, domain.Validationf("los grupos del binomio deben ser distintos")
	}
	if !domain.IsRealSocio(perfil1.Nombre) || !domain.IsRealSocio(perfil2.Nombre) {
		return domain.Comprobante{}, domain.Validationf("ambos grupos deben resolver a socios reales")
	}
	if domain.Canonical(perfil1.Nombre) == domain.Canonical(perfil2.Nombre) {
		return domain.Comprobante{}, domain.Validationf("los dos grupos resolvieron al mismo socio")
	}
	if strings.TrimSpace(lote.ID) == "" {
		return domain.Comprobante{}, domain.Validationf("no hay lote de tasas vigente")
	}
	if input.Extraccion.Monto <= 0 || math.IsNaN(input.Extraccion.Monto) ||
		math.IsInf(input.Extraccion.Monto, 0) {
		return domain.Comprobante{}, domain.Validationf("el monto del recibo debe ser positivo")
	}
	currency := domain.NormalizeCurrency(input.Extraccion.Moneda, "")
	if currency == "" {
		return domain.Comprobante{}, domain.Validationf("la moneda del recibo es obligatoria")
	}
	if now.IsZero() {
		return domain.Comprobante{}, domain.Validationf("la hora de creación es obligatoria")
	}

	amount := math.Abs(input.Extraccion.Monto)
	receiptRate, err := rateFor(lote, currency)
	if err != nil {
		return domain.Comprobante{}, err
	}
	tasa1, monto1, me1, err := side(perfil1, amount, receiptRate, lote)
	if err != nil {
		return domain.Comprobante{}, fmt.Errorf("lado 1: %w", err)
	}
	tasa2, monto2, me2, err := side(perfil2, amount, receiptRate, lote)
	if err != nil {
		return domain.Comprobante{}, fmt.Errorf("lado 2: %w", err)
	}

	fechaOperacion := input.FechaOperacion
	if fechaOperacion.IsZero() {
		fechaOperacion = now
	}
	comprobante := domain.Comprobante{
		HashLargo:      input.HashLargo,
		HashCorto:      input.HashCorto,
		Instancia:      input.Instancia,
		MontoRecibo:    domain.Round2(amount),
		MonedaRecibo:   currency,
		Banco:          input.Extraccion.Banco,
		Titular:        input.Extraccion.Titular,
		Referencia:     input.Extraccion.Referencia,
		URLImagen:      input.URLImagen,
		Grupo1:         input.Grupo1,
		Grupo2:         input.Grupo2,
		Caption1:       input.Caption1,
		Caption2:       input.Caption2,
		Socio1:         perfil1.Nombre,
		Socio2:         perfil2.Nombre,
		Tasa1:          tasa1,
		Tasa2:          tasa2,
		Monto1:         monto1,
		Monto2:         monto2,
		ME1:            me1,
		ME2:            me2,
		LoteTasa:       lote.ID,
		FechaOperacion: fechaOperacion,
		CreadoEn:       now,
		ActualizadoEn:  now,
	}
	comprobante = domain.NormalizeComprobante(comprobante)
	if err := comprobante.Validate(); err != nil {
		return domain.Comprobante{}, err
	}
	return comprobante, nil
}

func side(perfil domain.Perfil, amount, receiptRate float64, lote domain.LoteTasas) (float64, float64, float64, error) {
	if strings.TrimSpace(perfil.MonedaBase) == "" {
		return 0, 0, 0, domain.Validationf("el socio %q no tiene moneda base", perfil.Nombre)
	}
	baseRate, err := rateFor(lote, perfil.MonedaBase)
	if err != nil {
		return 0, 0, 0, err
	}
	tasa := domain.RoundRate(receiptRate / baseRate)
	if tasa <= 0 {
		return 0, 0, 0, domain.Validationf("tasa no positiva para %q", perfil.Nombre)
	}
	monto := domain.Round2(amount / tasa)
	me := domain.Round2(monto / baseRate)
	if math.Abs(monto) < 0.01 {
		return tasa, monto, me, domain.Validationf("el importe de %q se trunca a cero", perfil.Nombre)
	}
	if math.Abs(me) < 0.01 {
		return tasa, monto, me, domain.Validationf("el equivalente de %q se trunca a cero", perfil.Nombre)
	}
	return tasa, monto, me, nil
}

func rateFor(lote domain.LoteTasas, currency string) (float64, error) {
	code := domain.NormalizeCurrency(currency, "")
	if code == "" {
		return 0, domain.Validationf("la moneda es obligatoria")
	}
	if rate, ok := lote.Tasas[code]; ok && rate > 0 {
		return rate, nil
	}
	if code == "USD" || code == "USDT" {
		return 1, nil
	}
	return 0, domain.Validationf("el lote %q no tiene una tasa para %s", lote.ID, code)
}
