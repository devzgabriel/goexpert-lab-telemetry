package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	if city == "" {
		fmt.Println("Error: city is empty")
		return WeatherResponse{}, fmt.Errorf("city is empty")
	}
	if apiKey == "" {
		fmt.Println("Error: API key is empty")
		return WeatherResponse{}, fmt.Errorf("API key is empty")
	}

	url := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, url.QueryEscape(city))

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching weather data:", err)
		return WeatherResponse{}, err
	}
	defer resp.Body.Close()

	// DEBUG only
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	fmt.Println("Error reading response body:", err)
	// 	return WeatherResponse{}, err
	// }
	// fmt.Println("Response body:", string(body))
	// END DEBUG

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

	return weatherResponse, nil
}
