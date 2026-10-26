package gemini

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"hooty/internal/domain"
)

// DefaultPrompt fija el contrato de extracción: un objeto JSON, sin markdown.
const DefaultPrompt = `Eres un extractor de datos de comprobantes de pago (transferencias,
depósitos y remesas) enviados por WhatsApp. Analiza la imagen y devuelve
ÚNICAMENTE un objeto JSON válido, sin markdown ni explicaciones, con esta forma:

{
  "monto": 0.00,
  "moneda": "USD",
  "banco": "",
  "referencia": "",
  "titular": ""
}

Reglas:
1. "monto" es el importe numérico transferido. Usa punto como separador decimal
   y no incluyas símbolos, letras ni separadores de miles.
2. "moneda" es el código ISO de la divisa (USD, VES, COP, ARS, PEN, EUR...).
3. "banco" es la entidad emisora o receptora.
4. "referencia" es el número de operación, comprobante o referencia.
5. "titular" es el nombre completo del ordenante o beneficiario.
6. No inventes valores: si un campo no aparece, devuélvelo vacío y el monto 0.
7. Ignora logos, publicidad, sellos y texto ajeno al comprobante.
8. Si la imagen no es un comprobante, devuelve todos los campos vacíos.`

// PromptWithCaption añade el caption como contexto sin sustituir la imagen.
func PromptWithCaption(base, caption string) string {
	clean := domain.CleanText(caption, 2000)
	if clean == "" {
		return base
	}
	return base + "\n\nCaption adjunto al mensaje: \"" + clean + "\""
}

func parseJSON(text string) (domain.Extraccion, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return domain.Extraccion{}, fmt.Errorf("gemini: la respuesta no contiene un objeto JSON")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(text[start:end+1]), &fields); err != nil {
		return domain.Extraccion{}, fmt.Errorf("gemini: JSON inválido: %w", err)
	}
	return fromMap(fields), nil
}

func fromMap(fields map[string]any) domain.Extraccion {
	lookup := make(map[string]any, len(fields))
	for key, value := range fields {
		lookup[strings.ToLower(strings.TrimSpace(key))] = value
	}
	return domain.Extraccion{
		Monto:      parseAmount(pick(lookup, "monto", "amount", "importe", "valor")),
		Moneda:     cleanText(pick(lookup, "moneda", "currency", "divisa"), 12),
		Banco:      cleanText(pick(lookup, "banco", "bank", "entidad", "institucion"), 80),
		Referencia: cleanText(pick(lookup, "referencia", "reference", "ref", "comprobante", "numero"), 80),
		Titular:    cleanText(pick(lookup, "titular", "nombre_titular", "beneficiario", "nombre", "holder"), 120),
	}.Normalize()
}

func pick(fields map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := fields[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func cleanText(value any, limit int) string {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	case float64:
		text = strconv.FormatFloat(typed, 'f', -1, 64)
	}
	return domain.CleanText(text, limit)
}

func parseAmount(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		number, _ := typed.Float64()
		return number
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		return parseAmountString(typed)
	}
	return 0
}

func parseAmountString(raw string) float64 {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r == '.', r == ',', r == '-':
			return r
		}
		return -1
	}, raw)
	if cleaned == "" {
		return 0
	}

	lastDot := strings.LastIndex(cleaned, ".")
	lastComma := strings.LastIndex(cleaned, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastComma > lastDot {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
			cleaned = strings.Replace(cleaned, ",", ".", 1)
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		}
	case lastComma >= 0:
		cleaned = normalizeSingleSeparator(cleaned, ",")
	case lastDot >= 0:
		cleaned = normalizeSingleSeparator(cleaned, ".")
	}
	number, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return number
}

func normalizeSingleSeparator(value, separator string) string {
	parts := strings.Split(value, separator)
	if len(parts) == 1 {
		return value
	}
	decimal := parts[len(parts)-1]
	if len(decimal) > 0 && len(decimal) <= 2 {
		return strings.Join(parts[:len(parts)-1], "") + "." + decimal
	}
	return strings.Join(parts, "")
}
