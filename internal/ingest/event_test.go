package ingest_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"hooty/internal/ingest"
)

func TestParseEventoExtraeIdentidadYTimestamp(t *testing.T) {
	t.Parallel()

	hash := strings.Repeat("AB", 32)
	payload := `{
		"instance": "Glauk",
		"data": {
			"key": {
				"remoteJid": "120363000000000001@g.us",
				"fromMe": false,
				"id": "MSG1",
				"participant": "584140000000@s.whatsapp.net"
			},
			"message": {
				"imageMessage": {
					"caption": "  pago   de prueba ",
					"mimetype": "image/png",
					"fileSha256": "` + hash + `"
				}
			},
			"messageTimestamp": 1700000000000,
			"pushName": "Jairo"
		}
	}`

	event, err := ingest.ParseEvento([]byte(payload), "fallback")
	if err != nil {
		t.Fatalf("ParseEvento: %v", err)
	}
	if event.HashLargo != strings.ToLower(hash) {
		t.Fatalf("hash = %q", event.HashLargo)
	}
	if event.HashCorto != "ABABABAB" {
		t.Fatalf("hash corto = %q", event.HashCorto)
	}
	if event.IDGrupo != "120363000000000001@g.us" {
		t.Fatalf("grupo = %q", event.IDGrupo)
	}
	if event.Instancia != "Glauk" {
		t.Fatalf("instancia = %q", event.Instancia)
	}
	if event.Caption != "pago de prueba" {
		t.Fatalf("caption = %q", event.Caption)
	}
	if event.Timestamp != 1_700_000_000 {
		t.Fatalf("timestamp = %d", event.Timestamp)
	}
	if event.MimeType != "image/png" {
		t.Fatalf("mime = %q", event.MimeType)
	}
	if len(event.Key) == 0 || len(event.Message) == 0 {
		t.Fatal("key/message deben conservarse crudos")
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("evento válido rechazado: %v", err)
	}
}

func TestParseEventoUsaIDCuandoFaltaFileSha(t *testing.T) {
	t.Parallel()

	payload := `{
		"data": {
			"key": {"remoteJid": "120363000000000001@g.us", "id": "MSG-FALLBACK"},
			"message": {"imageMessage": {"caption": ""}},
			"messageTimestamp": 1700000000
		}
	}`
	event, err := ingest.ParseEvento([]byte(payload), "hooty")
	if err != nil {
		t.Fatalf("ParseEvento: %v", err)
	}
	sum := sha256.Sum256([]byte("MSG-FALLBACK"))
	expected := hex.EncodeToString(sum[:])
	if event.HashLargo != expected {
		t.Fatalf("hash de respaldo = %q, se esperaba %q", event.HashLargo, expected)
	}
	if event.Instancia != "hooty" {
		t.Fatalf("instancia por defecto = %q", event.Instancia)
	}
}

func TestParseEventoIgnoraMensajesNoImagen(t *testing.T) {
	t.Parallel()

	_, err := ingest.ParseEvento([]byte(`{
		"data": {
			"key": {"remoteJid": "120363000000000001@g.us", "id": "TXT"},
			"message": {"conversation": "hola"},
			"messageType": "conversation"
		}
	}`), "hooty")
	if !errors.Is(err, ingest.ErrNotImage) {
		t.Fatalf("se esperaba ErrNotImage, obtuve %v", err)
	}
}

func TestParseEventoIgnoraFromMe(t *testing.T) {
	t.Parallel()

	_, err := ingest.ParseEvento([]byte(`{
		"data": {
			"key": {"remoteJid": "120363000000000001@g.us", "fromMe": true, "id": "OWN"},
			"message": {"imageMessage": {"fileSha256": "`+strings.Repeat("CD", 32)+`"}}
		}
	}`), "hooty")
	if !errors.Is(err, ingest.ErrNotImage) {
		t.Fatalf("se esperaba ErrNotImage, obtuve %v", err)
	}
}

func TestParseEventoRechazaPayloadVacio(t *testing.T) {
	t.Parallel()

	_, err := ingest.ParseEvento(nil, "hooty")
	if !errors.Is(err, ingest.ErrInvalidPayload) {
		t.Fatalf("se esperaba ErrInvalidPayload, obtuve %v", err)
	}
}

func TestEventoValidateRechazaGrupoPlaceholder(t *testing.T) {
	t.Parallel()

	event := testEvent()
	event.IDGrupo = "GENERAL"
	if err := event.Validate(); err == nil {
		t.Fatal("un grupo placeholder debe rechazarse")
	}
}
