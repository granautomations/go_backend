package mqttservice

import (
	"testing"
)

func TestParseTopic(t *testing.T) {

	parts, err := parseTopic("home/indoor/esp32-1/temperature")

	if err != nil {
		t.Fatal(err)
	}

	if parts.Namespace != "home" || parts.Location != "indoor" || parts.DeviceID != "esp32-1" || parts.Metric != "temperature" {
		t.Fatalf("got deviceID=%q, metric=%q", parts.DeviceID, parts.Metric)
	}

	parts, err = parseTopic("home/garage/esp32-1/pressure")

	if err != nil {
		t.Fatal(err)
	}

	if parts.Namespace != "home" || parts.Location != "garage" || parts.DeviceID != "esp32-1" || parts.Metric != "pressure" {
		t.Fatalf("got deviceID=%q, metric=%q", parts.DeviceID, parts.Metric)
	}

	t.Logf("namespace: %v, location: %s, deviceID: %s, metric: %s\n", parts.Namespace, parts.Location, parts.DeviceID, parts.Metric)

	_, err = parseTopic("home/indoor/esp32-1/temperature/extra")

	if err == nil {
		t.Fatal("invalid topic format")
	}

	_, err = parseTopic("home/indoor//temperature")

	if err == nil {
		t.Fatal("expected an error for an empty device ID")
	}

}

// TestBuildTopic checks valid topic construction and rejection of invalid levels.
func TestBuildTopic(t *testing.T) {

	reading := Telemetry{
		Namespace: "home",
		Location:  "indoor",
		DeviceID:  "esp32-1",
		Metric:    "temperature",
	}
	got, err := buildTopic(reading)
	if err != nil {
		t.Fatal(err)
	}

	if expected := "home/indoor/esp32-1/temperature"; got != expected {
		t.Fatalf("topic = %q, want %q", got, expected)
	}

	tests := []struct {
		name    string
		reading Telemetry
	}{
		{name: "empty namespace",
			reading: Telemetry{
				Namespace: "",
				Location:  "indoor",
				DeviceID:  "esp32-1",
				Metric:    "temperature",
			}},
		{name: "empty location",
			reading: Telemetry{
				Namespace: "home",
				Location:  "",
				DeviceID:  "esp32-1",
				Metric:    "temperature",
			}},
		{name: "empty DeviceID",
			reading: Telemetry{
				Namespace: "home",
				Location:  "indoor",
				DeviceID:  "",
				Metric:    "temperature",
			}},
		{name: "empty Metric",
			reading: Telemetry{
				Namespace: "home",
				Location:  "indoor",
				DeviceID:  "esp32-1",
				Metric:    "",
			}},
		{name: "embedded separator",
			reading: Telemetry{
				Namespace: "home",
				Location:  "indoor",
				DeviceID:  "esp32/1",
				Metric:    "temperature",
			}},
		{name: "single-level wildcard",
			reading: Telemetry{
				Namespace: "home",
				Location:  "indoor+",
				DeviceID:  "esp32-1",
				Metric:    "temperature",
			}},
		{name: "multi-level wildcard",
			reading: Telemetry{
				Namespace: "home",
				Location:  "indoor",
				DeviceID:  "esp32-1",
				Metric:    "temperature#",
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTopic(tt.reading)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got != "" {
				t.Fatalf("topic = %q, want empty topic on error", got)
			}
		})
	}

}
