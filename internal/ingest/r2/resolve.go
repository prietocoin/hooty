package r2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PublicURL devuelve la URL pública de un objeto. Si el valor recibido es una
// URL absoluta del endpoint S3 interno se reescribe con el dominio público;
// las URLs externas se conservan. Nunca devuelve *.r2.cloudflarestorage.com
// cuando hay una base pública configurada.
func (s *Store) PublicURL(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if parsed, err := url.Parse(raw); err == nil && esEndpointS3(parsed, s.endpoint) {
			if key := s.keyFromEndpointPath(parsed.Path); key != "" {
				if rewritten := s.publicObjectURL(key); rewritten != "" {
					return rewritten
				}
			}
		}
		return raw
	}
	return s.publicObjectURL(raw)
}

// esEndpointS3 reconoce tanto el endpoint configurado como cualquier host
// interno de Cloudflare R2 que nunca debe exponerse.
func esEndpointS3(parsed, endpoint *url.URL) bool {
	if parsed == nil || endpoint == nil {
		return false
	}
	if strings.EqualFold(parsed.Host, endpoint.Host) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".r2.cloudflarestorage.com")
}

// keyFromEndpointPath extrae la clave de un path estilo S3
// (/<bucket>/<clave>) sin depender de la forma exacta del endpoint.
func (s *Store) keyFromEndpointPath(path string) string {
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	prefix := s.bucket + "/"
	if strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
	}
	return path
}

func (s *Store) endpointURL(key string) string {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	return s.endpoint.Scheme + "://" + s.endpoint.Host + "/" + s.bucket + "/" + escapePath(key)
}

// ResolveImageURL comprueba las claves candidatas del comprobante en R2. Se
// prueba primero la URL/clave actual y después las extensiones habituales
// usando el hash largo, que es la clave determinista de la ingesta.
func (s *Store) ResolveImageURL(ctx context.Context, hash, current string) (string, error) {
	hash = strings.TrimSpace(hash)
	raw := strings.TrimSpace(current)
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if existing == value {
				return
			}
		}
		candidates = append(candidates, value)
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if s.exists(ctx, raw) {
			return s.PublicURL(raw), nil
		}
	} else if raw != "" {
		add(strings.TrimLeft(raw, "/"))
	}
	if hash != "" {
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
			add("comprobantes/" + hash + ext)
		}
		add("comprobantes/" + hash)
	}

	for _, candidate := range candidates {
		publicURL := s.PublicURL(candidate)
		if publicURL == "" {
			continue
		}
		if s.exists(ctx, publicURL) {
			return publicURL, nil
		}
		// El endpoint S3 solo se usa para verificar; la URL devuelta siempre
		// es la pública.
		if s.publicDomain != "" {
			if endpoint := s.endpointURL(candidate); endpoint != publicURL && s.exists(ctx, endpoint) {
				return publicURL, nil
			}
		}
	}
	if raw != "" {
		// Se conserva la referencia original aunque el HEAD no sea concluyente:
		// puede ser una URL firmada de corta duración o un bucket privado.
		if resolved := s.PublicURL(raw); resolved != "" {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("r2: no se encontró la imagen del comprobante %q", hash)
}

func (s *Store) exists(ctx context.Context, rawURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false
	}
	if strings.EqualFold(req.URL.Host, s.endpoint.Host) {
		req.Header.Set("x-amz-content-sha256", emptyPayloadHash)
		s.sign(req, emptyPayloadHash)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func (s *Store) sign(req *http.Request, payloadHash string) {
	now := s.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := date + "/" + s.region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(sum[:]),
	}, "\n")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+sign(s.secretKey, date, s.region, stringToSign))
}
