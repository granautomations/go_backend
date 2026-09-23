package mqttservice

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

//MQTT_TEST_BROKER=tcp://127.0.0.1:1883 go test ./internals/mqttservice -run '^TestServiceConnect$' -v

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
	t.Cleanup(service.Close)

	if err := service.Connect(); err != nil {
		t.Fatal(err)
	}
}
