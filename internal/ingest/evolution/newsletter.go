package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxNewsletterResponseBytes = 4 << 20

// ResolveNewsletterJID resuelve el JID (@newsletter) de un canal a partir de su
// código de invitación. Evolution v2.3.7 no emite webhooks de canales, así que
// se consulta su API interna: primero el metadata del canal y, si no responde,
// la lista completa de newsletters de la instancia.
func (c *Client) ResolveNewsletterJID(ctx context.Context, inviteCode string) (string, error) {
	code := strings.TrimSpace(inviteCode)
	if code == "" {
		return "", errors.New("evolution: inviteCode es obligatorio")
	}

	metadataPath := "/chat/getNewsletterMetadata/" + url.PathEscape(c.instance)
	metadata, metadataStatus, metadataErr := c.getJSON(ctx, metadataPath, url.Values{"inviteCode": []string{code}})
	if metadataErr == nil {
		if jid := firstNewsletterJID(metadata); jid != "" {
			return jid, nil
		}
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	listPath := "/chat/findNewsletters/" + url.PathEscape(c.instance)
	list, listStatus, listErr := c.getJSON(ctx, listPath, nil)
	if listErr == nil {
		if jid := newsletterJIDByInvite(list, code); jid != "" {
			return jid, nil
		}
		if jid := soleNewsletterJID(list); jid != "" {
			return jid, nil
		}
	}

	return "", fmt.Errorf(
		"evolution: no se pudo resolver el canal %q (metadata status=%d, lista status=%d): metadata=%v, lista=%v",
		code, metadataStatus, listStatus, metadataErr, listErr,
	)
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values) (any, int, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("evolution: construir consulta de newsletter: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("apikey", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("evolution: consultar newsletter: %w", err)
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxNewsletterResponseBytes))
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("evolution: leer newsletter: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("evolution: estado %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("evolution: decodificar newsletter: %w", err)
	}
	return parsed, resp.StatusCode, nil
}

func isNewsletterJID(value string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(value)), "@newsletter")
}

func inviteMatches(invite, code string) bool {
	value := strings.TrimSpace(invite)
	return strings.EqualFold(value, code) || strings.Contains(value, code)
}

func firstNewsletterJID(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"id", "jid", "channelId", "newsletterId", "newsletter_id"} {
			if candidate, ok := typed[key].(string); ok && isNewsletterJID(candidate) {
				return strings.TrimSpace(candidate)
			}
		}
		for _, child := range typed {
			if jid := firstNewsletterJID(child); jid != "" {
				return jid
			}
		}
	case []any:
		for _, child := range typed {
			if jid := firstNewsletterJID(child); jid != "" {
				return jid
			}
		}
	case string:
		if isNewsletterJID(typed) {
			return strings.TrimSpace(typed)
		}
	}
	return ""
}

func newsletterJIDByInvite(value any, code string) string {
	switch typed := value.(type) {
	case map[string]any:
		id := ""
		for _, key := range []string{"id", "jid", "channelId", "newsletterId", "newsletter_id"} {
			if candidate, ok := typed[key].(string); ok && isNewsletterJID(candidate) {
				id = strings.TrimSpace(candidate)
				break
			}
		}
		if id != "" {
			for _, key := range []string{"invite", "inviteCode", "invite_code", "code"} {
				if invite, ok := typed[key].(string); ok && inviteMatches(invite, code) {
					return id
				}
			}
		}
		for _, child := range typed {
			if jid := newsletterJIDByInvite(child, code); jid != "" {
				return jid
			}
		}
	case []any:
		for _, child := range typed {
			if jid := newsletterJIDByInvite(child, code); jid != "" {
				return jid
			}
		}
	}
	return ""
}

func soleNewsletterJID(value any) string {
	seen := map[string]struct{}{}
	collectNewsletterJIDs(value, seen)
	if len(seen) != 1 {
		return ""
	}
	for jid := range seen {
		return jid
	}
	return ""
}

func collectNewsletterJIDs(value any, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"id", "jid", "channelId", "newsletterId", "newsletter_id"} {
			if candidate, ok := typed[key].(string); ok && isNewsletterJID(candidate) {
				seen[strings.TrimSpace(candidate)] = struct{}{}
			}
		}
		for _, child := range typed {
			collectNewsletterJIDs(child, seen)
		}
	case []any:
		for _, child := range typed {
			collectNewsletterJIDs(child, seen)
		}
	case string:
		if isNewsletterJID(typed) {
			seen[strings.TrimSpace(typed)] = struct{}{}
		}
	}
}
