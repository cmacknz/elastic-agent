# Beats repository changes for further binary size reduction

This document lists modifications that would reduce the size of the
`elastic-otel-collector` binary further when beats are imported only as
OTel receivers (filebeatreceiver, metricbeatreceiver, abreceiver, osqreceiver)
rather than as standalone command-line tools.

**These are suggestions only — they require changes in the `elastic/beats` repository
and are NOT implemented in this prototype.**

---

## Context

Even with beat subcommands removed from `elastic-otel-collector` and all
non-endpoint OTel components stripped, the binary remains large because the
beat *receiver* factories still pull in the full beat framework:

- `x-pack/filebeat/fbreceiver/factory.go` imports:
  - `filebeat/cmd` — CLI parsing and settings infrastructure
  - `x-pack/filebeat/input/default-inputs` — registers **every** filebeat input
    type (hundreds of inputs including cloud-provider integrations, network
    protocol parsers, SaaS API inputs, etc.)
  - `x-pack/filebeat/include` — initialises all x-pack modules

- `x-pack/metricbeat/mbreceiver/factory.go` has the same pattern via
  `x-pack/metricbeat/include` and `x-pack/metricbeat/module`.

The majority of this code is dead for an endpoint security use case, but
Go's linker cannot eliminate it because it is registered at `init()` time
through lookup tables.

---

## Suggested changes

### 1. Build-tag-controlled input registration in receivers

Add a build tag (e.g. `endpoint` or `slim_inputs`) so that only a curated
subset of inputs is compiled into the receiver binary.

**Example for `fbreceiver`:**

```go
// x-pack/filebeat/fbreceiver/factory.go  (current)
inputs "github.com/elastic/beats/v7/x-pack/filebeat/input/default-inputs"
...
beatCreator := beater.New(inputs.Init)
```

```go
// x-pack/filebeat/fbreceiver/factory_endpoint.go  (new, build tag endpoint)
//go:build endpoint

// For endpoint builds, only register the inputs needed for security telemetry.
inputs "github.com/elastic/beats/v7/x-pack/filebeat/input/endpoint-inputs"
```

`endpoint-inputs` would include only: `filestream`, `log`, `journald`, `stdin`.
It would exclude: AWS S3, Azure EventHub, Kafka, CEF, Syslog, HTTPJSON,
  and the other ~150 inputs that are irrelevant for endpoint security.

### 2. Remove `cmd` package import from receivers

Each receiver's factory.go currently imports the beat's `cmd` package to call
`FilebeatSettings(Name)` / `MetricbeatSettings(Name)`. This pulls in Cobra
CLI infrastructure and various flag definitions that are not needed at runtime.

**Suggestion:** Extract the `Settings` struct initialisation into a separate
function in `libbeat` (or the beat's own settings package) that does not
require importing the full `cmd` package. For example:

```go
// libbeat/otelreceiver/settings.go  (new)
// ReceiverSettings returns the minimum BeatSettings needed to run as a receiver.
func ReceiverSettings(name string) instance.Settings { ... }
```

The receiver factories would then import only the new helper rather than the
full CLI command tree.

### 3. Separate x-pack module initialisation from core receiver

`x-pack/filebeat/include` registers all x-pack modules at `init()` time.
For receiver-only builds these modules are wasted binary space.

**Suggestion:** Split `x-pack/filebeat/include` into:
- `include/core` — modules always needed (no external service dependencies)
- `include/cloud` — cloud-provider integrations
- `include/xpack_full` — everything (current behaviour, the default)

The receiver factory would import `include/core` by default and
`include/xpack_full` only when the `full_modules` build tag is set.

### 4. Consolidate `osquery-extension.ext` into osqreceiver

`osqreceiver` currently relies on a separately-shipped `osquery-extension.ext`
binary alongside the `osqueryd` daemon. For an endpoint build, these could
potentially be merged or linked statically, avoiding a separate file.
(This is a more invasive change and may conflict with osquery's update cycle.)

---

## Expected additional savings

Based on the size of the x-pack module registration tables in the beats
submodule:

| Change | Estimated saving |
|--------|-----------------|
| Slim fbreceiver inputs (suggestion 1) | 30–60 MiB |
| Remove cmd import from receivers (suggestion 2) | 5–15 MiB |
| Slim x-pack module init (suggestion 3) | 20–40 MiB |

These are rough estimates based on the number of input/module packages that
would be excluded and their typical size. Actual savings depend on how much
dead code the Go linker can already eliminate from unreachable symbol paths.

To validate, measure the binary after applying each change with:

```bash
go tool nm <binary> | awk '{print $3}' | grep -E 'elastic/beats|x-pack' | \
  wc -l   # count symbols remaining from beats
```
