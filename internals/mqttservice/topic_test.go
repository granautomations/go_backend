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
