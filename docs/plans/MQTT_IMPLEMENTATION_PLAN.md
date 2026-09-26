# MQTT integration learning plan

## Goal and scope

Build an internal Go MQTT service that connects directly to a local Mosquitto broker over TCP. It subscribes to temperature and humidity telemetry, validates and logs received messages, and exposes an internal Go method for publishing sample telemetry. There is no new HTTP route in this milestone. PostgreSQL storage is the next milestone; the first version only logs readings.

The existing HTTP server in `cmd/server/main.go` remains the application entry point. Start and stop the MQTT service alongside it, using the shutdown context already present there. Keep MQTT code outside `internals/httpapi`: the current `readingStore` is in-memory HTTP data, not a persistence boundary for device telemetry.

## Progress and resume point

Last reviewed: **2026-09-26**. We are beginning **step 4**, with parts of step 6 already implemented. Checkboxes track specific work, not completion of the entire milestone.

**Next small learning step:** document `TestEncodeReading` and use `t.Fatal` for its encoding failures. Then add non-finite-value error coverage and decide timestamp validation before implementing the publisher. The encoder is now in application code and documented.

### 1. Local broker

- [x] A local Mosquitto broker was used successfully by the Go integration tests.
- [ ] Document broker startup and how the ESP32 locates it.
- [ ] Record command-line verification of both sample metrics at QoS 1.

### 2. Telemetry model and validation

- [x] Parse four nonempty topic levels without hard-coding namespace or location.
- [x] Decode value and Unix-seconds timestamp, reject missing fields, and derive units from the metric registry.
- [x] Unit tests cover temperature, unsupported metrics, malformed JSON, a missing timestamp, and some invalid topics.
- [ ] Define acceptable timestamp behavior and implement its validation.
- [ ] Expand table-driven tests for humidity, missing/null fields, malformed levels, invalid timestamps, and numeric limits.
- [ ] Document the telemetry types and parsing helpers.
  - `parseTopic`, `decodeReading`, and `parseTelemetry` now document their contracts and validation scope; the telemetry types still need comments.

### 3. Connection, subscription, and logging

- [x] Configure Paho, register handlers before connecting, and verify QoS 1 subscription acknowledgments.
- [x] Track readiness, clear it on disconnect, and log received/rejected telemetry.
- [x] Implement and unit-test up to two immediate subscription attempts.
- [ ] Choose and test recovery behavior after both subscription attempts fail.
- [ ] Add reconnect-attempt logging and internal message/failure counters.
- [ ] Introduce a message-processing boundary before integrating PostgreSQL.

### 4. Internal publisher

- [x] Implement `buildTopic` and its valid-topic test.
- [x] Document the publish-topic builder's contract and validation scope.
- [x] Test invalid publish-topic levels.
  - All seven initial invalid cases share error and empty-topic assertions. The wildcard cases still need descriptive names; exhaustive forbidden-character coverage across every field is a possible later expansion.
- [ ] Validate and JSON-encode outgoing telemetry.
  - A documented `encodeReading` now exists in `telemetry.go`; its valid-payload test verifies exactly two keys and their numeric values. Semantic validation and error-path coverage remain pending.
- [ ] Implement `PublishTelemetry` with QoS 1, no retention, and bounded cancellation-aware waiting.
- [ ] Test success, invalid input, publish failures, timeout, and cancellation.

### 5. Server lifecycle

- [ ] Introduce centralized, typed YAML configuration with startup validation and a documented example file.
- [ ] Move HTTP/MQTT runtime settings out of hard-coded application values and inject them into services.
- [ ] Configure and construct MQTT in `cmd/server/main.go`.
- [ ] Bound startup failure and coordinate MQTT shutdown with the HTTP server.
- [ ] Verify real disconnect/reconnect and shutdown behavior.

### 6. End-to-end acceptance

- [x] Add opt-in broker tests with isolated client IDs and a unique telemetry topic.
- [x] Verify temperature delivery through Mosquitto and assert logged device ID, metric, value, unit, and timestamp.
- [ ] Replace the test's direct Paho publish with the internal publishing method.
- [ ] Verify humidity, namespace/location fields, and the invalid-payload rejection path.
- [ ] Verify publication through a separate subscriber and recovery after a real broker restart.

### Verification evidence

- Latest encoding checkpoint on 2026-09-26: the focused `TestEncodeReading` check and `go test ./... -count=1` passed with the encoder in `telemetry.go`. Broker tests were skipped without `MQTT_TEST_BROKER`. The test still needs its documentation comment and `t.Fatal` on encoding failure before the recommended commit.
- Latest focused check on 2026-09-26: `go test ./internals/mqttservice -run '^TestBuildTopic' -v -count=1` passed all seven invalid-input subtests and the valid-topic check. All invalid cases now share the same assertions.
- `go test ./... -count=1` passed on 2026-09-26. Broker tests were skipped because `MQTT_TEST_BROKER` was not supplied.
- `TestServiceReceivesTelemetry` passed against `tcp://127.0.0.1:1883` on 2026-09-25. It currently publishes directly through Paho at QoS 0 and verifies the received structured telemetry log; it does not yet exercise an internal publishing method.

### Decisions still needed

- Confirmed precision policy: outgoing timestamps use Unix seconds; subsecond precision is discarded, not rejected.
- Timestamp policy: define acceptable dates and clock skew; decoding an `int64` alone does not establish that a timestamp is valid.
- Subscription recovery: two immediate attempts are bounded retry, not ongoing recovery. If both fail while the connection remains open, readiness stays false and no further attempt is scheduled. Choose a deliberate policy before claiming sustained recovery.

## Message contract

| Kind | Topic | Example payload |
| --- | --- | --- |
| Temperature | `home/indoor/<device-id>/temperature` | `{"value":28.10,"timestamp":1790103271}` |
| Humidity | `home/indoor/<device-id>/humidity` | `{"value":37.00,"timestamp":1790103271}` |

Use four topic levels: `<namespace>/<location>/<device-id>/<metric>`. `home` and `indoor` are the initial values, not hard-coded parser requirements. Subscribe initially to `home/+/+/+`; the set of subscription filters can become configuration when other namespaces are needed. MQTT's `+` matches exactly one topic level. Require all four levels to be nonempty and reject `/`, `+`, and `#` in values used to construct publish topics. Keep topic-shape parsing separate from a registry of supported metrics and their units. Initially that registry contains `temperature` → `C` and `humidity` → `percent`; adding `pressure` later adds a metric definition without rewriting the topic parser. Unknown metrics are logged and rejected from telemetry processing. The ESP32 payload contains a numeric `value` and a Unix timestamp in seconds; it has no `unit` field. Require a finite value and a valid timestamp, then derive the unit from the metric registry. Log the parsed fields plus full topic context. Do not treat a received message as trusted merely because it came from the local broker.

## Client choice

Choose [`github.com/eclipse/paho.mqtt.golang`](https://github.com/eclipse/paho.mqtt.golang) for MQTT 3.1.1. Its public API covers TCP connections, publish and subscribe, QoS, automatic reconnect, and connection callbacks. [`github.com/eclipse/paho.golang/autopaho`](https://github.com/eclipse-paho/paho.golang) is the MQTT 5 alternative, with explicit connection management and session expiry controls. MQTT 5 features are not needed for this milestone, so the MQTT 3.1.1 client keeps the first learning steps focused. When implementation begins, pin a released version compatible with the project's Go version rather than using a moving branch.

A stable client ID identifies the backend to the broker; it does not by itself make a session persistent. Use a configured, stable ID such as `automation-backend-dev`, set `CleanSession(false)`, and subscribe at QoS 1. Give each test process its own client ID so it cannot disconnect the running backend. Mosquitto's broker persistence must also be enabled if queued session messages must survive a broker restart. QoS 1 means *at least once*: duplicates are possible, and a successful publish acknowledgment means the broker accepted the message, not that a future PostgreSQL write succeeded.

The ESP32 currently publishes telemetry at QoS 0. A QoS 1 subscription does not upgrade QoS 0 publications. The internal Go publishing method will use QoS 1; any firmware change needed to increase device delivery guarantees must be explained and requested before relying on it.

## Step-by-step implementation

After each step, run the focused test or manual check, explain the result, and only then move on. Write the code yourself; review each step before adding the next concept.

Write unit and integration tests throughout implementation. Step 6 is final acceptance coverage, not the first introduction of broker tests. Keep the progress checklist, verification evidence, unresolved decisions, and resume point current after meaningful changes.

Record meaningful completed changes in the repository-root `changelog.md`. Ask focused questions about intent and implementation tradeoffs when starting a new task, using prior answers and distinguishing non-blocking preferences from decisions needed before implementation.

Include documentation comments for types and functions and inline comments for non-obvious logic in each learning step. Explain configuration fields and behavior, particularly cancellation, concurrency, and failure handling. Add missing comments as existing code is reviewed.

### Centralized configuration approach

Implement this progressively as part of step 5, before wiring MQTT into the server. Configuration is not implemented yet; current HTTP and MQTT timeouts and retry limits still contain hard-coded runtime values.

- Use a documented `config.example.yaml` for non-secret settings and a typed configuration package such as `internals/config`. Select a small YAML decoder when this implementation step begins; a general-purpose configuration framework is not required.
- Load configuration once in `main`, reject unknown keys and invalid/missing required settings, and pass typed settings into HTTP and MQTT components. Do not read files or environment variables independently inside each service.
- Centralize HTTP listen address and read/write/idle/shutdown timeouts, MQTT broker URL and client ID, topic filters, metric-unit definitions, connection/subscription/publish timeouts, subscription retry policy, and logging settings. Document duration syntax, required fields, and any defaults in the example file.
- Keep passwords and future database credentials out of version control. Use explicitly documented environment-based secret overrides and never log secrets. Document precedence so configuration behavior is predictable.
- Keep protocol invariants, such as the four-level topic shape, distinct from deployment settings. Fixed sample values in isolated tests are fixtures, not server configuration.
- Test loading, unknown fields, missing required settings, invalid durations/ranges, and override behavior. Continue passing explicit test configuration into services rather than requiring a developer's configuration file.
- Learn: YAML decoding, typed configuration structs, validation, dependency injection, and separation of configuration from behavior.

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
- Track bounded subscription retry separately from automatic connection recovery. Decide what happens if retries are exhausted while the broker connection remains open; logging and false readiness alone do not schedule recovery.
- Log connection, disconnect, reconnect attempt, subscription failure, parse failure, and received-message events with structured fields. Keep internal counters for connected state, received messages, invalid messages, and publish failures; an HTTP metrics route is not required yet.
- Learn: constructors and interfaces express dependencies; callbacks are functions the library invokes on events; `slog` adds searchable key/value fields; callbacks may run concurrently, so shared counters need synchronization or atomics.

**Done when:** publishing either sample with `mosquitto_pub` produces a structured log entry with the correct device ID, metric, value, unit, and timestamp.

### 4. Add the internal publisher

- Add a service method such as `PublishTelemetry(ctx context.Context, reading Telemetry) error`. Validate the reading, construct its exact topic from namespace, location, device ID, and metric, JSON-encode the payload, and publish at QoS 1 without the retained flag.
- Wait for the publish result with a bounded context or timeout and return a useful error on failure. Do not silently turn a timeout into success.
- Exercise this method from a small local demo command or the integration test. The method is the application API for publishing; no HTTP handler calls it in this milestone.
- Learn: methods attach behavior to a type; `context.Context` carries cancellation and deadlines; JSON encoding turns a Go value into bytes; an MQTT publish acknowledgment has a narrower meaning than end-to-end processing.

**Done when:** the internal method publishes the sample temperature and humidity messages and `mosquitto_sub` can observe them.

### 5. Wire lifecycle into the server

- Implement and test the centralized configuration approach above, then replace hard-coded runtime settings in HTTP and MQTT code. Document how to select the configuration file and supply secrets.
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
