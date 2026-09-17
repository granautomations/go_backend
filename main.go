package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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

func newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /devices/{id}", deviceHandler)
	mux.HandleFunc("GET /devices/{id}/readings/latest", latestHandler)

	return mux
}

func main() {

	router := newRouter()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	go func() {
		log.Println("server listening on :8080")

		err := server.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()

	<-shutdownCtx.Done()

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("server stopped")

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
