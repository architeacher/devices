package infrastructure

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	exporterTypeGRPC   = "grpc"
	exporterTypeStdOut = "stdout"
)

// NewTracerProvider creates a new OpenTelemetry tracer provider.
func NewTracerProvider(appConfig config.App, telemetryConfig config.Telemetry) (trace.TracerProvider, func(context.Context) error, error) {
	ctx := context.Background()

	traceExporter, err := createExporter(ctx, telemetryConfig)
	if err != nil {
		return nil, nil, err
	}

	hostName, err := os.Hostname()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get host name: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(appConfig.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("env", appConfig.Env.Name),
			attribute.String("host", hostName),
			attribute.String("commit_sha", config.CommitSHA),
			attribute.String("Product-Cluster", telemetryConfig.OtelProductCluster),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	sampler := sdktrace.TraceIDRatioBased(telemetryConfig.Traces.SamplerRatio)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sampler,
			sdktrace.WithRemoteParentSampled(sampler),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, tp.Shutdown, nil
}

// NewNoopTracerProvider creates a no-op tracer provider for when tracing is disabled.
func NewNoopTracerProvider() trace.TracerProvider {
	return noop.NewTracerProvider()
}

func createExporter(ctx context.Context, cfg config.Telemetry) (exporter sdktrace.SpanExporter, err error) {
	switch strings.ToLower(cfg.ExporterType) {
	case exporterTypeGRPC:
		exporter, err = createGRPCExporter(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC exporter: %w", err)
		}
	case exporterTypeStdOut:
		exporter, err = createStdOutExporter()
		if err != nil {
			return nil, fmt.Errorf("failed to create StdOut exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exporter type %q", cfg.ExporterType)
	}

	return exporter, nil
}

func createGRPCExporter(ctx context.Context, cfg config.Telemetry) (*otlptrace.Exporter, error) {
	conn, err := grpc.NewClient(
		net.JoinHostPort(cfg.OtelGRPCHost, cfg.OtelGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a gRPC client connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create a gRPC trace exporter: %w", err)
	}

	return traceExporter, nil
}

func createStdOutExporter() (*stdouttrace.Exporter, error) {
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("failed to create an StdOut trace exporter: %w", err)
	}

	return traceExporter, nil
}
