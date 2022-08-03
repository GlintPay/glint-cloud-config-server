package main

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/api"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/git"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/health"
	"github.com/GlintPay/gccs/logging"
	"github.com/GlintPay/gccs/utils"
	"github.com/caarlos0/env/v6"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	promApi "github.com/poblish/promenade/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"sigs.k8s.io/yaml"
)

const serviceName = "gccs"

var envConfig = config.Configuration{}

func main() {
	if err := env.Parse(&envConfig); err != nil {
		log.Fatal().Msgf("Configuration loading failed: %+v", err)
	}

	appConfig := config.ApplicationConfiguration{}
	readConfig(envConfig.ApplicationConfigFileYmlPath, &appConfig)
	//if len(appConfig.Prometheus.Path) > 0 {
	metrics := promApi.NewMetrics(promApi.MetricOpts{MetricNamePrefix: serviceName})
	appConfig.Prometheus.Metrics = &metrics
	//}

	logging.Setup(os.Stdout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	////////////////////////////////////////////

	var backends backend.Backends
	backends = append(backends, &git.Backend{}) // just one for now!
	backends = append(backends, &git.Backend{EnableTrace: appConfig.Tracing.Enabled}) // just one for now!

	for _, each := range backends {
		if backendErr := each.Init(ctx, appConfig, appConfig.Prometheus.Metrics); backendErr != nil {
			log.Fatal().Stack().Err(backendErr).Msg("Backend init failed")
		}
	}

	////////////////////////////////////////////

	traceShutdown, e := setupTracing(ctx, appConfig)
	if e != nil {
		log.Fatal().Stack().Err(e).Msg("Trace setup failed")
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
		log.Info().Msgf("Listening on %s", port)
		if err := http.ListenAndServe(port, router); err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	err := g.Wait()
	if err != nil {
		log.Fatal().Stack().Err(err).Msg("startup failed")
	}
}

func readConfig(filePath string, config *config.ApplicationConfiguration) {
	yamlFile, err := os.ReadFile(filePath)
	if err == nil {
		log.Debug().Msgf("Loading YAML config from %s", utils.FriendlyFileName(filePath))
		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			log.Fatal().Stack().Err(err).Msg("Unmarshal")
		}
	} else {
		log.Printf("No config file found: %s", utils.FriendlyFileName(filePath))
	}
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
		log.Fatal().Stack().Err(err).Msg("failed to create resource")
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

	log.Info().Msgf("OpenTelemetry export is enabled, to: %s", config.Tracing.Endpoint)

	return func() {
		if err = tracerProvider.Shutdown(ctx); err != nil {
			log.Fatal().Stack().Err(err).Msg("failed to shutdown TracerProvider")
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
			log.Fatal().Stack().Err(e).Msg("route setup failed")
		}
	})

	if len(config.Prometheus.Path) > 0 {
		log.Info().Msgf("Registering metrics endpoint at: %s", config.Prometheus.Path)
		router.Handle(config.Prometheus.Path, promhttp.Handler())
	}

	return router
}

func setupHealthCheck(router *chi.Mux) {
	healthChk := health.New(health.WithChiMux(router))
	healthChk.StartListening()
}
