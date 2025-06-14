package cep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type ViaCepResponse struct {
	Cep          string `json:"cep"`
	State        string `json:"estado"`
	City         string `json:"localidade"`
	Neighborhood string `json:"bairro"`
	Street       string `json:"logradouro"`
}

func GetCepFromViaCep(cep string) (ViaCepResponse, error) {
	return GetCepFromViaCepWithContext(context.Background(), cep)
}

func GetCepFromViaCepWithContext(ctx context.Context, cep string) (ViaCepResponse, error) {
	tracer := otel.Tracer("cep-service-tracer")
	ctx, span := tracer.Start(ctx, "GetCepFromViaCep")
	defer span.End()

	// Add CEP as span attribute
	span.SetAttributes(attribute.String("cep.value", cep))

	// Create HTTP client with OpenTelemetry instrumentation
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	
	req, err := http.NewRequestWithContext(ctx, "GET", "https://viacep.com.br/ws/"+cep+"/json/", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return ViaCepResponse{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error fetching data from ViaCep:", err)
		return ViaCepResponse{}, err
	}
	defer resp.Body.Close()

	cepResponse := ViaCepResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&cepResponse); err != nil {
		fmt.Println("Error decoding ViaCep response:", err)
		return ViaCepResponse{}, err
	}

	// Add response data as span attributes
	span.SetAttributes(
		attribute.String("cep.city", cepResponse.City),
		attribute.String("cep.state", cepResponse.State),
	)

	return cepResponse, nil
}
