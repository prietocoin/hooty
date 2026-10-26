package postgres

import (
	"strings"
	"testing"

	"hooty/internal/dbschema"
)

func TestRequiredSchemaProtectsCaracasTimestamps(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		table  string
		column string
	}{
		{"impactos", "recibido_en"},
		{"comprobantes", "fecha_operacion"},
		{"comprobantes", "creado_en"},
		{"comprobantes", "actualizado_en"},
	} {
		spec, ok := requiredSchema[tc.table][tc.column]
		if !ok {
			t.Fatalf("falta %s.%s en requiredSchema", tc.table, tc.column)
		}
		if spec.Kind != dbschema.TimestampWall {
			t.Fatalf("%s.%s debe ser timestamp without time zone, es %s", tc.table, tc.column, spec.Kind)
		}
	}
}

func TestRequiredSchemaRejectsWrongTypeAndMissingColumn(t *testing.T) {
	t.Parallel()

	found := foundForSchema(requiredSchema)
	found["comprobantes"]["fecha_operacion"] = "timestamp with time zone"
	if err := dbschema.Validate(found, requiredSchema); err == nil ||
		!strings.Contains(err.Error(), "fecha_operacion") ||
		!strings.Contains(err.Error(), "timestamp with time zone") {
		t.Fatalf("se esperaba error de tipo para fecha_operacion: %v", err)
	}

	found = foundForSchema(requiredSchema)
	delete(found["impactos"], "recibido_en")
	if err := dbschema.Validate(found, requiredSchema); err == nil ||
		!strings.Contains(err.Error(), "impactos.recibido_en") {
		t.Fatalf("se esperaba error por columna faltante: %v", err)
	}
}

func foundForSchema(schema dbschema.Schema) map[string]map[string]string {
	dataType := func(kind dbschema.Kind) string {
		switch kind {
		case dbschema.Text:
			return "text"
		case dbschema.Integer:
			return "bigint"
		case dbschema.Boolean:
			return "boolean"
		case dbschema.Numeric:
			return "numeric"
		case dbschema.JSON:
			return "jsonb"
		case dbschema.TimestampWall:
			return "timestamp without time zone"
		default:
			return ""
		}
	}
	found := make(map[string]map[string]string, len(schema))
	for table, columns := range schema {
		found[table] = make(map[string]string, len(columns))
		for name, spec := range columns {
			found[table][name] = dataType(spec.Kind)
		}
	}
	return found
}
