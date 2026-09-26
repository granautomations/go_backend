package mqttservice

import (
	"testing"
	"time"
)

func TestDecodeReading(t *testing.T) {

	payload := []byte(`{"value":23, "timestamp":1790103261}`)

	reading, err := decodeReading(payload)

	if err != nil {
		t.Fatal(err)
	}

	if reading.Value == nil || *reading.Value != 23 {
		t.Fatalf("unexpected value: %v", reading.Value)
	}
	if reading.Timestamp == nil || *reading.Timestamp != 1790103261 {
		t.Fatalf("unexpected timestamp: %v", reading.Timestamp)
	}
	_, err = decodeReading([]byte(`{"value":23}`))
	if err == nil {
		t.Fatal("expected an error for a missing timestamp")
	}

	_, err = decodeReading([]byte(`{"value":`))
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestParseTelemetry(t *testing.T) {
	rawTopic := "home/indoor/esp32-1/temperature"
	payload := []byte(`{"value":23, "timestamp":1790103261}`)
	units := map[string]string{
		"temperature": "C",
		"humidity":    "percent",
	}

	telemetryResult, err := parseTelemetry(rawTopic, payload, units)

	if err != nil {
		t.Fatal(err)
	}

	if telemetryResult.Namespace != "home" {
		t.Fatal("Unexpected Namespace")
	}
	if telemetryResult.Location != "indoor" {
		t.Fatal("Unexpected Location")
	}
	if telemetryResult.DeviceID != "esp32-1" {
		t.Fatal("Unexpected DeviceID")
	}
	if telemetryResult.Metric != "temperature" {
		t.Fatal("Unexpected Metric")
	}
	if telemetryResult.Value != 23 {
		t.Fatal("Unexpected Value")
	}
	if telemetryResult.Unit != "C" {
		t.Fatal("Unexpected Unit")
	}

	wantTime := time.Unix(1790103261, 0).UTC()
	if !telemetryResult.Timestamp.Equal(wantTime) {
		t.Fatalf("timestamp = %v, want %v", telemetryResult.Timestamp, wantTime)
	}

	_, err = parseTelemetry("home/garage/esp32-1/pressure", payload, units)
	if err == nil {
		t.Fatal("expected an error for an unsupported metric")
	}

}
