package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestExtractTaskContextRestoresTraceParent(t *testing.T) {
	old := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(old)

	ctx := ExtractTaskContext(
		context.Background(),
		"00-00112233445566778899aabbccddeeff-0011223344556677-01",
		"",
	)
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		t.Fatal("expected valid extracted span context")
	}
	if spanContext.TraceID().String() != "00112233445566778899aabbccddeeff" {
		t.Fatalf("unexpected trace ID %s", spanContext.TraceID().String())
	}
	if !spanContext.IsRemote() {
		t.Fatal("expected queued task context to be remote")
	}
}
