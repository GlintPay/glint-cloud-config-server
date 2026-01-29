package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/GlintPay/gccs/api"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/setup"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/health"
	"github.com/GlintPay/gccs/logging"
	"github.com/GlintPay/gccs/resolver/k8s"
	"github.com/GlintPay/gccs/utils"
	"github.com/caarlos0/env/v6"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"golang.org/x/sync/errgroup"
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

	logging.Setup(os.Stdout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	////////////////////////////////////////////

	backends, backendErr := setup.Init(ctx, appConfig)
	if backendErr != nil {
		log.Fatal().Stack().Err(backendErr).Msg("Backend init failed")
	}

	////////////////////////////////////////////

	var k8sResolver *k8s.Resolver
	var e error
	if appConfig.Kubernetes.Enabled {
		k8sResolver, e = setupK8sResolver(appConfig.Kubernetes)
		if e != nil {
			log.Warn().Err(e).Msg("K8s resolver setup failed; K8s placeholders will return errors")
		}
	} else {
		log.Info().Msg("K8s secret/configmap resolver disabled")
	}

	////////////////////////////////////////////

	var traceShutdown func()
	traceShutdown, e = setupTracing(ctx, appConfig)
	if e != nil {
		log.Fatal().Stack().Err(e).Msg("Trace setup failed")
	}
	defer traceShutdown()

	router := setupRouter(appConfig, backends, k8sResolver)
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

func setupK8sResolver(cfg config.K8sConfig) (*k8s.Resolver, error) {
	client, err := k8s.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	log.Info().Msg("K8s secret/configmap resolver enabled")
	return k8s.NewResolver(client, cfg), nil
}

func setupRouter(config config.ApplicationConfiguration, backends backend.Backends, k8sResolver *k8s.Resolver) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.StripSlashes)

	routing := api.Routing{
		ServerName:   serviceName,
		ParentRouter: router,

		Backends:    backends,
		AppConfig:   config,
		K8sResolver: k8sResolver,
	}

	router.Route("/", func(r chi.Router) {
		r.Use(httplog.Handler(log.Logger))
		r.Use(middleware.RequestID)
		r.Use(middleware.Compress(5))

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
