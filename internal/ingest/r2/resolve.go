package r2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PublicURL devuelve la URL pública de un objeto. Si el valor recibido ya es
// una URL absoluta se conserva; si es una clave, se reconstruye con el dominio
// público configurado (o con el endpoint S3 como respaldo).
func (s *Store) PublicURL(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	key := strings.TrimLeft(raw, "/")
	if s.publicDomain != "" {
		return s.publicDomain + "/" + escapePath(key)
	}
	return s.endpointURL(key)
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
			return raw, nil
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
		urls := []string{s.PublicURL(candidate)}
		if s.publicDomain != "" {
			urls = append(urls, s.endpointURL(candidate))
		}
		for _, url := range urls {
			if url == "" {
				continue
			}
			if s.exists(ctx, url) {
				return url, nil
			}
		}
	}
	if raw != "" {
		// Se conserva la referencia original aunque el HEAD no sea concluyente:
		// puede ser una URL firmada de corta duración o un bucket privado.
		return s.PublicURL(raw), nil
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
