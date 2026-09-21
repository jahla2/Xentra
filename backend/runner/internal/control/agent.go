package control

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jahla2/Xentra/backend/runner/internal/application"
	"github.com/jahla2/Xentra/backend/runner/internal/domain"
	"github.com/jahla2/Xentra/backend/runner/internal/observability"
)

type Agent struct {
	client    *Client
	discovery *application.DiscoveryService
	tools     *application.ToolService
	pollEvery time.Duration
}

func NewAgent(client *Client, discovery *application.DiscoveryService, tools *application.ToolService, pollEvery time.Duration) *Agent {
	if pollEvery < time.Second {
		pollEvery = 2 * time.Second
	}
	return &Agent{client: client, discovery: discovery, tools: tools, pollEvery: pollEvery}
}

func (a *Agent) Run(ctx context.Context) error {
	discovery := a.discovery.Discover(ctx)
	log.Printf("xentra outbound Runner started for host %s", discovery.Hostname)

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		task, err := a.client.Poll(ctx, discovery)
		if err != nil {
			log.Printf("outbound Runner poll failed: %v", err)
			if !sleepContext(ctx, a.pollEvery) {
				return nil
			}
			continue
		}
		if task == nil {
			if !sleepContext(ctx, a.pollEvery) {
				return nil
			}
			continue
		}

		taskContext := observability.ExtractTaskContext(ctx, task.TraceParent, task.TraceState)
		taskContext, span := otel.Tracer("github.com/jahla2/Xentra/backend/runner/tasks").Start(
			taskContext,
			"runner.tool."+task.Tool,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("xentra.runner.task_id", task.ID),
				attribute.String("xentra.tool", task.Tool),
			),
		)
		result := a.tools.Execute(taskContext, task.Tool, task.Arguments)
		if !result.Success {
			span.SetStatus(codes.Error, result.Error)
		}
		if err := a.completeWithRetry(taskContext, task.ID, result); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			log.Printf("outbound Runner failed to submit task %s result: %v", task.ID, err)
		}
		span.End()
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *Agent) completeWithRetry(ctx context.Context, taskID string, result domain.ToolResult) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if err := a.client.Complete(ctx, taskID, result); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if !sleepContext(ctx, time.Duration(attempt+1)*time.Second) {
			return ctx.Err()
		}
	}
	return lastErr
}
