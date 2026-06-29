// Package telemetry wires optional OpenTelemetry tracing. It is a no-op unless
// OTEL_EXPORTER_OTLP_ENDPOINT is set, so the default $0 run pays nothing for it.
package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Enabled reports whether an OTLP endpoint is configured.
func Enabled() bool { return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" }

// Init installs a global tracer provider exporting spans over OTLP/HTTP when
// OTEL_EXPORTER_OTLP_ENDPOINT is set (the exporter reads the standard OTEL_*
// env vars). It always sets the W3C propagator so trace context flows to the AI
// service. The returned shutdown flushes pending spans; it is safe to call when
// tracing is disabled.
func Init(ctx context.Context, serviceName, version string) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	// Propagate trace context on outgoing calls even when not exporting, so a
	// collector placed in front still sees a connected trace.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if !Enabled() {
		return noop, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", version),
	))
	if err != nil {
		return noop, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
