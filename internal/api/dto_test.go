package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTipoOperacion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		moneda string
		perfil *Perfil
		want   string
	}{
		{"misma moneda base", "VES", &Perfil{Nombre: "CHASAN", MonedaBase: "VES"}, "A"},
		{"regla P del perfil", "USD", &Perfil{Nombre: "CHASAN", MonedaBase: "VES", Monedas: json.RawMessage(`{"USD":{"tipo":"P"}}`)}, "P"},
		{"regla D del perfil", "USD", &Perfil{Nombre: "CHASAN", MonedaBase: "VES", Monedas: json.RawMessage(`{"USD":{"def":"D"}}`)}, "D"},
		{"sin perfil", "USD", nil, "D"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tipoOperacion(tc.moneda, tc.perfil); got != tc.want {
				t.Fatalf("tipoOperacion = %q, se esperaba %q", got, tc.want)
			}
		})
	}
}

func TestComprobanteDTOAliases(t *testing.T) {
	t.Parallel()

	row := ComprobanteRow{
		HashLargo:      "8a3e",
		HashCorto:      "8A3E",
		Instancia:      "hooty",
		MontoRecibo:    100,
		MonedaRecibo:   "USD",
		Banco:          "BANCO",
		Titular:        "TITULAR",
		Referencia:     "REF",
		URLImagen:      "https://cdn.example/imagen.jpg",
		Grupo1:         "111@g.us",
		Grupo2:         "222@g.us",
		Caption1:       "uno",
		Caption2:       "dos",
		Socio1:         "CHASAN",
		Socio2:         "NELSY",
		Tasa1:          40,
		Tasa2:          41,
		Monto1:         4000,
		Monto2:         4100,
		ME1:            100,
		ME2:            -100,
		LoteTasa:       "T070",
		FechaOperacion: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
		CreadoEn:       time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC),
		ActualizadoEn:  time.Date(2026, 10, 25, 10, 2, 0, 0, time.UTC),
	}
	perfil1 := &Perfil{Nombre: "CHASAN", MonedaBase: "VES", Rol: "SOCIO", Monedas: json.RawMessage(`{"USD":{"tipo":"P"}}`)}
	perfil2 := &Perfil{Nombre: "NELSY", MonedaBase: "COP", Rol: "SOCIO"}

	dto := NewComprobanteDTO(row, perfil1, perfil2)
	if dto.Monto != dto.MontoRecibo || dto.MontoLocal != dto.MontoRecibo {
		t.Fatalf("aliases de monto inconsistentes: %+v", dto)
	}
	if dto.Moneda != "USD" || dto.MonedaRecibo != "USD" {
		t.Fatalf("moneda inesperada: %+v", dto)
	}
	if dto.URLR2Comprobante != dto.URLImagen || dto.LinkImg != dto.URLImagen {
		t.Fatalf("aliases de imagen inconsistentes: %+v", dto)
	}
	if dto.Lote != "T070" || dto.LoteTasaAsignado != "T070" {
		t.Fatalf("aliases de lote inconsistentes: %+v", dto)
	}
	if dto.TipoOp1 != "P" || dto.TipoOp2 != "D" {
		t.Fatalf("tipo_op inesperado: %q / %q", dto.TipoOp1, dto.TipoOp2)
	}
	if !dto.EsX2 {
		t.Fatal("dos socios reales deben marcar es_x2")
	}
	if dto.Polaridad1 != "+" || dto.Polaridad2 != "-" {
		t.Fatalf("polaridad inesperada: %q / %q", dto.Polaridad1, dto.Polaridad2)
	}
	if dto.FechaOperacion != "2026-10-25T10:00:00" {
		t.Fatalf("fecha_operacion = %q", dto.FechaOperacion)
	}
	if dto.FechaHoraComprobante != dto.FechaOperacion {
		t.Fatalf("alias de fecha inconsistente: %+v", dto)
	}
}

func TestPerfilDTOAliases(t *testing.T) {
	t.Parallel()

	dto := NewPerfilDTO(Perfil{
		Nombre:       "CHASAN",
		IDGrupo:      "111@g.us",
		Rol:          "SOCIO",
		MonedaBase:   "ves",
		SaldoInicial: 10,
		Mostrar:      json.RawMessage(`{"tasas":true}`),
		Monedas:      json.RawMessage(`{"USD":{"tipo":"P"}}`),
		Herencia:     true,
	})
	if dto.WhatsApp != "111@g.us" || dto.Roles != "SOCIO" || dto.MonedaSocio != "VES" {
		t.Fatalf("aliases de perfil inconsistentes: %+v", dto)
	}
	if dto.SaldoAnterior != 10 || !dto.Herencia {
		t.Fatalf("datos de perfil inconsistentes: %+v", dto)
	}
}

func TestFechaLocalNoDesplazaLaHora(t *testing.T) {
	t.Parallel()

	value := time.Date(2026, 10, 25, 11, 15, 0, 0, time.UTC)
	if got := fechaLocal(value); got != "2026-10-25T11:15:00" {
		t.Fatalf("fechaLocal = %q", got)
	}
	if fechaLocal(time.Time{}) != "" {
		t.Fatal("una fecha cero debe serializar vacío")
	}
}
