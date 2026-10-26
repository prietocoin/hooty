// Package dbschema valida el contrato de columnas y tipos que Hooty espera de
// PostgreSQL antes de aceptar tráfico.
package dbschema

import (
	"fmt"
	"sort"
	"strings"
)

// Kind es una familia de tipos compatible. No se compara el OID exacto porque
// `information_schema` normaliza los nombres y algunas columnas admiten más de
// una representación (por ejemplo varchar/text).
type Kind string

const (
	Text          Kind = "texto"
	Integer       Kind = "entero"
	Boolean       Kind = "booleano"
	Numeric       Kind = "numérico"
	JSON          Kind = "json"
	TimestampWall Kind = "timestamp without time zone"
)

// Column describe el tipo esperado de una columna.
type Column struct {
	Kind Kind
}

// Schema es table -> column -> contrato.
type Schema map[string]map[string]Column

// Validate compara las columnas encontradas (table -> column -> data_type) con
// el contrato requerido. Devuelve un error que incluye columnas faltantes y
// tipos incompatibles, con el tipo esperado y el encontrado.
func Validate(found map[string]map[string]string, required Schema) error {
	var missing []string
	var incompatible []string

	for table, columns := range required {
		for name, spec := range columns {
			dataType, ok := found[table][name]
			if !ok {
				missing = append(missing,
					fmt.Sprintf("%s.%s (esperado %s, no encontrada)", table, name, spec.Kind))
				continue
			}
			if !Compatible(spec.Kind, dataType) {
				incompatible = append(incompatible,
					fmt.Sprintf("%s.%s (esperado %s, encontrado %s)", table, name, spec.Kind, dataType))
			}
		}
	}
	sort.Strings(missing)
	sort.Strings(incompatible)

	var details []string
	if len(missing) > 0 {
		details = append(details, "faltan columnas: "+strings.Join(missing, ", "))
	}
	if len(incompatible) > 0 {
		details = append(details, "tipos incompatibles: "+strings.Join(incompatible, "; "))
	}
	if len(details) > 0 {
		return fmt.Errorf("schema: %s", strings.Join(details, " | "))
	}
	return nil
}

// Compatible indica si un data_type de information_schema pertenece a la
// familia esperada. Los timestamps contables son estrictos: solo se acepta
// "timestamp without time zone" para proteger la hora de pared de Caracas.
func Compatible(kind Kind, dataType string) bool {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	switch kind {
	case Text:
		return dataType == "text" ||
			dataType == "character varying" ||
			dataType == "character" ||
			dataType == "bpchar"
	case Integer:
		return dataType == "bigint" ||
			dataType == "integer" ||
			dataType == "smallint"
	case Boolean:
		return dataType == "boolean"
	case Numeric:
		return dataType == "numeric" ||
			dataType == "decimal" ||
			dataType == "double precision" ||
			dataType == "real"
	case JSON:
		return dataType == "json" ||
			dataType == "jsonb" ||
			// Algunas vistas históricas exponen JSON serializado como texto.
			dataType == "text" ||
			dataType == "character varying"
	case TimestampWall:
		return dataType == "timestamp without time zone"
	default:
		return false
	}
}
