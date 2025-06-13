package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
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

	if orchestratorURL == "" {
		fmt.Println("ORCHESTRATOR_URL is not set in the environment variables")
		panic("ORCHESTRATOR_URL is required")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is the Go Expert Lab Open Telemetry Input (Service A)!\n Use POST / to get the weather data for a given CEP.\n"))
	})

	http.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {

		reqBody := RequestBody{}

		err := json.NewDecoder(r.Body).Decode(&reqBody)

		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid request body",
			}
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if reqBody.Cep == "" || len(reqBody.Cep) != 8 {
			http.Error(w, "CEP is required", http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			response := ErrorResponse{
				Message: "Mensagem: invalid zipcode",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(response)
			return
		}

		orchestratorResponse, err := http.Post(orchestratorURL, "application/json", bytes.NewBuffer(
			[]byte(fmt.Sprintf(`{"cep": "%s"}`, reqBody.Cep)),
		))

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
	startServer()
}
