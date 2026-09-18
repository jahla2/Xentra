# Distributed Tracing

Xentra uses OpenTelemetry traces with W3C Trace Context propagation.

## Services

- `xentra-control-plane`
- `xentra-ai-service`
- `xentra-runner`

Tracing is optional. When no OTLP endpoint is configured, the Go and Python SDKs still provide local trace context for propagation but do not create an exporter.

## Request path

A typical outbound-Runner investigation follows one trace:

```
browser request
  -> control-plane HTTP server span
     -> AI HTTP client span
        -> FastAPI server span
           -> optional LLM / embedding HTTPX spans
     -> Runner task enqueue
        -> PostgreSQL runner_tasks stores traceparent / tracestate
        -> outbound Runner claims task
           -> runner.tool.<typed-tool> consumer span
              -> result HTTP client span
                 -> Runner-control HTTP server span
```

No secret values, command output, private keys, tokens, or tool argument values are attached to spans.

## Queue propagation

HTTP headers cannot cross a PostgreSQL task queue directly. Xentra therefore injects the active W3C `traceparent` and `tracestate` into dedicated `runner_tasks` columns when a task is created.

When a Runner claims the task, it receives those fields and extracts them as a remote parent before starting the typed-tool execution span. This keeps asynchronous Runner work in the original investigation trace.

## HTTP propagation

The control plane:

- extracts incoming `traceparent` and `tracestate`;
- creates server spans for the user API and dedicated Runner-control listener;
- injects trace context into AI, GitHub, and legacy Runner HTTP clients.

The AI service uses FastAPI and HTTPX OpenTelemetry instrumentation.

The outbound Runner injects context on task-result HTTP calls.

## Logs

Structured control-plane request logs include:

- `request_id`
- `trace_id`
- `span_id`

This allows logs and traces to be correlated without putting sensitive diagnostic evidence into telemetry attributes.

## Export

Applications use OTLP/HTTP protobuf. Set either:

- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`

Set `OTEL_TRACES_EXPORTER=none` to explicitly disable trace export.

The optional production Compose profile starts OpenTelemetry Collector 0.161.0 with OTLP gRPC/HTTP receivers and a batch processor. Its bundled debug exporter is intended as a safe validation default; production operators should replace it with their chosen tracing backend.
