package otel

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var ServerOptions = trace.WithSpanKind(trace.SpanKindServer)

const InstrumentationName = "github.com/GlintPay/glint-cloud-config-server"

func GetTracer(ctx context.Context) trace.Tracer {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		//fmt.Printf("GOT valid span for %s / %s, return for %v\n", span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String(), span.TracerProvider())
		return newTracer(span.TracerProvider())
	} else {
		//fmt.Printf("No valid span, return for %v\n", otel.GetTracerProvider())
		return newTracer(otel.GetTracerProvider())
	}
}

func newTracer(tp trace.TracerProvider) trace.Tracer {
	return tp.Tracer(InstrumentationName, trace.WithInstrumentationVersion("semver:1.0"))
}
