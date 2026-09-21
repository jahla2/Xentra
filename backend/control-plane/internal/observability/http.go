package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func HTTPMiddleware(spanName string, next http.Handler) http.Handler {
	tracer := otel.Tracer("github.com/jahla2/Xentra/backend/control-plane/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		spanNameValue := spanName
		if spanNameValue == "" {
			spanNameValue = r.Method + " " + r.URL.Path
		}
		ctx, span := tracer.Start(ctx, spanNameValue,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("url.path", r.URL.Path),
				attribute.String("network.protocol.version", r.Proto),
			),
		)
		defer span.End()

		recorder := &traceResponseWriter{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))
		span.SetAttributes(
			attribute.Int("http.response.status_code", recorder.status),
			attribute.Int64("http.server.duration_ms", time.Since(started).Milliseconds()),
		)
		if recorder.status >= 500 {
			span.SetStatus(codes.Error, strconv.Itoa(recorder.status))
		}
	})
}

type traceResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *traceResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *traceResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

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
	tracer := otel.Tracer("github.com/jahla2/Xentra/backend/control-plane/http-client")
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

func InjectContext(ctx context.Context) (string, string) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent"), carrier.Get("tracestate")
}

func ExtractContext(ctx context.Context, traceParent, traceState string) context.Context {
	carrier := propagation.MapCarrier{}
	if traceParent != "" {
		carrier.Set("traceparent", traceParent)
	}
	if traceState != "" {
		carrier.Set("tracestate", traceState)
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
