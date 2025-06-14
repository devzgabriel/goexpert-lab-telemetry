package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type WeatherResponse struct {
	Location struct {
		Name           string  `json:"name"`
		Region         string  `json:"region"`
		Country        string  `json:"country"`
		Lat            float32 `json:"lat"`
		Lon            float32 `json:"lon"`
		TzId           string  `json:"tz_id"`
		LocaltimeEpoch int64   `json:"localtime_epoch"`
		Localtime      string  `json:"localtime"`
	} `json:"location"`
	Current struct {
		LastUpdatedEpoch int64   `json:"last_updated_epoch"`
		LastUpdated      string  `json:"last_updated"`
		TempC            float32 `json:"temp_c"`
		TempF            float32 `json:"temp_f"`
		Condition        struct {
			Text string `json:"text"`
			Icon string `json:"icon"`
			Code int    `json:"code"`
		} `json:"condition"`
	}
}

func GetWeatherData(city string, apiKey string) (WeatherResponse, error) {
	return GetWeatherDataWithContext(context.Background(), city, apiKey)
}

func GetWeatherDataWithContext(ctx context.Context, city string, apiKey string) (WeatherResponse, error) {
	tracer := otel.Tracer("weather-service-tracer")
	ctx, span := tracer.Start(ctx, "GetWeatherData")
	defer span.End()

	span.SetAttributes(
		attribute.String("weather.city", city),
		attribute.Bool("weather.api_key_provided", apiKey != ""),
	)

	if city == "" {
		fmt.Println("Error: city is empty")
		return WeatherResponse{}, fmt.Errorf("city is empty")
	}
	if apiKey == "" {
		fmt.Println("Error: API key is empty")
		return WeatherResponse{}, fmt.Errorf("API key is empty")
	}

	weatherUrl := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, url.QueryEscape(city))

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	req, err := http.NewRequestWithContext(ctx, "GET", weatherUrl, nil)
	if err != nil {
		fmt.Println("Error creating weather request:", err)
		return WeatherResponse{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error fetching weather data:", err)
		return WeatherResponse{}, err
	}
	defer resp.Body.Close()

	if resp.Body == nil {
		fmt.Println("Error: response body is nil")
		return WeatherResponse{}, fmt.Errorf("response body is nil")
	}

	weatherResponse := WeatherResponse{}
	err = json.NewDecoder(resp.Body).Decode(&weatherResponse)
	if err != nil {
		fmt.Println("Error decoding weather response:", err)
		return WeatherResponse{}, err
	}

	span.SetAttributes(
		attribute.Float64("weather.temp_c", float64(weatherResponse.Current.TempC)),
		attribute.Float64("weather.temp_f", float64(weatherResponse.Current.TempF)),
		attribute.String("weather.location_name", weatherResponse.Location.Name),
		attribute.String("weather.condition", weatherResponse.Current.Condition.Text),
	)

	return weatherResponse, nil
}
