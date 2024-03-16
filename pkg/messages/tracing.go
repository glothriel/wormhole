package messages

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func ParseContext(m Message) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier(m.Context))
}

func DumpContext(ctx context.Context) map[string]string {
	carrier := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
	return carrier
}
