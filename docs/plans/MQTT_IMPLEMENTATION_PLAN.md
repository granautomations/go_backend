# MQTT integration learning plan

## Goal and scope

Build an internal Go MQTT service that connects directly to a local Mosquitto broker over TCP. It subscribes to temperature and humidity telemetry, validates and logs received messages, and exposes an internal Go method for publishing sample telemetry. There is no new HTTP route in this milestone. PostgreSQL storage is the next milestone; the first version only logs readings.

The existing HTTP server in `cmd/server/main.go` remains the application entry point. Start and stop the MQTT service alongside it, using the shutdown context already present there. Keep MQTT code outside `internals/httpapi`: the current `readingStore` is in-memory HTTP data, not a persistence boundary for device telemetry.

### Message contract

| Kind | Topic | Example payload |
| --- | --- | --- |
| Temperature | `home/indoor/<device-id>/temperature` | `{"value":28.10,"timestamp":1790103271}` |
| Humidity | `home/indoor/<device-id>/humidity` | `{"value":37.00,"timestamp":1790103271}` |

Use four topic levels: `<namespace>/<location>/<device-id>/<metric>`. `home` and `indoor` are the initial values, not hard-coded parser requirements. Subscribe initially to `home/+/+/+`; the set of subscription filters can become configuration when other namespaces are needed. MQTT's `+` matches exactly one topic level. Require all four levels to be nonempty and reject `/`, `+`, and `#` in values used to construct publish topics. Keep topic-shape parsing separate from a registry of supported metrics and their units. Initially that registry contains `temperature` → `C` and `humidity` → `percent`; adding `pressure` later adds a metric definition without rewriting the topic parser. Unknown metrics are logged and rejected from telemetry processing. The ESP32 payload contains a numeric `value` and a Unix timestamp in seconds; it has no `unit` field. Require a finite value and a valid timestamp, then derive the unit from the metric registry. Log the parsed fields plus full topic context. Do not treat a received message as trusted merely because it came from the local broker.

## Client choice

Choose [`github.com/eclipse/paho.mqtt.golang`](https://github.com/eclipse/paho.mqtt.golang) for MQTT 3.1.1. Its public API covers TCP connections, publish and subscribe, QoS, automatic reconnect, and connection callbacks. [`github.com/eclipse/paho.golang/autopaho`](https://github.com/eclipse-paho/paho.golang) is the MQTT 5 alternative, with explicit connection management and session expiry controls. MQTT 5 features are not needed for this milestone, so the MQTT 3.1.1 client keeps the first learning steps focused. When implementation begins, pin a released version compatible with the project's Go version rather than using a moving branch.

A stable client ID identifies the backend to the broker; it does not by itself make a session persistent. Use a configured, stable ID such as `automation-backend-dev`, set `CleanSession(false)`, and subscribe at QoS 1. Give each test process its own client ID so it cannot disconnect the running backend. Mosquitto's broker persistence must also be enabled if queued session messages must survive a broker restart. QoS 1 means *at least once*: duplicates are possible, and a successful publish acknowledgment means the broker accepted the message, not that a future PostgreSQL write succeeded.

## Step-by-step implementation

After each step, run the focused test or manual check, explain the result, and only then move on. Write the code yourself; review each step before adding the next concept.

### 1. Establish a local broker and inspect the wire contract

- Run Mosquitto on the development machine at port 1883, reachable from both the Go backend and the ESP32 on the local network. Document how to start it and how the ESP32 finds the broker. Add a small repository configuration or Compose setup only if it makes local setup reproducible.
- Use `mosquitto_sub` on `home/+/+/+` and `mosquitto_pub -q 1` to send the sample temperature JSON. Repeat for humidity. Confirm the exact topic and payload received.
- Learn: a broker routes messages by topic; a publisher and subscriber do not call each other directly. A subscription filter can contain wildcards; a published topic cannot.

**Done when:** the command-line clients exchange both sample messages at QoS 1.

### 2. Model and validate one telemetry message

- Add an MQTT-focused package, for example `internals/mqttservice`, with a telemetry type and a parser that takes `(topic string, payload []byte)` and returns a reading or an error.
- Use a small wire struct with JSON tags, `float64` for the measurement, and `int64` for Unix seconds. Convert the timestamp with `time.Unix` when creating the internal reading. Parse four nonempty topic levels into namespace, location, device ID, and metric. Validate supported metrics against a separate registry, then derive the unit. Check missing fields, timestamp, and payload. Keep parsing independent of the MQTT library so it is easy to test.
- Write table-driven unit tests for valid topics in different locations, malformed topics, supported and unsupported metrics, invalid JSON, missing fields, bad timestamp, and non-finite value.
- Learn: `struct` and JSON tags define the data contract; `[]byte` is the bytes received from the network; `(value, error)` makes failure explicit; table-driven tests exercise one rule with many examples.

**Done when:** parsing produces a typed reading for both examples and clear errors for invalid input.

### 3. Connect, subscribe, and log

- Add a service with configuration for broker URL and client ID. Accept a `*slog.Logger` and a small message-processing function or interface so logging can later be replaced by PostgreSQL persistence.
- Configure TCP, a stable client ID, `CleanSession(false)`, QoS 1, automatic reconnect, a finite connection timeout, and connection callbacks. Register the message handler before connecting so resumed-session deliveries have a handler.
- On each connection, confirm the configured topic subscriptions are active; log subscription errors. In the message handler, parse and log the reading. Keep this callback short so it does not stall network handling.
- Log connection, disconnect, reconnect attempt, subscription failure, parse failure, and received-message events with structured fields. Keep internal counters for connected state, received messages, invalid messages, and publish failures; an HTTP metrics route is not required yet.
- Learn: constructors and interfaces express dependencies; callbacks are functions the library invokes on events; `slog` adds searchable key/value fields; callbacks may run concurrently, so shared counters need synchronization or atomics.

**Done when:** publishing either sample with `mosquitto_pub` produces a structured log entry with the correct device ID, metric, value, unit, and timestamp.

### 4. Add the internal publisher

- Add a service method such as `PublishTelemetry(ctx context.Context, reading Telemetry) error`. Validate the reading, construct its exact topic from device ID and metric, JSON-encode the payload, and publish at QoS 1 without the retained flag.
- Wait for the publish result with a bounded context or timeout and return a useful error on failure. Do not silently turn a timeout into success.
- Exercise this method from a small local demo command or the integration test. The method is the application API for publishing; no HTTP handler calls it in this milestone.
- Learn: methods attach behavior to a type; `context.Context` carries cancellation and deadlines; JSON encoding turns a Go value into bytes; an MQTT publish acknowledgment has a narrower meaning than end-to-end processing.

**Done when:** the internal method publishes the sample temperature and humidity messages and `mosquitto_sub` can observe them.

### 5. Wire lifecycle into the server

- Construct the MQTT service in `cmd/server/main.go`, start it with the app, and stop it during the existing signal-driven shutdown. Define startup behavior explicitly: fail startup with a clear error if the initial broker connection cannot be established within a short timeout; use automatic reconnect after a connection that was established successfully.
- Ensure the MQTT client stops accepting new work and disconnects before process exit. Keep the HTTP and MQTT shutdown paths understandable and bounded.
- Learn: `main` composes services; contexts signal cancellation; a goroutine can run independent work; graceful shutdown gives in-flight work a bounded chance to finish.

**Done when:** stopping the process closes the MQTT connection cleanly, and temporarily stopping and restarting Mosquitto produces disconnect/reconnect logs followed by successful reception again.

### 6. Prove behavior against Mosquitto

- Add opt-in integration tests that run only when a local test broker address is supplied, for example `MQTT_TEST_BROKER=tcp://127.0.0.1:1883`. Keep ordinary `go test ./...` independent of a running broker.
- Give each test a unique client ID and device ID. Start the subscriber, wait for the subscription to succeed, call the internal publish method, and assert the received parsed fields through a channel with a timeout. Test both metrics and one invalid-payload log/error path.
- Test recovery by interrupting the local broker and reconnecting it during a manual check, or in a dedicated integration test if the test owns its broker process. Avoid using arbitrary sleeps to guess readiness.
- Learn: integration tests verify real protocol behavior; channels pass observations between callbacks and tests; timeouts prevent tests from hanging; unique IDs isolate tests from the running app.

**Done when:** unit tests pass without Mosquitto, integration tests pass against Mosquitto, publishing is verified both through the internal method and a separate subscriber, and reconnection is observed.

## Next milestone: PostgreSQL

Replace the logging-only message processor with an interface implemented by a PostgreSQL-backed reading repository. Preserve the same topic and payload contract. Decide how to handle QoS 1 redelivery before writing rows: a timestamp alone may not uniquely identify an event, so add an event ID or another deduplication key if duplicate rows would be harmful. Define acknowledgment and failure behavior deliberately; logging a failed insert while acknowledging the MQTT message can lose that reading from the application's point of view.

## References

- [Eclipse Paho Go MQTT 3.1.1 client](https://github.com/eclipse/paho.mqtt.golang)
- [Eclipse Paho MQTT 5 Go client](https://github.com/eclipse-paho/paho.golang)
- [Mosquitto publish command](https://mosquitto.org/man/mosquitto_pub-1.html)
- [Mosquitto broker configuration and persistence](https://mosquitto.org/man/mosquitto-conf-5.html)
