// Package r2 publica imágenes en Cloudflare R2 mediante la API S3 y firma
// SigV4. Se evita el SDK para mantener el binario pequeño y el contrato
// explícito: una clave determinista y un PUT idempotente.
package r2

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config reúne las credenciales de R2.
type Config struct {
	Endpoint     string
	Bucket       string
	AccessKey    string
	SecretKey    string
	PublicDomain string
	Region       string
	Timeout      time.Duration
	Now          func() time.Time
}

// Store implementa ingest.ImageStore.
type Store struct {
	endpoint     *url.URL
	bucket       string
	accessKey    string
	secretKey    string
	publicDomain string
	region       string
	http         *http.Client
	now          func() time.Time
}

// New valida la configuración y construye el publicador.
func New(cfg Config) (*Store, error) {
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("r2: endpoint inválido %q", cfg.Endpoint)
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return nil, fmt.Errorf("r2: endpoint debe ser http(s)")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("r2: R2_BUCKET_NAME es obligatorio")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("r2: R2_ACCESS_KEY_ID y R2_SECRET_ACCESS_KEY son obligatorios")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	return &Store{
		endpoint:     endpoint,
		bucket:       strings.TrimSpace(cfg.Bucket),
		accessKey:    strings.TrimSpace(cfg.AccessKey),
		secretKey:    strings.TrimSpace(cfg.SecretKey),
		publicDomain: strings.TrimRight(strings.TrimSpace(cfg.PublicDomain), "/"),
		region:       region,
		http:         &http.Client{Timeout: cfg.Timeout},
		now:          cfg.Now,
	}, nil
}

// Subir hace un PUT firmado. La clave se normaliza sin barra inicial.
func (s *Store) Subir(ctx context.Context, key string, data []byte, mimeType string) (string, error) {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if key == "" {
		return "", errors.New("r2: la clave del objeto es obligatoria")
	}
	if len(data) == 0 {
		return "", errors.New("r2: no se puede subir una imagen vacía")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = http.DetectContentType(data)
	}

	canonicalURI := "/" + s.bucket + "/" + escapePath(key)
	now := s.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256Hex(data)
	canonicalHeaders := "host:" + s.endpoint.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := date + "/" + s.region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := sign(s.secretKey, date, s.region, stringToSign)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.endpoint.Scheme+"://"+s.endpoint.Host+canonicalURI, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("r2: construir PUT: %w", err)
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("r2: subir objeto: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("r2: estado %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if s.publicDomain != "" {
		return s.publicDomain + "/" + escapePath(key), nil
	}
	return s.endpoint.Scheme + "://" + s.endpoint.Host + "/" + s.bucket + "/" + escapePath(key), nil
}

func sign(secret, date, region, stringToSign string) string {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	return hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
