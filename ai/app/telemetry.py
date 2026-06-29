"""Optional OpenTelemetry tracing.

A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set, so the default $0 run pays
nothing for it. When set, the OTLP/HTTP exporter reads the standard OTEL_* env
vars, FastAPI requests are traced (continuing the trace propagated from the Go
API), and psycopg queries become spans.
"""

import os

from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.psycopg import PsycopgInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


def setup_tracing(app: FastAPI) -> bool:
    """Instrument the app when an OTLP endpoint is configured.

    Returns whether tracing was enabled.
    """
    if not os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT"):
        return False

    provider = TracerProvider(resource=Resource.create({"service.name": "ragdesk-ai"}))
    provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(provider)

    FastAPIInstrumentor.instrument_app(app)
    PsycopgInstrumentor().instrument()
    return True
