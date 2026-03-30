package observability

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TraceConfig struct {
	ServiceName string
	Environment string
	Enabled     bool
	Endpoint    string
	Insecure    bool
}

func InitTracing(ctx context.Context, cfg TraceConfig) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	}

	if cfg.Enabled && cfg.Endpoint != "" {
		clientOptions := []otlptracehttp.Option{}
		if strings.HasPrefix(cfg.Endpoint, "http://") || strings.HasPrefix(cfg.Endpoint, "https://") {
			clientOptions = append(clientOptions, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		} else {
			clientOptions = append(clientOptions, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			clientOptions = append(clientOptions, otlptracehttp.WithInsecure())
		}

		exporter, err := otlptracehttp.New(ctx, clientOptions...)
		if err != nil {
			return nil, err
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}

	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)

	return provider.Shutdown, nil
}
