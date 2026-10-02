package mqttservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type wireReading struct {
	Value     *float64 `json:"value"`
	Timestamp *int64   `json:"timestamp"`
}

type Telemetry struct {
	Namespace string
	Location  string
	DeviceID  string
	Metric    string
	Value     float64
	Unit      string
	Timestamp time.Time
}

// validateTimestamp accepts measurement times from minimum through
// now plus maxFutureSkew, inclusive, and rejects times outside that range.
// A negative maxFutureSkew is invalid; historical readings within the range remain valid.
func validateTimestamp(timestamp, now, minimum time.Time, maxFutureSkew time.Duration) error {

	// Reject invalid policy before evaluating an otherwise valid reading.
	if maxFutureSkew < 0 {
		return fmt.Errorf("maximum future skew must not be negative: %v", maxFutureSkew)
	}
	// Reject implausibly early device times without limiting legitimate old readings.
	if timestamp.Before(minimum) {
		return fmt.Errorf("timestamp %s is before minimum %s",
			timestamp.UTC().Format(time.RFC3339),
			minimum.UTC().Format(time.RFC3339))
	}
	// Allow small clock differences, but reject readings beyond the future limit.
	latestAllowed := now.Add(maxFutureSkew)
	if timestamp.After(latestAllowed) {
		return fmt.Errorf("timestamp %s exceeds latest allowed %s",
			timestamp.UTC().Format(time.RFC3339),
			latestAllowed.UTC().Format(time.RFC3339))
	}
	return nil
}

// decodeReading decodes a JSON payload with a numeric value and an integer
// Unix-seconds timestamp. Both fields must be present and non-null.
// It does not enforce measurement or timestamp ranges. Invalid input returns
// a zero-valued wireReading and an error.
func decodeReading(payload []byte) (wireReading, error) {
	var reading wireReading
	if err := json.Unmarshal(payload, &reading); err != nil {
		return wireReading{}, err
	}

	if reading.Value == nil || reading.Timestamp == nil {
		return wireReading{}, errors.New("reading requires value and timestamp")
	}

	return reading, nil
}

// parseTelemetry combines a four-level topic and JSON reading into Telemetry.
// The metric must exist in units, which supplies the reading's unit; the
// Unix-seconds timestamp is converted to UTC. Timestamp and measurement ranges
// are not checked. Invalid input returns zero-valued Telemetry and an error.
func parseTelemetry(rawTopic string, payload []byte, units map[string]string) (Telemetry, error) {
	topicParts, parseErr := parseTopic(rawTopic)

	if parseErr != nil {
		return Telemetry{}, parseErr
	}

	reading, decodeErr := decodeReading(payload)

	if decodeErr != nil {
		return Telemetry{}, decodeErr
	}

	unit, ok := units[topicParts.Metric]
	if !ok {
		return Telemetry{}, fmt.Errorf("unsupported metric %q", topicParts.Metric)
	}

	return Telemetry{
		Namespace: topicParts.Namespace,
		Location:  topicParts.Location,
		DeviceID:  topicParts.DeviceID,
		Metric:    topicParts.Metric,
		Value:     *reading.Value,
		Unit:      unit,
		Timestamp: time.Unix(*reading.Timestamp, 0).UTC(),
	}, nil

}

// encodeReading returns JSON containing only value and an integer Unix-seconds
// timestamp, discarding subsecond precision and omitting device metadata and unit.
// It does not validate metrics or timestamp ranges. JSON encoding failures,
// including non-finite values, return a nil payload and a wrapped error.
func encodeReading(reading Telemetry) ([]byte, error) {

	payloadReading := struct {
		Value     float64 `json:"value"`
		Timestamp int64   `json:"timestamp"`
	}{
		Value:     reading.Value,
		Timestamp: reading.Timestamp.Unix(),
	}

	payload, err := json.Marshal(payloadReading)
	if err != nil {
		return nil, fmt.Errorf("encode reading: %w", err)
	}

	return payload, nil

}
