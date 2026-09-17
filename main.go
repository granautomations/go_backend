package main

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

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /devices/{id}", deviceHandler)
	mux.HandleFunc("GET /devices/{id}/readings/latest", latestHandler)

	log.Println("server listening on :8080")

	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		fmt.Println("server error: ", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {

	response := HealthResponse{
		Status: "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(device)

}

func latestHandler(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")

	response := Reading{
		DeviceID:     id,
		Temperature:  24.3,
		Humidity:     61.2,
		SoilMoisture: 43,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

}
