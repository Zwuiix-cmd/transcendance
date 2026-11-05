package telemetry

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	route "card-arena/api/internal/api/route"
)

type Route struct{ route.Route }

type WebVital struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Path  string  `json:"path"`
}

func Handle(context *route.Context) {
	var vital WebVital
	decoder := json.NewDecoder(http.MaxBytesReader(context.Writer, context.Request.Body, 4096))
	if decoder.Decode(&vital) != nil || !valid(vital) {
		context.Writer.WriteHeader(http.StatusNoContent)
		return
	}
	slog.Info("web vital", "metric", vital.Name, "value", vital.Value, "path", vital.Path, "user_agent", context.Request.UserAgent())
	context.Writer.WriteHeader(http.StatusNoContent)
}

func valid(vital WebVital) bool {
	if vital.Value < 0 || vital.Value > 120000 || len(vital.Path) > 512 {
		return false
	}
	switch strings.ToUpper(vital.Name) {
	case "LCP", "CLS", "INP", "FCP", "TTFB":
		return true
	default:
		return false
	}
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodPost, "/api/v1/telemetry/vitals", route.AccessPublic)}
