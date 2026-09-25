package mqttservice

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestServiceConnect checks readiness before connection, after subscription,
// and after shutdown against a running MQTT broker.
//
// Run with MQTT_TEST_BROKER=tcp://127.0.0.1:1883 and select this test with
// go test ./internals/mqttservice -run '^TestServiceConnect$' -v.
func TestServiceConnect(t *testing.T) {
	brokerURL := os.Getenv("MQTT_TEST_BROKER")
	if brokerURL == "" {
		t.Skip("set MQTT_TEST_BROKER to run the broker test")
	}

	cfg := Config{
		BrokerURL:    brokerURL,
		ClientID:     fmt.Sprintf("backend-test-%d", time.Now().UnixNano()),
		TopicFilters: []string{"home/+/+/+"},
		Units:        map[string]string{"temperature": "C", "humidity": "percent"},
	}

	service, err := NewService(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		service.Close()
		if service.Ready() {
			t.Error("service should not be ready after closing")
		}
	})

	if service.Ready() {
		t.Fatal("Service should not be ready before connecting")
	}

	if err := service.Connect(); err != nil {
		t.Fatal(err)
	}

	if !service.Ready() {
		t.Fatal("service should be ready after subscriptions are acknowledged")
	}
}

// logCaptureWriter sends complete log writes to a channel for assertions.
type logCaptureWriter struct {
	lines chan string
}

func (w *logCaptureWriter) Write(p []byte) (int, error) {
	select {
	case w.lines <- string(p):
	default:
	}
	return len(p), nil
}

var _ io.Writer = (*logCaptureWriter)(nil)

// TestServiceReceivesTelemetry sets up an isolated subscription against a real
// MQTT broker for an end-to-end telemetry delivery check. Set MQTT_TEST_BROKER
// (for example, tcp://127.0.0.1:1883) to run it; otherwise the test is skipped.
func TestServiceReceivesTelemetry(t *testing.T) {
	brokerURL := os.Getenv("MQTT_TEST_BROKER")
	if brokerURL == "" {
		t.Skip("set MQTT_TEST_BROKER to run the broker test")
	}

	deviceID := fmt.Sprintf("test-device-%d", time.Now().UnixNano())
	topic := "home/indoor/" + deviceID + "/temperature"

	cfg := Config{
		BrokerURL:    brokerURL,
		ClientID:     "backend-" + deviceID,
		TopicFilters: []string{topic},
		Units:        map[string]string{"temperature": "C"},
	}

	lines := make(chan string, 16)
	logger := slog.New(slog.NewJSONHandler(&logCaptureWriter{lines: lines}, nil))

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)

	if err := service.Connect(); err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"value":22.4,"timestamp":1790103261}`)
	token := service.client.Publish(topic, 0, false, payload)

	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("timed out publishing telemetry")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("publish telemetry: %v", err)
	}

	deadline := time.After(5 * time.Second)

	for {
		select {
		case line := <-lines:
			var record struct {
				Message   string    `json:"msg"`
				DeviceID  string    `json:"device_id"`
				Metric    string    `json:"metric"`
				Value     float64   `json:"value"`
				Unit      string    `json:"unit"`
				Timestamp time.Time `json:"timestamp"`
			}
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("decode log line: %v", err)
			}
			if record.Message != "mqtt telemetry received" {
				continue
			}

			if record.DeviceID != deviceID ||
				record.Metric != "temperature" ||
				record.Value != 22.4 ||
				record.Unit != "C" ||
				!record.Timestamp.Equal(time.Unix(1790103261, 0).UTC()) {
				t.Fatalf("unexpected telemetry log: %s", line)
			}

			t.Logf("received telemetry log: %s", line)
			return

		case <-deadline:
			t.Fatal("timed out waiting for telemetry log")
		}
	}

}
