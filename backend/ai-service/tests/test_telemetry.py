from app.telemetry import Telemetry


def test_telemetry_exporter_disabled_without_endpoint(monkeypatch) -> None:
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", raising=False)
    monkeypatch.delenv("OTEL_TRACES_EXPORTER", raising=False)

    assert Telemetry._exporter_enabled() is False


def test_telemetry_exporter_enabled_with_otlp_endpoint(monkeypatch) -> None:
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
    monkeypatch.delenv("OTEL_TRACES_EXPORTER", raising=False)

    assert Telemetry._exporter_enabled() is True


def test_telemetry_exporter_none_overrides_endpoint(monkeypatch) -> None:
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
    monkeypatch.setenv("OTEL_TRACES_EXPORTER", "none")

    assert Telemetry._exporter_enabled() is False
