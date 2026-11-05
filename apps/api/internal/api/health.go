package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

func (server *Server) allowRate(request *http.Request, key string, limit int) (bool, int) {
	if server.cache != nil && strings.TrimSpace(server.cfg.RedisURL) != "" {
		ctx, cancel := context.WithTimeout(request.Context(), 100*time.Millisecond)
		defer cancel()
		distributedKey := hashRateKey(key)
		if allowed, retry, err := server.cache.AllowRate(ctx, distributedKey, limit, time.Minute); err == nil {
			return allowed, retry
		}
	}
	return server.limiter.allow(key, limit)
}

func hashRateKey(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

func (server *Server) readiness(writer http.ResponseWriter, request *http.Request) {
	checks := map[string]map[string]any{}
	ready := true

	dbCheck := map[string]any{"status": "UP"}
	ctx, cancel := context.WithTimeout(request.Context(), 750*time.Millisecond)
	if err := server.store.DB.PingContext(ctx); err != nil {
		dbCheck["status"] = "DOWN"
		dbCheck["error"] = "sqlite unavailable"
		ready = false
	}
	cancel()
	checks["sqlite"] = dbCheck

	redisCheck := map[string]any{"status": "DOWN", "configured": strings.TrimSpace(server.cfg.RedisURL) != ""}
	if redisCheck["configured"] == true {
		ctx, cancel = context.WithTimeout(request.Context(), 750*time.Millisecond)
		if err := server.cache.Ping(ctx); err == nil {
			redisCheck["status"] = "UP"
		} else {
			redisCheck["error"] = "redis unavailable"
			ready = false
		}
		cancel()
	} else {
		redisCheck["error"] = "redis is not configured"
		ready = false
	}
	checks["redis"] = redisCheck

	cdnCheck := map[string]any{"status": "DOWN", "configured": strings.TrimSpace(server.cfg.CDNURL) != ""}
	if cdnCheck["configured"] == true {
		ctx, cancel = context.WithTimeout(request.Context(), 750*time.Millisecond)
		cdnRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(server.cfg.CDNURL, "/")+"/readyz", nil)
		if err == nil {
			response, requestErr := server.client.Do(cdnRequest)
			if requestErr == nil {
				_ = response.Body.Close()
				if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
					cdnCheck["status"] = "UP"
				} else {
					cdnCheck["error"] = "cdn readiness check failed"
					ready = false
				}
			} else {
				cdnCheck["error"] = "cdn unavailable"
				ready = false
			}
		} else {
			cdnCheck["error"] = "invalid cdn URL"
			ready = false
		}
		cancel()
	} else {
		cdnCheck["error"] = "cdn is not configured"
		ready = false
	}
	checks["cdn"] = cdnCheck

	status := "UP"
	code := http.StatusOK
	if !ready {
		status = "DOWN"
		code = http.StatusServiceUnavailable
	}
	writeJSON(writer, code, map[string]any{"status": status, "checks": checks})
}
