package handlers

import (
	"net"
	"net/http"
	"strings"
	"time"
)

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func requestUserAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.UserAgent())
}

func parseRFC3339Param(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	u := t.UTC()
	return &u, nil
}
