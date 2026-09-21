package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestTracingTransportInjectsTraceParent(t *testing.T) {
	oldProvider := otel.GetTracerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(oldProvider)
		otel.SetTextMapPropagator(oldPropagator)
	}()

	traceID, _ := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	spanID, _ := trace.SpanIDFromHex("0011223344556677")
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID: spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), parent)

	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: NewTracingTransport(nil)}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if received == "" {
		t.Fatal("traceparent header was not injected")
	}
	if !strings.Contains(received, traceID.String()) {
		t.Fatalf("traceparent %q does not contain parent trace ID %s", received, traceID.String())
	}
}

func TestHTTPMiddlewareExtractsIncomingTrace(t *testing.T) {
	oldProvider := otel.GetTracerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(oldProvider)
		otel.SetTextMapPropagator(oldPropagator)
	}()

	var got trace.TraceID
	handler := HTTPMiddleware("test.request", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanContextFromContext(r.Context()).TraceID()
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("traceparent", "00-00112233445566778899aabbccddeeff-0011223344556677-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got.String() != "00112233445566778899aabbccddeeff" {
		t.Fatalf("unexpected extracted trace ID %s", got.String())
	}
}
