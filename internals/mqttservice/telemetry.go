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
