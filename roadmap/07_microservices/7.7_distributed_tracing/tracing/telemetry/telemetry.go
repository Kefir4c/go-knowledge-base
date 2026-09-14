package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracer настраивает TracerProvider для сервиса.
// Возвращает функцию shutdown, которую надо вызвать при остановке.
func InitTracer(ctx context.Context, serviceName string) (func(ctx2 context.Context) error, error) {
	// Экспортёр: отправляет span'ы в Jaeger по OTLP/gRPC.
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure())

	if err != nil {
		return nil, err
	}

	// Resource: метаданные сервиса. Видны в Jaeger UI.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("dev")))

	if err != nil {
		return nil, err
	}

	// TracerProvider с batch-экспортом и head-based sampling.
	tp := sdktrace.NewTracerProvider(
		// Батчинг: снижает нагрузку на сеть и backend.
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(2*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		// 100% в dev. В проде — 0.01-0.1.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(tp)

	// Propagator: прокидывает trace context через headers.
	// W3C Trace Context — стандарт индустрии.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{}))

	return tp.Shutdown, nil
}
