package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Setup enables a local JSON exporter when explicitly requested. Production
// deployments can replace this exporter with their collector configuration.
func Setup(enabled bool) (func(context.Context) error, error) {
	if !enabled {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	if err != nil {
		return nil, err
	}
	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithSampler(trace.TraceIDRatioBased(0.1)),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
