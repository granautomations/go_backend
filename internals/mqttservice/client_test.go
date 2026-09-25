package mqttservice

import (
	"errors"
	"io"
	"log/slog"
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
		BrokerURL: "tcp://127.0.0.1:1883",
		ClientID:  "backend-test",
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
