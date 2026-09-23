package mqttservice

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	BrokerURL    string
	ClientID     string
	TopicFilters []string
	Units        map[string]string
}

type Service struct {
	client mqtt.Client
	cfg    Config
	logger *slog.Logger
}

func newClientOptions(cfg Config) (*mqtt.ClientOptions, error) {

	if cfg.BrokerURL == "" {
		return nil, errors.New("brokerURL is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("clientID is required")
	}

	clientOptions := mqtt.NewClientOptions()
	clientOptions.AddBroker(cfg.BrokerURL)
	clientOptions.SetClientID(cfg.ClientID)
	clientOptions.SetCleanSession(false)
	clientOptions.SetAutoReconnect(true)
	clientOptions.SetOrderMatters(false)
	clientOptions.SetConnectTimeout(5 * time.Second)

	return clientOptions, nil

}

func NewService(cfg Config, logger *slog.Logger) (*Service, error) {

	if logger == nil {
		return nil, errors.New("logger required")
	}

	options, err := newClientOptions(cfg)

	if err != nil {
		return nil, err
	}

	service := &Service{
		cfg:    cfg,
		logger: logger,
	}

	options.SetDefaultPublishHandler(service.handleMessage)

	service.client = mqtt.NewClient(options)

	return service, nil
}

func (s *Service) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	reading, err := parseTelemetry(msg.Topic(), msg.Payload(), s.cfg.Units)
	if err != nil {
		s.logger.Error("mqtt message rejected",
			"topic", msg.Topic(),
			"error", err,
		)
		return
	}

	s.logger.Info("mqtt telemetry received",
		"namespace", reading.Namespace,
		"location", reading.Location,
		"device_id", reading.DeviceID,
		"metric", reading.Metric,
		"value", reading.Value,
		"unit", reading.Unit,
		"timestamp", reading.Timestamp,
	)
}

func (s *Service) Connect() error {
	token := s.client.Connect()
	token.Wait()

	if err := token.Error(); err != nil {
		return fmt.Errorf("connect to MQTT broker: %w", err)
	}

	s.logger.Info("mqtt connected", "broker", s.cfg.BrokerURL)
	return nil
}

func (s *Service) Close() {
	s.client.Disconnect(250)
}
