# MQTT integration learning plan

## Goal and scope

Build an internal Go MQTT service that connects directly to a local Mosquitto broker over TCP. It subscribes to temperature and humidity telemetry, validates and logs received messages, and exposes an internal Go method for publishing sample telemetry. There is no new HTTP route in this milestone. PostgreSQL storage is the next milestone; the first version only logs readings.

The existing HTTP server in `cmd/server/main.go` remains the application entry point. Start and stop the MQTT service alongside it, using the shutdown context already present there. Keep MQTT code outside `internals/httpapi`: the current `readingStore` is in-memory HTTP data, not a persistence boundary for device telemetry.

## Progress and resume point

Last reviewed: **2026-10-03** against the backend code. We are in **step 4**: topic construction and payload encoding are implemented and tested; timestamp validation is wired into received-message handling but not yet into an internal publisher, which remains unimplemented. Parts of step 6 are already implemented. Checkboxes track specific work, not completion of the entire milestone.

**Next small learning step:** verify the updated received-message path against Mosquitto when a test broker is available, then return to the internal publisher's validation and QoS 1 method. Broker-free tests now prove the callback rejects future and pre-minimum timestamps while accepting a historical reading. The outgoing path remains unimplemented; do not claim it enforces the policy yet.

### 1. Local broker

- [x] A local Mosquitto broker was used successfully by the Go integration tests.
- [ ] Document broker startup and how the ESP32 locates it.
- [ ] Record command-line verification of both sample metrics at QoS 1.

### 2. Telemetry model and validation

- [x] Parse four nonempty topic levels without hard-coding namespace or location.
- [x] Decode value and Unix-seconds timestamp, reject missing fields, and derive units from the metric registry.
- [x] Unit tests cover temperature, unsupported metrics, malformed JSON, a missing timestamp, and some invalid topics.
- [ ] Define acceptable timestamp behavior and implement its validation.
  - Inclusive lower/upper-bound policy, zero-skew acceptance, and negative-skew rejection are covered by helper tests. Received-message handling now calls the validator and has broker-free rejection/acceptance coverage; the outgoing publishing path is still absent.
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
  - A documented `encodeReading` now exists in `telemetry.go`; tests verify exactly two keys and their numeric values, and reject NaN and both infinities with errors and nil payloads. Timestamp validation remains pending.
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

- On 2026-10-03, comments on all `Service` fields were normalized above their declarations without changing behavior. `go test ./internals/mqttservice -count=1`, formatting, and staged/unstaged diff checks passed; broker-backed tests were not run.
- On 2026-10-03, focused `TestParseAndValidateTelemetryRejectsFutureTimestamp` and `TestHandleMessageAppliesTimestampPolicy`, `go test ./... -count=1`, `go vet ./...`, and `git diff --check` passed after wiring `handleMessage` to the validated service method. The new broker-free handler test covers future rejection, pre-minimum rejection, and historical acceptance with structured-log assertions. Broker-backed tests were skipped without `MQTT_TEST_BROKER`; a local TCP probe of `127.0.0.1:1883` did not find a listening broker.
- Earlier on 2026-10-03, the new handler test failed as intended: future and pre-minimum readings were logged as received, while the historical reading passed. This established the missing callback wiring before implementation.
- On 2026-10-03, focused `TestParseAndValidateTelemetryRejectsFutureTimestamp` and `go test ./... -count=1` passed after the service method returned contextual parsing/validation errors. Broker-backed tests were skipped without `MQTT_TEST_BROKER`. `handleMessage` still bypasses the method, and the new test lacks its documentation comment.
- On 2026-10-03, the focused `TestParseAndValidateTelemetryRejectsFutureTimestamp` compiled but failed at `expected future timestamp to be rejected`. `parseAndValidateTelemetry` calls the validator but mistakenly returns the earlier nil parse error when validation fails. `go vet ./internals/mqttservice` passed; handler wiring is still absent.
- On 2026-10-03, the focused `TestParseAndValidateTelemetryRejectsFutureTimestamp` build failed only with `service.parseAndValidateTelemetry undefined`, the intended clean red test-first result. The test still needs its documentation comment; no service validation method or handler wiring exists yet.
- On 2026-10-03, the next focused future-timestamp test run stopped at `missing ',' before newline in composite literal`: the draft assigns a `time.Time` to the `now func() time.Time` field and references an undefined `fixedNow`. The assertion now expects rejection, but the test has not reached the intended missing-method compile failure.
- On 2026-10-03, the revised focused future-timestamp test still failed to compile: `_, err := ...` has no new nonblank variable because `err` was already declared, and `parseAndValidateTelemetry` is not yet implemented. The draft's `NewService` call would also reject its missing broker URL/client ID before the timestamp assertion; no valid red test exists yet.
- On 2026-10-03, the first `TestParseAndValidateTelemetryRejectsFutureTimestamp` run failed to compile with the expected missing `parseAndValidateTelemetry` method plus unintended undefined `topic` and `payload` variables. The draft test also reads `MQTT_TEST_BROKER`, so it is not yet broker-independent. Do not treat this as a clean test-first red result.
- On 2026-10-03, `go test ./... -count=1`, `go vet ./...`, and `git diff --check` passed after the fixed integration-test clock was removed. The injectable clock remains unused, so these checks do not verify timestamp-policy behavior; broker-backed tests were skipped without `MQTT_TEST_BROKER`.
- On 2026-10-03, `go test ./internals/mqttservice -count=1` and `git diff --check` passed after a `now func() time.Time` field was initialized to `time.Now` in `NewService`. This does not verify clock-based behavior because the field is not read yet; the integration test's new fixed clock is inconsistent with its sample payload and should be removed before wiring validation.
- On 2026-10-03, `go test ./... -count=1`, `go vet ./...`, and `git diff --check` passed after documenting the minimum-timestamp test, adding the date to both broker-test fixtures, and restoring idiomatic `t.Fatal(err)` calls. A focused verbose run confirmed `TestServiceConnect` and `TestServiceReceivesTelemetry` were skipped because `MQTT_TEST_BROKER` was unset; broker behavior was not reverified.
- On 2026-10-03, `go test ./... -count=1`, `go vet ./...`, and `git diff --check` passed using a temporary Go build cache because the default cache was inaccessible to the sandbox. `MQTT_TEST_BROKER` was unset, so broker-backed tests were skipped. Their two `Config` fixtures still omit the required `MinimumTimestamp` and would fail service construction when enabled.
- On 2026-10-03, `TestNewClientOptionsRejectsMissingMinimumTimestamp`, `go test ./... -count=1`, and `git diff --check` passed after the guard and positive test case were added. The ordinary suite skipped broker-backed checks; their `Config` fixtures still need the required minimum timestamp. The empty-topic-filter test also has an unrelated negative-skew value that should be corrected before calling the checkpoint complete.
- Earlier on 2026-10-03, `TestNewClientOptionsRejectsMissingMinimumTimestamp` failed as intended because `newClientOptions` accepted an otherwise valid configuration without a minimum measurement date. After the guard was added, the test still needed a populated success case; that case was fixed before the later passing check above.
- On 2026-10-03, `go test ./... -count=1` passed after adding startup validation for negative `MaxFutureSkew` and comments explaining the broker, client-ID, and subscription requirements. Broker-backed tests were skipped without `MQTT_TEST_BROKER`.
- On 2026-10-03, `TestNewClientOptionsRejectsNegativeFutureSkew` compiled but failed (`expected an error for negative future skew`) after the field became `time.Duration`. This is the intended red test; do not claim the current full suite passes until the guard is implemented and verified.
- On 2026-10-03, the first `TestNewClientOptionsRejectsNegativeFutureSkew` run failed to compile: `MaxFutureSkew` was declared `time.Time`, but the test assigns `-time.Second` (`time.Duration`). No service policy validation has been implemented yet.
- On 2026-10-03, `go test ./... -count=1` and `git diff --check` passed after the negative-skew subtest began checking the specific rejection reason. No broker test ran in this check; service-level timestamp validation remains unimplemented.
- On 2026-10-03, focused `TestValidateTimestamp` and `go test ./... -count=1` passed after adding reason checks for minimum/future timestamp errors. The negative-skew case still checks only error presence, not its reason. Broker tests were skipped without `MQTT_TEST_BROKER`.
- On 2026-10-03, focused `TestValidateTimestamp` passed. The tests do not yet assert specific rejection reasons; service and publisher paths do not call the helper.
- On 2026-10-02, `go test ./... -count=1` passed after explicit negative-skew validation and concise branch comments were added. Broker-backed tests were skipped without `MQTT_TEST_BROKER`.
- On 2026-10-02, the corrected negative-skew test used `now.Add(-time.Minute)` and failed as intended (`negative future skew should be rejected`). Other timestamp boundary subtests passed; the explicit policy guard is not yet implemented.
- On 2026-10-02, the focused timestamp test passed after zero/negative-skew cases were added, but the negative case is a false positive: `timestamp == now` exceeds `now.Add(-time.Second)` even without configuration validation. The helper still lacks an explicit negative-skew guard.
- On 2026-10-02, `go test ./... -count=1` and `git diff --check` passed with `validateTimestamp` moved to production `telemetry.go`. The helper formats actual and allowed UTC times in boundary errors. No broker tests ran in this check.
- On 2026-10-02, `go test ./internals/mqttservice -run '^TestValidateTimestamp$' -v -count=1` passed all seven boundary subtests using a test-only draft helper. This does not verify production integration or useful error text; the helper still lives in `_test.go` and returns placeholder errors.
- On 2026-10-02, the first focused `TestValidateTimestamp` run failed to compile with `undefined: validateTimestamp`, as expected for the test-first stage. The test's current `err != tt.wantErr` comparison also needs correction before implementation.
- Review on 2026-10-02: `go test ./... -count=1` passed. Broker-backed tests were skipped because `MQTT_TEST_BROKER` was not set. No `validateTimestamp`, `PublishTelemetry`, or centralized configuration package exists yet; `cmd/server/main.go` still starts only HTTP.
- Commit checkpoint on 2026-09-27: `go test ./... -count=1` and `git diff --check` passed. Broker tests were skipped without `MQTT_TEST_BROKER`. The non-finite-value test now has the required function-name documentation comment.
- Latest encoding checkpoint on 2026-09-26: `go test ./internals/mqttservice -run '^TestEncodeReading' -v -count=1` passed the valid payload test and all three non-finite-value subtests. The previous full-suite check also passed with the encoder in `telemetry.go`; broker tests were skipped without `MQTT_TEST_BROKER`.
- Latest focused check on 2026-09-26: `go test ./internals/mqttservice -run '^TestBuildTopic' -v -count=1` passed all seven invalid-input subtests and the valid-topic check. All invalid cases now share the same assertions.
- `go test ./... -count=1` passed on 2026-09-26. Broker tests were skipped because `MQTT_TEST_BROKER` was not supplied.
- `TestServiceReceivesTelemetry` passed against `tcp://127.0.0.1:1883` on 2026-09-25. It currently publishes directly through Paho at QoS 0 and verifies the received structured telemetry log; it does not yet exercise an internal publishing method.

### Decisions and dependencies

- Firmware inspection on 2026-09-26 found existing telemetry publishers gated by clock validity, contrary to the earlier no-telemetry report. Their timestamps currently describe publication of cached values, not captured measurement epochs. See [ESP32 timestamp handoff](ESP32_TELEMETRY_TIMESTAMP_HANDOFF.md); no firmware changes or tests were performed during this inspection.
- Subsequent implementation review on 2026-09-26 confirmed captured measurement timestamps and application snapshot wiring in the updated firmware. ESP32 build and 32 native tests passed (native tests required per-command Xcode SDK selection). Device verification is still pending; the handoff records remaining coverage and timestamp-width/documentation limitations. No firmware implementation was changed by the reviewing agent.
- Confirmed precision policy: outgoing timestamps use Unix seconds; subsecond precision is discarded, not rejected.
- Confirmed history policy (2026-09-27): accept valid delayed/historical readings and preserve their measurement timestamps. Do not reject solely because a reading is old. Freshness checks for future automations are separate from ingestion validity; this does not imply the firmware buffers offline readings.
- Confirmed future-skew policy (2026-09-27): initial configurable tolerance is five minutes. Accept the exact boundary and reject later timestamps from normal processing with a useful error/log; preserve the original measurement time. Supply reference time explicitly in tests and configuration through service dependencies. Backend clock accuracy is an operational prerequisite.
- Confirmed configuration boundary (2026-10-02): zero future skew is a valid strict policy; negative future skew is invalid. The standalone helper and tests enforce this; centralized configuration validation and telemetry-path wiring remain pending.
- Confirmed application scope (2026-10-02): apply the same minimum-date and future-skew policy to both received telemetry and the internal publishing method. Validate at each operation's processing time; do not give one path a weaker policy.
- Confirmed placement (2026-10-03): keep `parseTelemetry` pure and independent of the clock. Perform incoming timestamp validation in the MQTT service after parsing, then reuse the same policy in the internal publisher.
- Confirmed minimum-date policy (2026-09-27): device telemetry accepts 2020-01-01T00:00:00Z and later. The lower boundary is inclusive; the firmware's current plausibility check is slightly stricter (`>` rather than `>=`). No maximum-age ingestion rule should be introduced within the supported date range. Policy implementation and tests remain pending.
- Confirmed configuration choice (2026-10-03): expose the minimum accepted measurement date as an explicit typed MQTT setting, later loaded from the centralized YAML configuration. Validate that it is present at startup; the initial configured value will be 2020-01-01T00:00:00Z.
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
