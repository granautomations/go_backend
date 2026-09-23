package mqttservice

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
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
	client    mqtt.Client
	cfg       Config
	logger    *slog.Logger
	ready     chan error
	readyOnce sync.Once
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
		ready:  make(chan error, 1),
	}

	options.SetDefaultPublishHandler(service.handleMessage)
	options.SetOnConnectHandler(service.onConnect)

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

	select {
	case err := <-s.ready:
		if err != nil {
			return fmt.Errorf("subscribe to MQTT topics: %w", err)
		}
	case <-time.After(10 * time.Second):
		return errors.New("timed out waiting for MQTT subscription")
	}

	s.logger.Info("mqtt connected", "broker", s.cfg.BrokerURL)
	return nil
}

func (s *Service) onConnect(client mqtt.Client) {
	filters := make(map[string]byte, len(s.cfg.TopicFilters))
	for _, filter := range s.cfg.TopicFilters {
		filters[filter] = 1 // request QoS 1
	}

	token := client.SubscribeMultiple(filters, nil)
	token.Wait()
	if err := token.Error(); err != nil {
		s.logger.Error("mqtt subscription failed", "error", err)
		s.reportInitialSubscription(err)
		return
	}

	subToken, ok := token.(*mqtt.SubscribeToken)
	if !ok {
		err := fmt.Errorf("unexpected MQTT subscription token: %T", token)
		s.logger.Error("mqtt subscription failed", "error", err)
		s.reportInitialSubscription(err)
		return
	}

	granted := subToken.Result()
	for filter := range filters {
		qos, found := granted[filter]
		if !found || qos != 1 {
			err := fmt.Errorf("MQTT subscription %q not granted at QoS 1 (granted %d, found %t)",
				filter, qos, found)
			s.logger.Error("mqtt subscription failed", "error", err)
			s.reportInitialSubscription(err)
			return
		}
	}

	s.reportInitialSubscription(nil)
	s.logger.Info("mqtt subscription acknowledged", "filters", s.cfg.TopicFilters)
}

func (s *Service) reportInitialSubscription(err error) {
	s.readyOnce.Do(func() {
		s.ready <- err
	})
}

func (s *Service) Close() {
	s.client.Disconnect(250)
}
