package gemini

import (
	"strings"
	"testing"
)

func TestParseJSONToleraMarkdownYAlias(t *testing.T) {
	t.Parallel()

	result, err := parseJSON("Aquí está:\n```json\n{\"amount\":\"1.234,56\",\"currency\":\"usd\",\"bank\":\"BANCO\",\"reference\":\"REF\",\"holder\":\"JOSÉ PÉREZ\"}\n```")
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}
	if result.Monto != 1234.56 {
		t.Fatalf("monto = %v", result.Monto)
	}
	if result.Moneda != "USD" || result.Banco != "BANCO" || result.Referencia != "REF" {
		t.Fatalf("campos = %+v", result)
	}
	if result.Titular != "JOSÉ PÉREZ" {
		t.Fatalf("titular = %q", result.Titular)
	}
}

func TestPromptWithCaptionNoSustituyeLaImagen(t *testing.T) {
	t.Parallel()

	prompt := PromptWithCaption("base", "  pago   del día ")
	if !strings.HasPrefix(prompt, "base") || !strings.Contains(prompt, `"pago del día"`) {
		t.Fatalf("prompt inesperado: %q", prompt)
	}
	if PromptWithCaption("base", " ") != "base" {
		t.Fatal("un caption vacío no debe agregar texto")
	}
}
