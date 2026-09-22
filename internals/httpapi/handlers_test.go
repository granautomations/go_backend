package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateReadingHandler(t *testing.T) {
	router := NewRouter()

	body := strings.NewReader(
		`{"temperature":24.3,"humidity":61.2,"soil_moisture":43}`,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/devices/plant-42/readings",
		body,
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusCreated,
			rec.Code,
		)
	}

	var response Reading
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.DeviceID != "plant-42" {
		t.Errorf(
			"expected device ID plant-42, got %s",
			response.DeviceID,
		)
	}

	if response.Temperature != 24.3 {
		t.Errorf(
			"expected temperature 24.3, got %v",
			response.Temperature,
		)
	}

	if response.Humidity != 61.2 {
		t.Errorf(
			"expected humidity 61.2, got %v",
			response.Humidity,
		)
	}

	if response.SoilMoisture != 43 {
		t.Errorf(
			"expected soil moisture 43, got %v",
			response.SoilMoisture,
		)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}

	contentType := rec.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %s",
			contentType,
		)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Status != "ok" {
		t.Errorf(
			"expected status ok, got %s",
			response.Status,
		)
	}

}

func TestLatestHandler(t *testing.T) {
	router := NewRouter()

	body := strings.NewReader(
		`{"temperature":24.3,"humidity":61.2,"soil_moisture":43}`,
	)

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/devices/plant-42/readings",
		body,
	)

	createRec := httptest.NewRecorder()

	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf(
			"expected create status %d, got %d",
			http.StatusCreated,
			createRec.Code,
		)
	}

	latestReq := httptest.NewRequest(
		http.MethodGet,
		"/devices/plant-42/readings/latest",
		nil,
	)

	latestRec := httptest.NewRecorder()

	router.ServeHTTP(latestRec, latestReq)

	if latestRec.Code != http.StatusOK {
		t.Fatalf(
			"expected latest status %d, got %d",
			http.StatusOK,
			latestRec.Code,
		)
	}

	var response Reading

	if err := json.NewDecoder(latestRec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.DeviceID != "plant-42" {
		t.Errorf(
			"expected device ID plant-42, got %s",
			response.DeviceID,
		)
	}

	if response.Temperature != 24.3 {
		t.Errorf(
			"expected temperature 24.3, got %v",
			response.Temperature,
		)
	}
}

func TestDeviceHandler(t *testing.T) {
	router := NewRouter()

	req := httptest.NewRequest(
		http.MethodGet,
		"/devices/plant-42",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}

	var response Device

	err := json.NewDecoder(rec.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if response.ID != "plant-42" {
		t.Errorf(
			"expected device ID plant-42, got %s",
			response.ID,
		)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %s",
			rec.Header().Get("Content-Type"),
		)
	}

	if response.Name != "ESP32" {
		t.Errorf(
			"expected device name ESP32, got %s",
			response.Name,
		)
	}
}
