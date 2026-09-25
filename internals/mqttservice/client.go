// Package mqttservice connects to an MQTT broker and processes device telemetry.
package mqttservice

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config defines the broker connection and telemetry subscriptions.
type Config struct {
	// BrokerURL is the MQTT broker address, such as tcp://127.0.0.1:1883.
	BrokerURL string
	// ClientID is the stable identifier used across reconnects.
	ClientID string
	// TopicFilters contains MQTT subscription filters, which may use wildcards.
	TopicFilters []string
	// Units maps supported metric names to the units assigned to readings.
	Units map[string]string
}

// Service manages an MQTT connection and logs received telemetry.
// Construct it with NewService, then call Connect once and Close on shutdown.
type Service struct {
	client     mqtt.Client
	cfg        Config
	logger     *slog.Logger
	ready      chan error  // Result of the initial subscription attempt.
	readyOnce  sync.Once   // Prevents reconnects from sending another startup result.
	subscribed atomic.Bool // Whether the last subscription attempt was acknowledged.
}

// subscriptionClient contains the MQTT operation needed for one subscription attempt.
type subscriptionClient interface {
	SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token
}

// subscriptionResultToken exposes the broker's granted QoS for each filter.
type subscriptionResultToken interface {
	Result() map[string]byte
}

// newClientOptions validates connection settings and configures the Paho client.
func newClientOptions(cfg Config) (*mqtt.ClientOptions, error) {

	if cfg.BrokerURL == "" {
		return nil, errors.New("brokerURL is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("clientID is required")
	}

	if len(cfg.TopicFilters) == 0 {
		return nil, errors.New("at least one topic filter is required")
	}

	clientOptions := mqtt.NewClientOptions()
	clientOptions.AddBroker(cfg.BrokerURL)
	clientOptions.SetClientID(cfg.ClientID)
	// Preserve the broker session so subscriptions and queued QoS messages can resume.
	clientOptions.SetCleanSession(false)
	clientOptions.SetAutoReconnect(true)
	// Message callbacks only log; they do not need to run in delivery order.
	clientOptions.SetOrderMatters(false)
	clientOptions.SetConnectTimeout(5 * time.Second)

	return clientOptions, nil

}

// NewService constructs the MQTT service without connecting to the broker.
// The logger must be non-nil.
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
		// The callback can report before Connect starts receiving.
		ready: make(chan error, 1),
	}

	// Register handlers before connecting so queued messages have a receiver.
	options.SetDefaultPublishHandler(service.handleMessage)
	options.SetOnConnectHandler(service.onConnect)
	options.SetConnectionLostHandler(service.onConnectionLost)

	service.client = mqtt.NewClient(options)

	return service, nil
}

// handleMessage parses an incoming MQTT message and logs either its telemetry
// fields or the reason it was rejected.
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

// Connect establishes the initial MQTT connection and waits until the broker
// acknowledges the configured subscriptions. It is intended to be called once.
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
	case <-time.After(15 * time.Second):
		return errors.New("timed out waiting for MQTT subscription")
	}

	s.logger.Info("mqtt connected", "broker", s.cfg.BrokerURL)
	return nil
}

// onConnect subscribes to the configured filters with up to two subscription attempts on initial connection or reconnection.
func (s *Service) onConnect(client mqtt.Client) {
	s.subscribed.Store(false)

	if err := s.subscribeWithRetry(client); err != nil {
		s.logger.Error("mqtt subscription failed", "error", err)
		s.reportInitialSubscription(err)
		return
	}

	s.subscribed.Store(true)
	// Publish readiness before waking the initial Connect call.
	s.reportInitialSubscription(nil)
	s.logger.Info("mqtt subscription acknowledged", "filters", s.cfg.TopicFilters)
}

// reportInitialSubscription sends only the first subscription result to
// Connect; later reconnect results are handled by logging and readiness state.
func (s *Service) reportInitialSubscription(err error) {
	s.readyOnce.Do(func() {
		s.ready <- err
	})
}

// Close marks the service unavailable and disconnects the MQTT client.
func (s *Service) Close() {
	s.subscribed.Store(false)
	s.client.Disconnect(250)
}

// Ready reports whether the MQTT client is connected and its subscriptions
// have been acknowledged by the broker.
func (s *Service) Ready() bool {
	return s.client.IsConnected() && s.subscribed.Load()
}

// onConnectionLost marks subscriptions unavailable and logs an unexpected
// disconnection.
func (s *Service) onConnectionLost(_ mqtt.Client, err error) {
	s.subscribed.Store(false)
	s.logger.Warn("mqtt connection lost", "broker", s.cfg.BrokerURL,
		"error", err)
}

// subscribeOnce requests the configured filters and verifies that the broker
// granted QoS 1 for each one. It does not retry or update service readiness.
func (s *Service) subscribeOnce(client subscriptionClient) error {

	filters := make(map[string]byte, len(s.cfg.TopicFilters))
	for _, filter := range s.cfg.TopicFilters {
		filters[filter] = 1 // request QoS 1
	}

	token := client.SubscribeMultiple(filters, nil)

	if !token.WaitTimeout(5 * time.Second) {
		err := errors.New("MQTT subscription timed out")
		return err
	}

	if err := token.Error(); err != nil {
		return err
	}

	// A completed token can still contain a rejected or downgraded subscription.
	subToken, ok := token.(subscriptionResultToken)
	if !ok {
		err := fmt.Errorf("unexpected MQTT subscription token: %T", token)
		return err
	}

	granted := subToken.Result()
	for filter := range filters {
		qos, found := granted[filter]
		if !found || qos != 1 {
			err := fmt.Errorf("MQTT subscription %q not granted at QoS 1 (granted %d, found %t)",
				filter, qos, found)
			return err
		}
	}
	return nil
}

// subscribeWithRetry makes at most two subscription attempts and returns
// the last error if neither attempt succeeds.
func (s *Service) subscribeWithRetry(client subscriptionClient) error {
	const maxAttempts = 2

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = s.subscribeOnce(client)
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("subscribe after %d attempts: %w", maxAttempts, lastErr)
}
