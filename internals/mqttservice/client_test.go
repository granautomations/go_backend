package mqttservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// fakeSubscribeToken simulates subscription results in tests.
type fakeSubscribeToken struct {
	completed bool
	err       error
	granted   map[string]byte
}

func (f *fakeSubscribeToken) WaitTimeout(_ time.Duration) bool {
	return f.completed
}

func (f *fakeSubscribeToken) Result() map[string]byte {
	return f.granted
}

// These methods panic because subscribeOnce should only use WaitTimeout, Error,
// and Result; an unexpected call should fail the test immediately.
func (f *fakeSubscribeToken) Wait() bool {
	panic("unexpected Wait call")
}

func (f *fakeSubscribeToken) Done() <-chan struct{} {
	panic("unexpected Done call")
}

func (f *fakeSubscribeToken) Error() error {
	return f.err
}

// fakeSubscriptionClient supplies a chosen token for subscription tests.
type fakeSubscriptionClient struct {
	token   mqtt.Token
	filters map[string]byte
}

func (f *fakeSubscriptionClient) SubscribeMultiple(
	filters map[string]byte,
	_ mqtt.MessageHandler,
) mqtt.Token {
	f.filters = filters
	return f.token
}

// Verify at compile time that the fakes satisfy the production interfaces.
var _ subscriptionClient = (*fakeSubscriptionClient)(nil)
var _ mqtt.Token = (*fakeSubscribeToken)(nil)
var _ subscriptionResultToken = (*fakeSubscribeToken)(nil)

// TestOnConnectionLostClearsSubscription verifies that a lost connection
// invalidates the previous subscription acknowledgement.
func TestOnConnectionLostClearsSubscription(t *testing.T) {

	service := &Service{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	service.subscribed.Store(true)

	service.onConnectionLost(nil, errors.New("simulated disconnect"))

	if service.subscribed.Load() {
		t.Fatal("subscription remains acknowledged after connection loss; want false")
	}

}

// TestNewClientOptionsRejectsEmptyTopicFilters verifies that a service
// cannot start without any subscriptions.
func TestNewClientOptionsRejectsEmptyTopicFilters(t *testing.T) {
	_, err := newClientOptions(Config{
		BrokerURL:        "tcp://127.0.0.1:1883",
		ClientID:         "backend-test",
		MaxFutureSkew:    time.Second,
		MinimumTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected an error when no topic filters are configured")
	}
}

// TestSubscribeOnceSuccess verifies the requested filter and granted QoS.
func TestSubscribeOnceSuccess(t *testing.T) {
	const filter = "home/+/+/+"

	service := &Service{
		cfg: Config{TopicFilters: []string{filter}},
	}
	client := &fakeSubscriptionClient{
		token: &fakeSubscribeToken{
			completed: true,
			granted:   map[string]byte{filter: 1},
		},
	}

	if err := service.subscribeOnce(client); err != nil {
		t.Fatal(err)
	}

	qos, found := client.filters[filter]
	if !found || qos != 1 {
		t.Fatalf("requested QoS for %q = %d (found=%t), want 1",
			filter, qos, found)
	}
}

// TestSubscribeOnceTimeout verifies that an unfinished subscription returns an error.
func TestSubscribeOnceTimeout(t *testing.T) {
	const filter = "home/+/+/+"

	service := &Service{
		cfg: Config{TopicFilters: []string{filter}},
	}
	client := &fakeSubscriptionClient{
		token: &fakeSubscribeToken{completed: false},
	}

	if err := service.subscribeOnce(client); err == nil {
		t.Fatal("expected an error when the subscription times out")
	}
}

// TestSubscribeOnceRejectsLowerQoS verifies that a completed subscription is
// rejected when the broker grants less than the requested QoS 1.
func TestSubscribeOnceRejectsLowerQoS(t *testing.T) {
	const filter = "home/+/+/+"

	service := &Service{
		cfg: Config{TopicFilters: []string{filter}},
	}
	client := &fakeSubscriptionClient{
		token: &fakeSubscribeToken{
			completed: true,
			granted:   map[string]byte{filter: 0},
		},
	}

	if err := service.subscribeOnce(client); err == nil {
		t.Fatal("expected an error when the subscription does not grant QoS 1")
	}

	qos, found := client.filters[filter]
	if !found || qos != 1 {
		t.Fatalf("requested QoS for %q = %d (found=%t), want 1",
			filter, qos, found)
	}
}

// fakeSequenceSubscriptionClient returns one token per subscription attempt.
type fakeSequenceSubscriptionClient struct {
	tokens []mqtt.Token
	calls  int
}

func (f *fakeSequenceSubscriptionClient) SubscribeMultiple(
	_ map[string]byte,
	_ mqtt.MessageHandler,
) mqtt.Token {
	if f.calls >= len(f.tokens) {
		panic("unexpected extra subscription attempt")
	}
	token := f.tokens[f.calls]
	f.calls++
	return token
}

var _ subscriptionClient = (*fakeSequenceSubscriptionClient)(nil)

// TestSubscribeWithRetryRecovers verifies that a second attempt succeeds
// after the first subscription attempt fails.
func TestSubscribeWithRetryRecovers(t *testing.T) {

	const filter = "home/+/+/+"

	service := &Service{
		cfg: Config{TopicFilters: []string{filter}},
	}
	client := &fakeSequenceSubscriptionClient{
		tokens: []mqtt.Token{
			&fakeSubscribeToken{
				completed: true,
				err:       errors.New("temporary failure"),
			},
			&fakeSubscribeToken{
				completed: true,
				granted:   map[string]byte{filter: 1},
			},
		},
	}

	if err := service.subscribeWithRetry(client); err != nil {
		t.Fatalf("expected retry to recover, got: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("subscription attempts = %d, want 2", client.calls)
	}

}

// TestSubscribeWithRetryExhausted verifies that retries stop after two
// failures and return the final underlying error.
func TestSubscribeWithRetryExhausted(t *testing.T) {
	const filter = "home/+/+/+"
	secondErr := errors.New("second subscription failure")

	service := &Service{
		cfg: Config{TopicFilters: []string{filter}},
	}
	client := &fakeSequenceSubscriptionClient{
		tokens: []mqtt.Token{
			&fakeSubscribeToken{
				completed: true,
				err:       errors.New("first subscription failure"),
			},
			&fakeSubscribeToken{
				completed: true,
				err:       secondErr,
			},
		},
	}

	err := service.subscribeWithRetry(client)
	if err == nil {
		t.Fatal("expected an error after both attempts failed")
	}
	if client.calls != 2 {
		t.Fatalf("subscription attempts = %d, want 2", client.calls)
	}
	if !errors.Is(err, secondErr) {
		t.Fatalf("error = %v, want it to wrap %v", err, secondErr)
	}
}

// TestNewClientOptionsRejectsNegativeFutureSkew verifies invalid policy
// settings fail during startup rather than during message handling.
func TestNewClientOptionsRejectsNegativeFutureSkew(t *testing.T) {
	cfg := Config{
		BrokerURL:        "tcp://127.0.0.1:1883",
		ClientID:         "backend-test",
		TopicFilters:     []string{"home/+/+/+"},
		MaxFutureSkew:    -time.Second,
		MinimumTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if _, err := newClientOptions(cfg); err == nil {
		t.Fatal("expected an error for negative future skew")
	}
}

// TestNewClientOptionsRejectsMissingMinimumTimestamp verifies that an unset
// lower bound is rejected while an explicitly configured bound is accepted.
func TestNewClientOptionsRejectsMissingMinimumTimestamp(t *testing.T) {
	cfg := Config{
		BrokerURL:     "tcp://127.0.0.1:1883",
		ClientID:      "backend-test",
		TopicFilters:  []string{"home/+/+/+"},
		MaxFutureSkew: time.Second,
	}

	t.Run("minimum timestamp omission", func(t *testing.T) {
		_, err := newClientOptions(cfg)
		if err == nil {
			t.Fatal("expected an error for minimum timestamp omission")
		}

		if !strings.Contains(err.Error(), "minimumTimestamp") {
			t.Fatalf("error = %q, want minimumTimestamp reason", err.Error())
		}
	})

	cfg.MinimumTimestamp = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := newClientOptions(cfg); err != nil {
		t.Fatal(err)
	}
}

// TestParseAndValidateTelemetryRejectsFutureTimestamp verifies that the service
// rejects a parsed reading beyond its configured future-skew allowance.
func TestParseAndValidateTelemetryRejectsFutureTimestamp(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	service := &Service{
		cfg: Config{
			Units:            map[string]string{"temperature": "C"},
			MaxFutureSkew:    5 * time.Minute,
			MinimumTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		// Supply a function so every test call observes the same clock.
		now: func() time.Time { return fixedNow },
	}

	// Build a reading six minutes ahead of the test's fixed clock.
	topic := "home/indoor/esp32-1/temperature"
	payload := []byte(fmt.Sprintf(
		`{"value":22.4,"timestamp":%d}`,
		fixedNow.Add(6*time.Minute).Unix(),
	))

	_, err := service.parseAndValidateTelemetry(topic, payload)
	if err == nil {
		t.Fatal("expected future timestamp to be rejected")
	}

	if !strings.Contains(err.Error(), "exceeds latest allowed") {
		t.Fatalf("error = %q, want it to contain %q", err, "exceeds latest allowed")
	}
}

// fakeTelemetryMessage supplies the MQTT fields read by handleMessage without a broker.
type fakeTelemetryMessage struct {
	topic   string
	payload []byte
}

// Duplicate reports that this test message is not a redelivery.
func (m fakeTelemetryMessage) Duplicate() bool { return false }

// Qos reports the delivery level of this test message.
func (m fakeTelemetryMessage) Qos() byte { return 0 }

// Retained reports that this test message was not retained by the broker.
func (m fakeTelemetryMessage) Retained() bool { return false }

// Topic returns the topic supplied by the test.
func (m fakeTelemetryMessage) Topic() string { return m.topic }

// MessageID returns no broker-assigned identifier for this test message.
func (m fakeTelemetryMessage) MessageID() uint16 { return 0 }

// Payload returns the payload supplied by the test.
func (m fakeTelemetryMessage) Payload() []byte { return m.payload }

// Ack is a no-op because this broker-free test does not acknowledge delivery.
func (m fakeTelemetryMessage) Ack() {}

// Keep the fake aligned with the MQTT library's message contract.
var _ mqtt.Message = fakeTelemetryMessage{}

// TestHandleMessageAppliesTimestampPolicy verifies that received telemetry is
// logged only when its measurement time satisfies the service policy.
func TestHandleMessageAppliesTimestampPolicy(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	minimum := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	const topic = "home/indoor/esp32-1/temperature"

	tests := []struct {
		name       string
		timestamp  time.Time
		wantLog    string
		wantReason string
	}{
		{name: "future", timestamp: fixedNow.Add(6 * time.Minute), wantLog: "mqtt message rejected", wantReason: "exceeds latest allowed"},
		{name: "before minimum", timestamp: minimum.Add(-time.Second), wantLog: "mqtt message rejected", wantReason: "before minimum"},
		{name: "historical", timestamp: fixedNow.Add(-24 * time.Hour), wantLog: "mqtt telemetry received"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := &Service{
				cfg: Config{
					Units:            map[string]string{"temperature": "C"},
					MaxFutureSkew:    5 * time.Minute,
					MinimumTimestamp: minimum,
				},
				logger: slog.New(slog.NewJSONHandler(&logs, nil)),
				// Fix the clock before invoking the callback to avoid wall-time-dependent assertions.
				now: func() time.Time { return fixedNow },
			}
			payload := []byte(fmt.Sprintf(`{"value":22.4,"timestamp":%d}`, tt.timestamp.Unix()))

			service.handleMessage(nil, fakeTelemetryMessage{topic: topic, payload: payload})

			var record struct {
				Message   string    `json:"msg"`
				Topic     string    `json:"topic"`
				Error     string    `json:"error"`
				DeviceID  string    `json:"device_id"`
				Timestamp time.Time `json:"timestamp"`
			}
			if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
				t.Fatalf("decode telemetry log: %v (raw log: %q)", err, logs.String())
			}
			if record.Message != tt.wantLog {
				t.Fatalf("log message = %q, want %q", record.Message, tt.wantLog)
			}
			if tt.wantReason != "" {
				if record.Topic != topic || !strings.Contains(record.Error, tt.wantReason) {
					t.Fatalf("rejection log = %q, want topic %q and reason %q", logs.String(), topic, tt.wantReason)
				}
				return
			}
			if record.DeviceID != "esp32-1" || !record.Timestamp.Equal(tt.timestamp) {
				t.Fatalf("accepted telemetry log = %q, want device and measurement timestamp", logs.String())
			}
		})
	}
}
