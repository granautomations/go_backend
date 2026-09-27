# ESP32 telemetry timestamp handoff

Prepared: 2026-09-26. Target repository: `/Users/roylopez/Documents/Projects/PlatformIO/Projects/indoor_automation`.

## Status and scope

Initial inspection only; at preparation time no firmware was changed and no firmware build or native tests were run. The initially inspected worktree contained an unrelated `.obsidian/workspace.json` modification; preserve unrelated work.

## Implementation review — 2026-09-26

The separately implemented firmware now captures measurement epochs in `EnvironmentService`, exposes a timestamped application snapshot, and uses it in both publishers. Topic/payload shape, retention, and clock gating are preserved. No firmware source was modified during this review.

- ESP32 build passed: `pio run -e 4d_systems_esp32s3_gen4_r8n16`.
- Native test command initially failed before execution because the selected macOS SDK was incompatible with the linker. A per-command Xcode selection succeeded: `DEVELOPER_DIR=/Applications/Xcode-26.app/Contents/Developer SDKROOT=/Applications/Xcode-26.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk /Users/roylopez/.platformio/penv/bin/pio test -e native`. All 32 tests passed; existing objects emitted deployment-version warnings.
- `git diff --check` passed in the firmware repository.
- User confirmed no device verification yet; no upload was performed.
- Acceptance gaps: add explicit clock-advance-after-acquisition and shared-snapshot tests; humidity interval/failure recovery coverage; failed-read invalidation after a previously valid snapshot; and finite-value serialization overflow coverage. Existing tests do not demonstrate every acceptance condition.
- Timestamp width limitation: `TimeService` widens an `unsigned long` epoch from `TimeAdapter` to `uint64_t`. On the ESP32 this does not eliminate the existing 32-bit epoch limitation; a future full-width guarantee requires updating the adapter boundary too.
- Documentation qualification: independent publisher retries can cause the retained temperature and humidity values to come from different acquisitions. Timestamp equality is guaranteed only when they use the same snapshot, not for every pair of latest retained messages.

The earlier statement that telemetry is absent does not match the current source. `WifiManager.cpp` already creates temperature/humidity publishers when `MQTT_BROKER_HOST` is defined. `handleWifi()` calls them only when `timeService.isSynchronized()` is true. Both publishers reject timestamp zero, publish retained messages, and use `home/indoor/<MQTT_CLIENT_ID>/<metric>`.

The concrete gap is that `handleWifi()` supplies the current epoch to publishers while their providers return cached sensor values. `EnvironmentReading.timestampMs` records acquisition uptime, but no measurement epoch is stored. `main.cpp` invokes `handleWifi()` before its periodic sensor update, so publication can use an earlier sample with a newer timestamp.

Objective: make payload timestamp mean measurement time, never invent an epoch for a sample captured before clock validity, and preserve non-blocking telemetry gating. Do not add a blocking NTP wait or rewrite networking.

## Required workflow

Read the target repository's `AGENTS.md` and relevant `ARCHITECTURE.md` sections first. Follow its ports-and-adapters boundaries, test conventions, and documentation requirements. If its planner/coder workflow requires a target-repository task plan, create it in `.agents/plans/` using this handoff as input. Do not copy the backend's `docs/plans/` convention into the firmware repository.

Reinspect the current files before coding. Return to planning if the implementation has materially changed. Never read, print, commit, or alter real credentials in `include/Secrets.h`; use safe templates when documentation needs configuration examples.

## Implementation steps

1. **Introduce a narrow clock port.** Add `src/application/ports/EpochClock.h`, independent of Arduino and ESP32 headers, with `bool tryEpochSeconds(uint64_t& out) const`. False means no usable measurement epoch. Make `TimeService` implement it by checking the existing time adapter's validity and obtaining its epoch. Document that the current adapter's post-2020 check establishes plausibility, not proof of recent NTP synchronization. Preserve existing time/UI methods; do not silently claim stronger synchronization guarantees.

2. **Timestamp sensor acquisition.** Extend `src/domain/EnvironmentModels.h` with an explicit epoch-seconds field and timestamp-validity flag; retain `timestampMs` for existing uptime uses. Inject `EpochClock&` into `EnvironmentService`. After a successful sensor read, obtain and store measurement epoch once. If clock validity is unavailable, keep local sensor/UI data valid but mark its epoch invalid. Never assign a fresh epoch to that cached sample later. A failed sensor read must invalidate telemetry eligibility. Document that timestamp represents acquisition completion, not publication.

3. **Expose a timestamped snapshot through an application port.** Add `src/application/ports/IndoorTelemetryProvider.h` with a method such as `bool tryIndoorTelemetry(EnvironmentReading& out) const`. Implement it in `EnvironmentService`; return a copy of the cached snapshot only when sensor data and measurement timestamp are valid. Keep existing temperature/humidity UI-provider interfaces intact. MQTT must receive the snapshot directly from the application service, not depend on `SampleDashboardDataProvider` for sensor telemetry.

4. **Use measurement timestamps in both publishers.** Update `MqttTemperaturePublisher` and `MqttHumidityPublisher` to consume the snapshot port. Change their `handle` methods to take scheduling uptime only, and encode the snapshot's stored epoch rather than the current wall clock. Preserve temperature Celsius, humidity percent, two-decimal payload values, the existing topic paths, retained flag, configured interval, and current publish-failure accounting. Reject non-finite values and check serialization/truncation failure. Use safe integer formatting for the selected epoch type; avoid narrowing to a 32-bit timestamp. Do not introduce an offline queue or promise exactly-once delivery.

5. **Wire the dependencies.** In `main.cpp`, inject `getTimeService()` into `EnvironmentService` and pass that service as the telemetry provider. Adapt the `setupWifi` signature in `WifiManager.h/.cpp` to accept the snapshot provider while preserving the dashboard dependency. Construct both publishers with it. Keep the existing `isSynchronized()` gate in `handleWifi()` and remove the current-epoch argument to publishers. A cached sample captured before synchronization must remain ineligible until a new valid acquisition occurs. No blocking wait, new delay, or hardware access belongs in application services.

6. **Add native tests with fake ports.** Update existing environment, time-service, temperature-publisher, and humidity-publisher tests. Add new source files to the native `build_src_filter` only if required. Cover:
   - Clock unavailable: sensor/UI data remains usable, telemetry snapshot is unavailable, and neither metric publishes.
   - Clock becomes valid: an old untimestamped sample stays ineligible; the next successful acquisition carries the measurement epoch and publishes.
   - Clock advances between acquisition and publication: both payload timestamps remain the captured epoch, not the later time.
   - Invalid sensor read/non-finite values: no telemetry publication.
   - Configured interval, failure/retry bookkeeping, retained flags, topic paths, and zero-timestamp rejection remain correct.
   - Temperature and humidity use the same snapshot timestamp when sourced from the same acquisition.
   - Time synchronization can remain pending across multiple loop iterations without preventing normal processing. Do not make fake clock configuration automatically imply synchronization in tests intended to cover this case.

7. **Update firmware documentation.** Update `ARCHITECTURE.md`'s dependency graph and MQTT contract: direct application snapshot provider, measurement-time timestamps, pre-sync behavior, current plausibility-based clock validity, retained latest-value semantics, and no offline-history guarantee. Correct stale provider/service names where affected. Update `README.md` for manual verification steps and add concise code contract comments. Do not change credentials or add a MQTT library migration.

8. **Verify before handoff.** Run from the firmware repository:

   ```sh
   /Users/roylopez/.platformio/penv/bin/pio test -e native
   /Users/roylopez/.platformio/penv/bin/pio run -e 4d_systems_esp32s3_gen4_r8n16
   git diff --check
   ```

   Report exact outcomes, changed files, and unrun checks. Firmware upload/flash requires separate user authorization. After an authorized upload, manually verify no telemetry while clock validity is unavailable, both metrics after a newly timestamped acquisition, and preserved measurement timestamps across a delay. Account for retained broker messages: receiving an old retained value is not proof that the device published before clock validity.

## Acceptance and non-goals

- Existing topic/payload structure remains compatible with the Go backend: numeric value and integer UTC Unix seconds, no unit field.
- Each outgoing timestamp comes from the measurement snapshot, not send time.
- Clock-unavailable samples never become timestamped retroactively.
- Native tests and device build pass; actual device checks are reported separately.
- Do not add historical buffering, change QoS/retention, implement actuators, replace the MQTT library, or invent a new NTP accuracy guarantee in this task. Backend historical/freshness policy remains a separate decision.
