# Changelog

Meaningful implementation, testing, configuration, and documentation changes are recorded here. Unreleased entries are grouped by date, category, and feature; released changes will be moved into dated version sections when releases are made. Dates use `YYYY-MM-DD` in the project's local timezone.

This initial baseline records the current MQTT work and project guidance as of 2026-09-26. Its date is the baseline recording date, not the original implementation date of every item. It is not a reconstructed history of earlier commits.

## Unreleased

### 2026-10-02

#### Added

- Added a timestamp-validation helper with deterministic tests for the inclusive 2020 minimum, historical readings, the future-skew boundary, and zero/negative skew. The helper is not yet wired into telemetry processing.

#### Changed

- Refreshed the MQTT plan against the current backend code and test results, clarifying the remaining timestamp, publishing, configuration, and server-lifecycle work.
- Documented the decision to apply the same timestamp policy to received telemetry and internal publishing; neither path is wired to the helper yet.
- Documented telemetry test functions and the timestamp-validation branches. Clarified that zero future skew is valid and negative skew is rejected; centralized configuration wiring is still pending.

### 2026-09-27

#### Changed

- Refreshed the MQTT plan's resume point and verification evidence, distinguishing completed encoding tests from pending timestamp validation and internal publishing.
- Defined the backend ingestion policy to accept valid delayed/historical telemetry, preserve measurement time, and keep freshness separate from validity. Timestamp validation implementation remains pending.
- Defined a configurable five-minute future-timestamp tolerance with an inclusive boundary; validation implementation and configuration wiring remain pending.
- Defined 2020-01-01T00:00:00Z as the inclusive minimum date for device telemetry; validation implementation remains pending.

### 2026-09-26

#### Added

- Internal MQTT service using Eclipse Paho with configurable broker/client settings, persistent-session configuration, automatic reconnection, subscription acknowledgment checks, and readiness tracking.
- Up to two immediate subscription attempts, with unit coverage for recovery and exhausted retries. Ongoing recovery after exhausted attempts remains unimplemented.
- Telemetry topic parsing, JSON decoding, metric-unit lookup, and structured logging of received or rejected readings.
- Publish-topic builder rejecting empty levels and embedded separators or wildcards, with seven table-driven invalid-input cases verifying both errors and empty returned topics.
- Opt-in Mosquitto integration tests for connection readiness and temperature delivery, with isolated identifiers, bounded waits, and structured log assertions. The delivery test currently publishes directly through Paho; the internal publishing method is not implemented yet.
- MQTT implementation plan with progress checklists, verification evidence, unresolved decisions, and a resume point.
- Repository changelog and instructions for maintaining meaningful change records.
- Application telemetry encoder and tests produce and verify exactly the value and integer Unix-seconds timestamp fields, omitting internal metadata. Table-driven error tests verify that NaN and both infinities return errors and nil payloads; documentation describes precision and validation limits.

#### Changed

- Project guidance now requires in-code documentation and keeping implementation plans current during development.
- Planned server configuration now uses centralized, typed YAML settings with startup validation and external secret overrides. The configuration loader and migration of hard-coded runtime values are not implemented yet.
- Collaboration guidance now calls for meaningful questions about intent and implementation tradeoffs, with non-blocking questions for clear requests and clarification before consequential assumptions.
- Changelog entries now include a dated grouping, with an explicit distinction between baseline recording dates and original implementation dates.
- Changelog policy now limits entries to backend-repository changes; removed firmware-only handoff and review entries.
- Documented topic parsing/building and telemetry decoding/parsing contracts, validation scope, and error results; runtime behavior is unchanged.
