package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"istio-test/internal/config"
	"istio-test/internal/metadata"
	"istio-test/internal/observability"
	"istio-test/internal/security"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

func main() {
	ctx := context.Background()

	// Load configuration
	conf := config.Load()

	observability.Init(conf.Observability.LogLevel, observability.Config{
		EnablePIIRedaction: conf.Observability.EnablePIIRedaction,
	})

	observability.InfoWithContext(ctx, "Application is starting")

	if conf.Observability.EnableTracing {
		tracer.Start(tracer.WithRuntimeMetrics())
		defer tracer.Stop()
	}

	if conf.Observability.EnableProfiler {
		err := profiler.Start(
			profiler.WithProfileTypes(
				profiler.CPUProfile,
				profiler.HeapProfile,
			),
		)
		if err != nil {
			observability.ErrorWithContext(ctx, fmt.Sprintf("Warning: Failed to start profiler: %v", err))
		}
		defer profiler.Stop()
	}

	// Create metadata client with configuration
	metadataClient := metadata.NewClient(
		conf.Metadata.HTTPTimeout,
		conf.Metadata.MaxRetries,
		conf.Metadata.BaseRetryDelay,
		conf.Metadata.MaxRetryDelay,
		conf.Metadata.RetryMultiplier,
	)

	// Create security options for API endpoints (less restrictive)
	apiSecurityOptions := security.CustomSecurityOptions(
		conf.Security.APICOEP,
		conf.Security.APICOOP,
		conf.Security.APICORP,
	)

	// Create security options for default endpoints (more restrictive)
	defaultSecurityOptions := security.CustomSecurityOptions(
		conf.Security.DefaultCOEP,
		conf.Security.DefaultCOOP,
		conf.Security.DefaultCORP,
	)

	mux := httptrace.NewServeMux()
	mux.HandleFunc("/istio-test/metadata/", metadata.SecureMetadataHandlerWithOptions(metadataClient.FetchMetadata, apiSecurityOptions))
	mux.HandleFunc("/istio-test/health", metadata.SecureEnhancedHealthCheckHandlerWithOptions(metadataClient, apiSecurityOptions))
	mux.HandleFunc("/istio-test/health/basic", metadata.SecureHealthCheckHandlerWithOptions(apiSecurityOptions)) // Keep basic health check for compatibility
	mux.HandleFunc("/", metadata.SecureNotFoundHandlerWithOptions(defaultSecurityOptions))

	// Wrap the entire mux with request logging middleware
	loggedHandler := observability.RequestLoggingMiddleware(mux)

	server := &http.Server{
		Addr:         ":" + conf.Server.Port,
		ReadTimeout:  conf.Server.ReadTimeout,
		WriteTimeout: conf.Server.WriteTimeout,
		IdleTimeout:  conf.Server.IdleTimeout,
		Handler:      loggedHandler,
	}

	go func() {
		observability.InfoWithContext(ctx, fmt.Sprintf("Starting server on port %s...", conf.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			observability.ErrorWithContext(ctx, fmt.Sprintf("Failed to start server: %v", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	observability.InfoWithContext(ctx, "Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), conf.Observability.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		observability.ErrorWithContext(ctx, fmt.Sprintf("Server forced to shutdown: %v", err))
	}

	observability.InfoWithContext(ctx, "Server exiting")
}
