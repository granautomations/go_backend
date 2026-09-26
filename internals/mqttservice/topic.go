package mqttservice

import (
	"errors"
	"slices"
	"strings"
)

type topicParts struct {
	Namespace string
	Location  string
	DeviceID  string
	Metric    string
}

// parseTopic splits a topic shaped as <namespace>/<location>/<device-id>/<metric>.
// It requires exactly four nonempty levels but does not restrict their names
// or check supported metrics; metric validation belongs to parseTelemetry.
// Invalid input returns zero-valued topicParts and an error.
func parseTopic(raw string) (topicParts, error) {
	levels := strings.Split(raw, "/")
	if len(levels) != 4 || slices.Contains(levels, "") {
		return topicParts{}, errors.New("invalid topic format")
	}

	return topicParts{
		Namespace: levels[0],
		Location:  levels[1],
		DeviceID:  levels[2],
		Metric:    levels[3],
	}, nil
}

// buildTopic joins the reading's namespace, location, device ID, and metric
// into a publish topic. Each level must be nonempty and contain no '/', '+', or '#'.
// Invalid input returns an empty topic and an error. Supported metrics and
// payload fields are not validated here.
func buildTopic(reading Telemetry) (string, error) {
	levels := []string{
		reading.Namespace,
		reading.Location,
		reading.DeviceID,
		reading.Metric,
	}
	for _, level := range levels {
		if level == "" || strings.ContainsAny(level, "/+#") {
			return "", errors.New("invalid publish topic level")
		}
	}

	return strings.Join(levels, "/"), nil
}
