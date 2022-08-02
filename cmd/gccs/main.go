package main

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/api"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/git"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/health"
	"github.com/GlintPay/gccs/utils"
	"github.com/caarlos0/env/v6"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	promApi "github.com/poblish/promenade/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"sigs.k8s.io/yaml"
)

const serviceName = "gccs"

var envConfig = config.Configuration{}

func main() {
	if err := env.Parse(&envConfig); err != nil {
		log.Fatalf("Configuration loading failed: %+v\n", err)
	}

	appConfig := config.ApplicationConfiguration{}
	readConfig(envConfig.ApplicationConfigFileYmlPath, &appConfig)
	//if len(appConfig.Prometheus.Path) > 0 {
	metrics := promApi.NewMetrics(promApi.MetricOpts{MetricNamePrefix: serviceName})
	appConfig.Prometheus.Metrics = &metrics
	//}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	////////////////////////////////////////////

	var backends backend.Backends
	backends = append(backends, &git.Backend{}) // just one for now!

	for _, each := range backends {
		if backendErr := each.Init(ctx, appConfig, appConfig.Prometheus.Metrics); backendErr != nil {
			log.Fatal("Backend init failed", backendErr)
		}
	}

	////////////////////////////////////////////

	traceShutdown, e := setupTracing(ctx, appConfig)
	if e != nil {
		log.Fatalf("Trace setup failed: %s", e)
	}
	defer traceShutdown()

	router := setupRouter(appConfig, backends)
	setupHealthCheck(router)

	////////////////////////////////////////////

	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		if appConfig.Server.Port == 0 {
			appConfig.Server.Port = 80
		}
		port := fmt.Sprintf(":%d", appConfig.Server.Port)
		fmt.Println("Listening on", port)
		if err := http.ListenAndServe(port, router); err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	err := g.Wait()
	if err != nil {
		log.Fatalf("startup failed: %s", err)
	}
}

func readConfig(filePath string, config *config.ApplicationConfiguration) {
	yamlFile, err := os.ReadFile(filePath)
	if err == nil {
		fmt.Println("Loading YAML config from", utils.FriendlyFileName(filePath))
		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			log.Fatalf("Unmarshal: %v", err)
		}
	} else {
		log.Printf("No config file found: %s", utils.FriendlyFileName(filePath))
	}

	//fmt.Printf("%+v\n", config)
}

var emptyShutdown = func() {}

func setupTracing(ctx context.Context, config config.ApplicationConfiguration) (func(), error) {
	if !config.Tracing.Enabled {
		return emptyShutdown, nil
	}

	if config.Tracing.Endpoint == "" {
		return emptyShutdown, fmt.Errorf("missing tracing endpoint")
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource %v", err)
	}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(config.Tracing.Endpoint),
	)
	if err != nil {
		return emptyShutdown, fmt.Errorf("failed to create trace exporter %v", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.Tracing.SamplerFraction)),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	fmt.Printf("OpenTelemetry export is enabled, to: %s\n", config.Tracing.Endpoint)

	return func() {
		if err = tracerProvider.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown TracerProvider %v", err)
		}
	}, nil
}

func setupRouter(config config.ApplicationConfiguration, backends backend.Backends) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.StripSlashes)

	routing := api.Routing{
		ServerName:   serviceName,
		ParentRouter: router,

		Backends:  backends,
		AppConfig: config,
	}

	router.Route("/", func(r chi.Router) {
		if e := routing.SetupFunctionalRoutes(r); e != nil {
			log.Fatal(e)
		}
	})

	if len(config.Prometheus.Path) > 0 {
		fmt.Println("Registering metrics endpoint at:", config.Prometheus.Path)
		router.Handle(config.Prometheus.Path, promhttp.Handler())
	}

	return router
}

func setupHealthCheck(router *chi.Mux) {
	healthChk := health.New(health.WithChiMux(router))
	healthChk.StartListening()
}
