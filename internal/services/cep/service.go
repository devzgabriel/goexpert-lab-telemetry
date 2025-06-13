package cep

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ViaCepResponse struct {
	Cep          string `json:"cep"`
	State        string `json:"estado"`
	City         string `json:"localidade"`
	Neighborhood string `json:"bairro"`
	Street       string `json:"logradouro"`
}

func GetCepFromViaCep(cep string) (ViaCepResponse, error) {
	resp, err := http.Get("https://viacep.com.br/ws/" + cep + "/json/")
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

	return cepResponse, nil
}
