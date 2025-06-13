package main

import (
	"devzgabriel/goexpert-lab-telemetry/internal/services/cep"
	"devzgabriel/goexpert-lab-telemetry/internal/services/weather"
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

	weatherSecretKey := os.Getenv("WEATHER_SECRET_KEY")

	if weatherSecretKey == "" {
		fmt.Println("WEATHER_SECRET_KEY is not set in the environment variables")
		weatherSecretKey = "f0b7cd19ad1841aeb40192648250906"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World! This is the Go Expert Lab Open Telemetry Orchestrator (Service B)!\n Use POST / to get the weather data for a given CEP.\n"))
	})

	http.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {

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

		cepResponse, err := cep.GetCepFromViaCep(reqBody.Cep)
		if err != nil {
			http.Error(w, "Error fetching CEP data", http.StatusInternalServerError)
			return
		}

		if cepResponse.Cep == "" {
			http.Error(w, "Invalid CEP", http.StatusNotFound)
			return
		}

		weatherResponse, err := weather.GetWeatherData(cepResponse.City, weatherSecretKey)
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
	})

	fmt.Printf("Starting server on port %s...\n", os.Getenv("ORCHESTRATOR_PORT"))
	http.ListenAndServe(
		fmt.Sprintf(":%s", os.Getenv("ORCHESTRATOR_PORT")), nil)
}

func main() {
	startServer()
}
