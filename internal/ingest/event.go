package ingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"hooty/internal/domain"
)

// ErrNotImage permite responder 200 e ignorar textos, audios y mensajes
// enviados por la propia instancia.
var ErrNotImage = errors.New("ingest: el mensaje no es una imagen")

// ErrInvalidPayload separa un 400 del cliente de un fallo interno (500).
var ErrInvalidPayload = errors.New("ingest: payload inválido")

// Evento es un mensaje de Evolution ya normalizado. Key y Message se
// conservan crudos porque son el contrato exacto de getBase64FromMediaMessage.
type Evento struct {
	Instancia  string          `json:"instancia"`
	HashLargo  string          `json:"hash_largo"`
	HashCorto  string          `json:"hash_corto"`
	Usuario    string          `json:"usuario"`
	IDGrupo    string          `json:"id_grupo"`
	NombrePush string          `json:"nombre_push"`
	Caption    string          `json:"caption"`
	Timestamp  int64           `json:"timestamp"`
	Key        json.RawMessage `json:"-"`
	Message    json.RawMessage `json:"-"`
	MimeType   string          `json:"mime_type"`
}

// Normalize limpia los campos de texto y fija la identidad.
func (e Evento) Normalize() Evento {
	e.Instancia = strings.TrimSpace(e.Instancia)
	if e.Instancia == "" {
		e.Instancia = "hooty"
	}
	e.HashLargo = domain.NormalizeHash(e.HashLargo)
	if strings.TrimSpace(e.HashCorto) == "" {
		e.HashCorto = domain.ShortHash(e.HashLargo)
	}
	e.HashCorto = strings.ToUpper(strings.TrimSpace(e.HashCorto))
	e.Usuario = strings.TrimSpace(e.Usuario)
	e.IDGrupo = strings.TrimSpace(e.IDGrupo)
	e.NombrePush = domain.CleanText(e.NombrePush, 120)
	e.Caption = domain.CleanText(e.Caption, 2000)
	e.MimeType = strings.ToLower(strings.TrimSpace(e.MimeType))
	if e.MimeType == "" {
		e.MimeType = "image/jpeg"
	}
	return e
}

// Validate exige el mínimo para crear un impacto, antes de tocar PostgreSQL.
func (e Evento) Validate() error {
	if e.HashLargo == "" {
		return domain.Validationf("hash_largo es obligatorio")
	}
	if e.HashCorto == "" {
		return domain.Validationf("hash_corto es obligatorio")
	}
	if !domain.IsRealGroup(e.IDGrupo) {
		return domain.Validationf("id_grupo no puede ser vacío ni un placeholder")
	}
	if e.Instancia == "" {
		return domain.Validationf("instancia es obligatoria")
	}
	if len(e.Key) == 0 || len(e.Message) == 0 {
		return domain.Validationf("key y message son obligatorios")
	}
	return nil
}

type envelope struct {
	Instance string          `json:"instance"`
	Data     json.RawMessage `json:"data"`
	Body     json.RawMessage `json:"body"`
}

type messageData struct {
	// Se guardan crudos: un struct tipado descartaría mediaKey, directPath,
	// url, fileEncSha256 y demás campos que Evolution exige al descargar.
	Key              json.RawMessage `json:"key"`
	Message          json.RawMessage `json:"message"`
	PushName         string          `json:"pushName"`
	MessageTimestamp flexInt64       `json:"messageTimestamp"`
	MessageType      string          `json:"messageType"`
	Instance         string          `json:"instance"`
	InstanceID       string          `json:"instanceId"`
}

type keyData struct {
	RemoteJID      string `json:"remoteJid"`
	FromMe         bool   `json:"fromMe"`
	ID             string `json:"id"`
	Participant    string `json:"participant"`
	ParticipantAlt string `json:"participantAlt"`
}

type messageBody struct {
	ImageMessage *imageMessage `json:"imageMessage"`
}

type imageMessage struct {
	Caption    string          `json:"caption"`
	Mimetype   string          `json:"mimetype"`
	FileSha256 json.RawMessage `json:"fileSha256"`
}

// ParseEvento acepta las variantes v1 (body), v2 (data) y los arreglos que
// entrega Evolution. El hash se deriva de fileSha256 o, si falta, del id.
func ParseEvento(raw []byte, defaultInstance string) (Evento, error) {
	data, instance, err := locateData(raw, 0)
	if err != nil {
		return Evento{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	var message messageData
	if err := json.Unmarshal(data, &message); err != nil {
		return Evento{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	var key keyData
	if len(message.Key) > 0 {
		if err := json.Unmarshal(message.Key, &key); err != nil {
			return Evento{}, fmt.Errorf("%w: key inválida: %v", ErrInvalidPayload, err)
		}
	}
	var body messageBody
	if len(message.Message) > 0 {
		if err := json.Unmarshal(message.Message, &body); err != nil {
			return Evento{}, fmt.Errorf("%w: message inválido: %v", ErrInvalidPayload, err)
		}
	}

	if body.ImageMessage == nil && message.MessageType != "imageMessage" {
		return Evento{}, ErrNotImage
	}
	if key.FromMe {
		return Evento{}, ErrNotImage
	}

	hash, err := hashFromMessage(body, key)
	if err != nil {
		return Evento{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	caption := ""
	if body.ImageMessage != nil {
		caption = body.ImageMessage.Caption
	}
	event := Evento{
		Instancia:  firstNonEmpty(instance, message.Instance, message.InstanceID, defaultInstance, "hooty"),
		HashLargo:  domain.NormalizeHash(hash),
		HashCorto:  domain.ShortHash(hash),
		Usuario:    firstNonEmpty(key.ParticipantAlt, key.Participant, key.RemoteJID),
		IDGrupo:    strings.TrimSpace(key.RemoteJID),
		NombrePush: strings.TrimSpace(message.PushName),
		Caption:    domain.CleanText(caption, 2000),
		Timestamp:  int64(message.MessageTimestamp),
		Key:        message.Key,
		Message:    message.Message,
		MimeType:   mimeOf(body.ImageMessage),
	}
	return event.Normalize(), nil
}

func locateData(raw []byte, depth int) (json.RawMessage, string, error) {
	if depth > 4 {
		return nil, "", errors.New("payload demasiado anidado")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, "", errors.New("payload vacío")
	}
	if trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil || len(items) == 0 {
			return nil, "", errors.New("arreglo de eventos vacío")
		}
		return locateData(items[0], depth+1)
	}

	var env envelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, "", err
	}
	if len(env.Data) > 0 {
		return firstObject(env.Data), env.Instance, nil
	}
	if len(env.Body) > 0 {
		if data, instance, err := locateData(env.Body, depth+1); err == nil {
			return data, firstNonEmpty(env.Instance, instance), nil
		}
		return firstObject(env.Body), env.Instance, nil
	}
	return trimmed, env.Instance, nil
}

func firstObject(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err == nil && len(items) > 0 {
			return items[0]
		}
	}
	return trimmed
}

func hashFromMessage(body messageBody, key keyData) (string, error) {
	if body.ImageMessage != nil {
		if hash := decodeFileSha(body.ImageMessage.FileSha256); hash != "" {
			return hash, nil
		}
	}
	id := strings.TrimSpace(key.ID)
	if id == "" {
		return "", errors.New("no se pudo calcular el hash: falta fileSha256 e id")
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:]), nil
}

func decodeFileSha(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	switch trimmed[0] {
	case '{':
		return decodeFileShaObject(trimmed)
	case '[':
		var values []int
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return ""
		}
		return bytesToHex(values)
	case '"':
		var encoded string
		if err := json.Unmarshal(trimmed, &encoded); err != nil {
			return ""
		}
		return decodeFileShaString(encoded)
	}
	return ""
}

func decodeFileShaObject(raw []byte) string {
	var indexed map[string]int
	if err := json.Unmarshal(raw, &indexed); err == nil {
		values := make([]int, 0, 32)
		for index := 0; index < 32; index++ {
			if value, ok := indexed[strconv.Itoa(index)]; ok {
				values = append(values, value)
			}
		}
		if len(values) == 32 {
			return bytesToHex(values)
		}
	}
	var buffer struct {
		Data []int `json:"data"`
	}
	if err := json.Unmarshal(raw, &buffer); err == nil && len(buffer.Data) > 0 {
		return bytesToHex(buffer.Data)
	}
	return ""
}

func bytesToHex(values []int) string {
	if len(values) == 0 {
		return ""
	}
	buffer := make([]byte, len(values))
	for index, value := range values {
		buffer[index] = byte(value)
	}
	return hex.EncodeToString(buffer)
}

func decodeFileShaString(value string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return ""
	}
	if len(clean) == 64 {
		if _, err := hex.DecodeString(clean); err == nil {
			return strings.ToLower(clean)
		}
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(clean); err == nil && len(decoded) > 0 {
			return hex.EncodeToString(decoded)
		}
	}
	return ""
}

func mimeOf(image *imageMessage) string {
	if image == nil {
		return "image/jpeg"
	}
	return firstNonEmpty(image.Mimetype, "image/jpeg")
}

// flexInt64 acepta segundos, milisegundos y cadenas numéricas en el timestamp
// que entrega Evolution.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	value := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if value == "" || value == "null" {
		*f = 0
		return nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		float, floatErr := strconv.ParseFloat(value, 64)
		if floatErr != nil {
			return fmt.Errorf("timestamp inválido: %q", value)
		}
		number = int64(float)
	}
	switch {
	case number > 1_000_000_000_000_000:
		number /= 1_000_000
	case number > 1_000_000_000_000:
		number /= 1_000
	}
	*f = flexInt64(number)
	return nil
}
