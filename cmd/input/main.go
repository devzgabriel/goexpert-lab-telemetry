package main

import (
	"bytes"
	"context"
	otel_provider "devzgabriel/goexpert-lab-telemetry/internal/otel"
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
	"go.opentelemetry.io/otel/attribute"
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

func startServer() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	orchestratorURL := os.Getenv("ORCHESTRATOR_URL")
	tracer := otel.Tracer("input-service-tracer")

	if orchestratorURL == "" {
		fmt.Println("ORCHESTRATOR_URL is not set in the environment variables")
		panic("ORCHESTRATOR_URL is required")
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is the Go Expert Lab Open Telemetry Input (Service A)!\n Use POST / to get the weather data for a given CEP.\n"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		carrier := propagation.HeaderCarrier(r.Header)
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		ctx, span := tracer.Start(ctx, os.Getenv("OTEL_SERVICE_NAME")+" - Input Service A - Input Handler")
		defer span.End()

		reqBody := RequestBody{}

		err := json.NewDecoder(r.Body).Decode(&reqBody)

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid request body",
			}
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if reqBody.Cep == "" || len(reqBody.Cep) != 8 {
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid zipcode",
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(response)
			return
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, "POST", orchestratorURL, bytes.NewBuffer(
			[]byte(fmt.Sprintf(`{"cep": "%s"}`, reqBody.Cep)),
		))
		if err != nil {
			http.Error(w, "Error creating request to orchestrator", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
		orchestratorResponse, err := http.DefaultClient.Do(req)

		// orchestratorResponse, err := http.Post(orchestratorURL, "application/json", bytes.NewBuffer(
		// 	[]byte(fmt.Sprintf(`{"cep": "%s"}`, reqBody.Cep)),
		// ))

		if err != nil {
			http.Error(w, "Error fetching orchestrator response", http.StatusInternalServerError)
			return
		}
		defer orchestratorResponse.Body.Close()

		if orchestratorResponse.StatusCode != http.StatusOK {
			http.Error(w, "Error fetching orchestrator response", http.StatusInternalServerError)
			return
		}

		var weatherResponse SuccessResponse
		if err := json.NewDecoder(orchestratorResponse.Body).Decode(&weatherResponse); err != nil {
			http.Error(w, "Error decoding orchestrator response", http.StatusInternalServerError)
			return
		}

		response := SuccessResponse{
			TempC: weatherResponse.TempC,
			TempF: weatherResponse.TempF,
			TempK: weatherResponse.TempC + 273,
		}

		fmt.Println("Request processed successfully for CEP:", reqBody.Cep)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	fmt.Printf("Starting server on port %s...\n", os.Getenv("INPUT_SERVICE_A_PORT"))
	http.ListenAndServe(
		fmt.Sprintf(":%s", os.Getenv("INPUT_SERVICE_A_PORT")), nil)
}

func main() {

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	// shutdown, err := otel_provider.InitProvider(os.Getenv("OTEL_SERVICE_NAME"), os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	// if err != nil {
	// 	fmt.Printf("failed to initialize TracerProvider: %v\n", err)
	// }
	// defer func() {
	// 	if err := shutdown(context.Background()); err != nil {
	// 		fmt.Printf("failed to shutdown TracerProvider: %v\n", err)
	// 	} else {
	// 		fmt.Println("TracerProvider shutdown successfully")
	// 	}
	// }()

	// startServer()

	run()
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	otelShutdown, err := otel_provider.SetupOTelSDK(ctx)
	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", os.Getenv("INPUT_SERVICE_A_PORT")),
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

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is the Go Expert Lab Open Telemetry Input (Service A)!\n Use POST / to get the weather data for a given CEP.\n"))
	})
	handleFunc("POST /", getHandler(otel.Tracer("input-service-tracer"), os.Getenv("ORCHESTRATOR_URL")))

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}

func getHandler(tracer trace.Tracer, orchestratorURL string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		carrier := propagation.HeaderCarrier(r.Header)
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		ctx, span := tracer.Start(ctx, os.Getenv("OTEL_SERVICE_NAME")+" - Input Service A - Input Handler")
		defer span.End()

		reqBody := RequestBody{}

		err := json.NewDecoder(r.Body).Decode(&reqBody)

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid request body",
			}
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		cepValueAttr := attribute.String("cep.value", reqBody.Cep)
		span.SetAttributes(cepValueAttr)

		if reqBody.Cep == "" || len(reqBody.Cep) != 8 {
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid zipcode",
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(response)
			return
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, "POST", orchestratorURL, bytes.NewBuffer(
			[]byte(fmt.Sprintf(`{"cep": "%s"}`, reqBody.Cep)),
		))
		if err != nil {
			http.Error(w, "Error creating request to orchestrator", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
		orchestratorResponse, err := http.DefaultClient.Do(req)

		// orchestratorResponse, err := http.Post(orchestratorURL, "application/json", bytes.NewBuffer(
		// 	[]byte(fmt.Sprintf(`{"cep": "%s"}`, reqBody.Cep)),
		// ))

		if err != nil {
			http.Error(w, "Error fetching orchestrator response", http.StatusInternalServerError)
			return
		}
		defer orchestratorResponse.Body.Close()

		if orchestratorResponse.StatusCode != http.StatusOK {
			http.Error(w, "Error fetching orchestrator response", http.StatusInternalServerError)
			return
		}

		var weatherResponse SuccessResponse
		if err := json.NewDecoder(orchestratorResponse.Body).Decode(&weatherResponse); err != nil {
			http.Error(w, "Error decoding orchestrator response", http.StatusInternalServerError)
			return
		}

		response := SuccessResponse{
			TempC: weatherResponse.TempC,
			TempF: weatherResponse.TempF,
			TempK: weatherResponse.TempC + 273,
		}

		fmt.Println("Request processed successfully for CEP:", reqBody.Cep)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
