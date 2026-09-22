package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Reading struct {
	DeviceID     string  `json:"device_id"`
	Temperature  float32 `json:"temperature"`
	Humidity     float32 `json:"humidity"`
	SoilMoisture float32 `json:"soil_moisture"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status: "ok",
	}

	writeJSON(w, http.StatusOK, response)
}

func deviceHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Method: ", r.Method)
	fmt.Println("Path: ", r.URL.Path)
	fmt.Println("ID: ", r.PathValue("id"))

	id := r.PathValue("id")

	device := Device{
		ID:   id,
		Name: "ESP32",
	}

	writeJSON(w, http.StatusOK, device)

}

func latestHandler(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")

	response := Reading{
		DeviceID:     id,
		Temperature:  24.3,
		Humidity:     61.2,
		SoilMoisture: 43,
	}

	writeJSON(w, http.StatusOK, response)

}
