package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	if response.Status != "ok" {
		t.Errorf(
			"expected status ok, got %s",
			response.Status,
		)
	}

}

func TestLatestHandler(t *testing.T) {
	router := NewRouter()

	req := httptest.NewRequest(
		http.MethodGet,
		"/devices/plant-42/readings/latest",
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

	var response Reading

	err := json.NewDecoder(rec.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if response.DeviceID != "plant-42" {
		t.Errorf(
			"expected device ID plant-42, got %s",
			response.DeviceID,
		)
	}
}
