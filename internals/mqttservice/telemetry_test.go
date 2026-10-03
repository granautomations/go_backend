package mqttservice

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// TestDecodeReading verifies required JSON fields and malformed-payload errors.
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

// TestParseTelemetry verifies topic context, unit lookup, and UTC timestamp conversion.
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

// TestEncodeReadingRejectsNonFiniteValue verifies encoding fails without a payload.
func TestEncodeReadingRejectsNonFiniteValue(t *testing.T) {
	tests := []struct {
		name    string
		reading Telemetry
	}{
		{name: "not a number",
			reading: Telemetry{
				Value:     math.NaN(),
				Timestamp: time.Unix(1790103261, 0).UTC(),
			}},
		{name: "positive infinity",
			reading: Telemetry{
				Value:     math.Inf(1),
				Timestamp: time.Unix(1790103261, 0).UTC(),
			}},
		{name: "negative infinity",
			reading: Telemetry{
				Value:     math.Inf(-1),
				Timestamp: time.Unix(1790103261, 0).UTC(),
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := encodeReading(tt.reading)
			if err == nil {
				t.Fatal("expected an error for a non-finite value")
			}
			if payload != nil {
				t.Fatalf("payload = %q, want nil", payload)
			}
		})
	}

}

// TestValidateTimestamp checks inclusive date and future-skew boundaries.
func TestValidateTimestamp(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	minimum := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	maxFutureSkew := 5 * time.Minute

	tests := []struct {
		name        string
		timestamp   time.Time
		wantErr     bool
		wantMessage string
	}{
		{

			name:        "before minimum",
			timestamp:   minimum.Add(-time.Second),
			wantErr:     true,
			wantMessage: "before minimum",
		},
		{

			name:        "exact minimum",
			timestamp:   minimum,
			wantErr:     false,
			wantMessage: "",
		},
		{

			name:        "historical reading",
			timestamp:   now.Add(-24 * time.Hour),
			wantErr:     false,
			wantMessage: "",
		},
		{

			name:        "current reading",
			timestamp:   now,
			wantErr:     false,
			wantMessage: "",
		},
		{

			name:        "exact future boundary",
			timestamp:   now.Add(maxFutureSkew),
			wantErr:     false,
			wantMessage: "",
		},
		{

			name:        "beyond future boundary",
			timestamp:   now.Add(maxFutureSkew + time.Second),
			wantErr:     true,
			wantMessage: "exceeds latest allowed",
		},
		{

			name:        "zero time",
			timestamp:   time.Time{},
			wantErr:     true,
			wantMessage: "before minimum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimestamp(tt.timestamp, now, minimum, maxFutureSkew)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("validateTimestamp(%v) error = %v, want error = %t",
					tt.timestamp, err, tt.wantErr)
			}
			if tt.wantMessage != "" && !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantMessage)
			}
		})
	}

	if err := validateTimestamp(now, now, minimum, 0); err != nil {
		t.Fatalf("zero future skew should be valid: %v", err)
	}

	t.Run("negative future skew", func(t *testing.T) {
		err := validateTimestamp(
			now.Add(-time.Minute), now, minimum, -time.Second,
		)

		if err == nil {
			t.Fatal("expected an error for negative future skew")
		}
		if !strings.Contains(err.Error(), "must not be negative") {
			t.Fatalf("error = %q, want negative-skew reason", err.Error())
		}
	})
}
