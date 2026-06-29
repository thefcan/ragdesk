package telemetry

import (
	"context"
	"testing"
)

func TestInitDisabledIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if Enabled() {
		t.Fatal("Enabled() reported true with no endpoint")
	}
	shutdown, err := Init(context.Background(), "ragdesk-test", "0.0.0")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Init returned a nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestEnabledReflectsEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	if !Enabled() {
		t.Fatal("Enabled() reported false with an endpoint set")
	}
}
