package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func rolldice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := otel.Tracer("rolldice").Start(r.Context(), "rolldice")

	fmt.Printf("%v", ctx)
	defer span.End()
	reqBaggage := baggage.FromContext(ctx)
	span.SetAttributes(attribute.String(
		"req.addr",
		reqBaggage.Member("req.addr").Value()),
	)
	roll := 1 + rand.Intn(6)

	resp := strconv.Itoa(roll) + "\n"
	if _, err := io.WriteString(w, resp); err != nil {
		log.Printf("Write failed: %v\n", err)
	}
}
func setUpObservability() (*trace.TracerProvider, error) {
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("rolldice"),
			semconv.ServiceVersion("v0.0.1"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %v", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//receiverAddress := "64.23.238.142:4318"
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint("localhost:4318"))
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %v", err)
	}

	provider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res),
		trace.WithBatcher(exp),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{}),
	)
	return provider, nil

}
func main() {
	tpProvider, err := setUpObservability()
	if err != nil {
		fmt.Printf("Error setting up observability: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := tpProvider.Shutdown(context.Background()); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v", err)
			os.Exit(1)
		}
	}()
	http.HandleFunc("/rolldice", rolldice)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
