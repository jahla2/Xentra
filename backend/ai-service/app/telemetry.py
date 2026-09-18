from __future__ import annotations

import os

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


class Telemetry:
    def __init__(self, service_name: str) -> None:
        provider = TracerProvider(
            resource=Resource.create({"service.name": service_name})
        )
        if self._exporter_enabled():
            provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))

        trace.set_tracer_provider(provider)
        self._provider = provider
        self._httpx_instrumented = False

    def instrument_httpx(self) -> None:
        if self._httpx_instrumented:
            return
        HTTPXClientInstrumentor().instrument()
        self._httpx_instrumented = True

    def instrument_fastapi(self, app) -> None:
        FastAPIInstrumentor.instrument_app(app)

    def shutdown(self) -> None:
        if self._httpx_instrumented:
            HTTPXClientInstrumentor().uninstrument()
            self._httpx_instrumented = False
        self._provider.shutdown()

    @staticmethod
    def _exporter_enabled() -> bool:
        if os.getenv("OTEL_TRACES_EXPORTER", "").strip().lower() == "none":
            return False
        return bool(
            os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "").strip()
            or os.getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "").strip()
        )


def configure_telemetry(service_name: str) -> Telemetry:
    telemetry = Telemetry(service_name)
    telemetry.instrument_httpx()
    return telemetry
