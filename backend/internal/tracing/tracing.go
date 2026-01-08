package tracing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// Config holds the configuration for OpenTelemetry tracing initialization.
// ServiceName is required; Environment and TracesExport default from env when unset.
// TracesExport supports "stdout" (default) and "none"/"noop". PrettyPrint enables
// human-readable stdout traces for local development.
type Config struct {
	ServiceName  string
	Environment  string
	PrettyPrint  bool
	TracesExport string // stdout|none (default: stdout)
}

// InitTracer initializes OpenTelemetry tracing (tracer provider + propagators).
// It returns a shutdown function that should be called on process exit.
func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("tracing: ServiceName is required")
	}
	if cfg.Environment == "" {
		cfg.Environment = getenvDefault("APP_ENV", "development")
	}
	if cfg.TracesExport == "" {
		cfg.TracesExport = getenvDefault("OTEL_TRACES_EXPORTER", "stdout")
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(parseSamplerFromEnv(cfg.Environment)),
	}

	switch cfg.TracesExport {
	case "none", "noop":
		// No span processor/exporter configured: spans will be no-op exported.
	default:
		expOpts := []stdouttrace.Option{}
		if cfg.PrettyPrint {
			expOpts = append(expOpts, stdouttrace.WithPrettyPrint())
		}
		exporter, err := stdouttrace.New(expOpts...)
		if err != nil {
			return nil, fmt.Errorf("tracing: init stdout exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(cfg.ServiceName)

	return tp.Shutdown, nil
}

// GetTracer returns the global tracer instance
func GetTracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer("fifteen-thirty-one-go")
	}
	return tracer
}

// StartSpan is a helper function to start a new span
func StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, spanName)
}

func getenvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func parseSamplerFromEnv(appEnv string) sdktrace.Sampler {
	// Spec-ish: https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
	s := os.Getenv("OTEL_TRACES_SAMPLER")
	arg := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	switch s {
	case "", "parentbased_always_on":
		// In development, default to always-on for better local observability.
		if appEnv == "development" {
			return sdktrace.ParentBased(sdktrace.AlwaysSample())
		}
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		ratio, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			log.Printf("tracing: invalid OTEL_TRACES_SAMPLER_ARG=%q for traceidratio; defaulting to 1.0", arg)
			ratio = 1.0
		}
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		log.Printf("tracing: unsupported OTEL_TRACES_SAMPLER=%q; defaulting to 1.0", s)
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))
	}
}
