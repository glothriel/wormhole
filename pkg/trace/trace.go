package trace

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func Trace(name string, ctx context.Context, f func(context.Context) error) error {
	logrus.Error(name)
	tr := otel.GetTracerProvider().Tracer(name)
	newCtx, span := tr.Start(ctx, name, trace.WithSpanKind(trace.SpanKindUnspecified))
	defer span.End()
	return f(newCtx)
}
