package observability

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type tracingRoundTripper struct {
	base http.RoundTripper
}

func NewTracingTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &tracingRoundTripper{base: base}
}

func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := otel.Tracer("github.com/jahla2/Xentra/backend/runner/http-client")
	ctx, span := tracer.Start(req.Context(), req.Method+" "+req.URL.Host,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.request.method", req.Method),
			attribute.String("server.address", req.URL.Hostname()),
			attribute.String("url.path", req.URL.Path),
		),
	)
	defer span.End()

	cloned := req.Clone(ctx)
	cloned.Header = req.Header.Clone()
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(cloned.Header))

	res, err := t.base.RoundTrip(cloned)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", res.StatusCode))
	if res.StatusCode >= 500 {
		span.SetStatus(codes.Error, res.Status)
	}
	return res, nil
}

func ExtractTaskContext(ctx context.Context, traceParent, traceState string) context.Context {
	carrier := propagation.MapCarrier{}
	if traceParent != "" {
		carrier.Set("traceparent", traceParent)
	}
	if traceState != "" {
		carrier.Set("tracestate", traceState)
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
