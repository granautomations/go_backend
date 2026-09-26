package mqttservice

import (
	"encoding/json"
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

// TestEncodeReading verifies the wire payload's exact keys and numeric values.
func TestEncodeReading(t *testing.T) {
	reading := Telemetry{
		Namespace: "home",
		Location:  "indoor",
		DeviceID:  "esp32-3",
		Metric:    "temperature",
		Value:     24.4,
		Unit:      "C",
		Timestamp: time.Unix(1790103261, 0).UTC(),
	}

	payload, err := encodeReading(reading)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode generated payload: %v", err)
	}

	if len(fields) != 2 {
		t.Fatalf("payload had %d fields, expected 2", len(fields))
	}

	valueJSON, ok := fields["value"]
	if !ok {
		t.Fatal("payload is missing value")
	}

	timestampJSON, ok := fields["timestamp"]
	if !ok {
		t.Fatal("payload is missing timestamp")
	}

	// Decode into the wire types to verify both representation and value.
	var decodedValue float64
	if err := json.Unmarshal(valueJSON, &decodedValue); err != nil {
		t.Fatalf("decode value: %v", err)
	}
	if decodedValue != reading.Value {
		t.Fatalf("value = %g, want %g", decodedValue, reading.Value)
	}

	var decodedTimestamp int64
	if err := json.Unmarshal(timestampJSON, &decodedTimestamp); err != nil {
		t.Fatalf("decode timestamp: %v", err)
	}
	if expected := reading.Timestamp.Unix(); decodedTimestamp != expected {
		t.Fatalf("timestamp = %d, want %d", decodedTimestamp, expected)
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
