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
