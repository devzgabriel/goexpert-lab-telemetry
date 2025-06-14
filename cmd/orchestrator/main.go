package main

import (
	"context"
	otel_provider "devzgabriel/goexpert-lab-telemetry/internal/otel"
	"devzgabriel/goexpert-lab-telemetry/internal/services/cep"
	"devzgabriel/goexpert-lab-telemetry/internal/services/weather"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type RequestBody struct {
	Cep string `json:"cep"`
}

type SuccessResponse struct {
	TempC float32 `json:"temp_c"`
	TempF float32 `json:"temp_f"`
	TempK float32 `json:"temp_k"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	run()
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	// otelShutdown, err := otel_provider.SetupOTelSDK(ctx)
	otelShutdown, err := otel_provider.InitProvider(os.Getenv("OTEL_SERVICE_NAME"), os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", os.Getenv("ORCHESTRATOR_PORT")),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		mux.Handle(pattern, handler)
	}

	weatherSecretKey := os.Getenv("WEATHER_SECRET_KEY")

	if weatherSecretKey == "" {
		fmt.Println("WEATHER_SECRET_KEY is not set in the environment variables")
		weatherSecretKey = "f0b7cd19ad1841aeb40192648250906"
	}

	handleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is the Go Expert Lab Open Telemetry Orchestrator (Service B)!\n Use POST / to get the weather data for a given CEP.\n"))
	})
	handleFunc("POST /", getHandler(otel.Tracer("orchestrator-tracer"), weatherSecretKey))

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}
func getHandler(tracer trace.Tracer, weatherSecretKey string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		carrier := propagation.HeaderCarrier(r.Header)
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		ctx, span := tracer.Start(ctx, os.Getenv("OTEL_SERVICE_NAME")+" - Service B - Orchestrator Handler")
		defer span.End()

		reqBody := RequestBody{}

		err := json.NewDecoder(r.Body).Decode(&reqBody)

		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if reqBody.Cep == "" || len(reqBody.Cep) != 8 {
			http.Error(w, "CEP is required", http.StatusBadRequest)
			return
		}

		cepResponse, err := cep.GetCepFromViaCepWithContext(ctx, reqBody.Cep)
		if err != nil {
			http.Error(w, "Error fetching CEP data", http.StatusInternalServerError)
			return
		}

		if cepResponse.Cep == "" {
			http.Error(w, "Invalid CEP", http.StatusNotFound)
			return
		}

		weatherResponse, err := weather.GetWeatherDataWithContext(ctx, cepResponse.City, weatherSecretKey)
		if err != nil {
			http.Error(w, "Error fetching weather data", http.StatusInternalServerError)
			return
		}

		response := SuccessResponse{
			TempC: weatherResponse.Current.TempC,
			TempF: weatherResponse.Current.TempF,
			TempK: weatherResponse.Current.TempC + 273,
		}

		fmt.Println("Request processed successfully for CEP:", reqBody.Cep)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
