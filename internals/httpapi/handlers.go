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

type CreateReadingRequest struct {
	Temperature  float32 `json:"temperature"`
	Humidity     float32 `json:"humidity"`
	SoilMoisture float32 `json:"soil_moisture"`
}

type ErrorResponse struct {
	Error string `json:"error"`
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

func (h *handler) latestHandler(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")

	reading, found := h.readings.Latest(id)

	if !found {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error: "reading not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, reading)

}

func (h *handler) createReadingHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var request CreateReadingRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid JSON body",
		})
		return
	}

	reading := Reading{
		DeviceID:     id,
		Temperature:  request.Temperature,
		Humidity:     request.Humidity,
		SoilMoisture: request.SoilMoisture,
	}

	h.readings.Save(reading)

	writeJSON(w, http.StatusCreated, reading)
}
