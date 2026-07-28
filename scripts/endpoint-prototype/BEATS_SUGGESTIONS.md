# Beats repository changes for further binary size reduction

This document lists modifications that would reduce the size of the
`elastic-otel-collector` binary further when beats are imported only as
OTel receivers (filebeatreceiver, metricbeatreceiver, osqreceiver) for
an endpoint security use case.

**These are suggestions only — they require changes in the `elastic/beats` repository
and are NOT implemented in this prototype.**

---

## Background: what the receivers currently compile in

### fbreceiver dependency chain

`x-pack/filebeat/fbreceiver/factory.go` imports:

- `filebeat/beater` — which **unconditionally** imports `filebeat/autodiscover`
- `filebeat/autodiscover/defaults.go` — imports the kubernetes and docker autodiscover
  providers regardless of build tags; these pull in `k8s.io/client-go` and `moby/moby`
- `x-pack/filebeat/input/default-inputs` — registers every filebeat input type
  including AWS (s3, cloudwatch), Azure (blobstorage, eventhub), GCP (gcs, pubsub),
  CEL, salesforce, o365audit, entity analytics, lumberjack, netflow, httpjson, …
- `x-pack/filebeat/include` — registers ~30 x-pack modules via `init()`

For endpoint security, only `filestream`, `log`, and `journald` (Linux) are needed.

### mbreceiver dependency chain

`x-pack/metricbeat/mbreceiver/factory.go` imports:

- `metricbeat/include/list_docker.go` — registers kubernetes (22 metricsets),
  docker, and system modules. Kubernetes metricsets pull in the full
  `k8s.io/client-go` tree.
- `x-pack/metricbeat/include/list.go` — registers AWS, Azure, GCP, containerd,
  istio, Prometheus, and other x-pack modules.

For endpoint security, only the `system` module (cpu, memory, process,
filesystem, network) is needed.

### Estimated SDK sizes compiled into the current binary

These are not exact but give the order of magnitude for each dependency:

| Dependency tree | Root cause | Estimated binary contribution |
|---|---|---|
| `k8s.io/client-go` + `k8s.io/api` | `fbreceiver` autodiscover + `mbreceiver` kubernetes modules | **>25 MB** |
| AWS SDK v2 (s3, sqs, ec2, cloudwatch, costexplorer, rds, iam, orgs, health, sts…) | `fbreceiver` aws inputs + `mbreceiver` aws modules | **>25 MB** |
| Azure SDK (azcore, azidentity, armmonitor, armresources, armconsumption…) | `fbreceiver` azure inputs + `mbreceiver` azure modules | **>15 MB** |
| GCP SDK (storage, pubsub, bigquery, monitoring, redis…) | `fbreceiver` gcs/gcppubsub inputs + `mbreceiver` gcp modules | **>15 MB** |
| `github.com/moby/moby/client` | `fbreceiver` docker autodiscover + `mbreceiver` docker module | **~5–10 MB** |
| CEL, CloudFoundry, Salesforce, o365audit, lumberjack, netflow, httpjson… | Various x-pack filebeat inputs | **~5–10 MB** |

**Rough total avoidable: 90–100 MB** from the `elastic-otel-collector` binary
(currently ~236 MB on darwin-arm64). Combined with input/module registration
changes below, the binary could plausibly reach ~100–130 MB.

---

## Suggested changes (priority order)

### 1. Make `filebeat/autodiscover` opt-in  ← HIGHEST VALUE

`filebeat/beater/filebeat.go` unconditionally imports `filebeat/autodiscover`:

```go
// filebeat/beater/filebeat.go (current)
_ "github.com/elastic/beats/v7/filebeat/autodiscover"
```

`filebeat/autodiscover/defaults.go` registers the kubernetes and docker providers
which pull in `k8s.io/client-go`, `k8s.io/api`, and `moby/moby/client` for every
binary that uses `fbreceiver`, whether or not autodiscovery is configured.

**Suggestion:** move the autodiscover import out of `beater` and into a separate
`beater_full.go` file guarded by a build tag (e.g. `!slim_inputs`):

```go
// filebeat/beater/beater_autodiscover.go  (new, build tag !slim_inputs)
//go:build !slim_inputs

import _ "github.com/elastic/beats/v7/filebeat/autodiscover"
```

The receiver factory would use `slim_inputs` so k8s/docker providers are
never compiled in. **Estimated saving: >25 MB**

### 2. Build-tag-controlled input registration

Add a build tag (e.g. `slim_inputs`) so that only a curated subset of inputs
is compiled into the receiver binary.

**Example for `fbreceiver`:**

```go
// x-pack/filebeat/input/default-inputs/inputs_endpoint.go  (new)
//go:build slim_inputs

// Only register inputs needed for endpoint security telemetry.
import (
    _ "github.com/elastic/beats/v7/filebeat/input/filestream"
    _ "github.com/elastic/beats/v7/filebeat/input/log"
)
// journald registered separately on Linux via existing build tag
```

The existing `inputs_other.go` (which registers all cloud/SaaS inputs) would
gain a `//go:build !slim_inputs` guard.

`endpoint-inputs` would include only: `filestream`, `log`, `journald`.
It would exclude: AWS S3, Azure EventHub, GCS, CEL, Kafka, CEF, Syslog, HTTPJSON,
  CloudFoundry, entity analytics, salesforce, o365audit, lumberjack, netflow,
  streaming, and the other ~25 inputs irrelevant for endpoint security.

**Estimated saving: 30–40 MB** (when combined with suggestion #1)

### 3. Slim metricbeat module registration

`metricbeat/include/list_docker.go` registers kubernetes (22 metricsets), docker,
and system modules in a single file. For endpoint security only `system/*` is needed.

**Suggestion:** introduce a `slim_modules` build tag variant:

```go
// metricbeat/include/list_endpoint.go  (new)
//go:build slim_modules

import (
    _ "github.com/elastic/beats/v7/metricbeat/module/system"
)
```

The existing `list_docker.go` would gain `//go:build !slim_modules`.
The same pattern applies to `x-pack/metricbeat/include/list.go` (AWS, Azure, GCP
modules).

**Estimated saving: ~20–30 MB** (eliminates k8s.io/client-go from mbreceiver side
and the AWS/Azure/GCP metricbeat module SDKs)

### 4. Remove `cmd` package import from receivers

Each receiver's factory imports the beat's `cmd` package to construct settings:

```go
// x-pack/filebeat/fbreceiver/factory.go (current)
import filebeatcmd "github.com/elastic/beats/v7/x-pack/filebeat/cmd"
...
settings := filebeatcmd.FilebeatSettings(Name)
```

This pulls in Cobra CLI infrastructure and flag definitions not needed at runtime.

**Suggestion:** extract settings construction into a separate function in `libbeat`
that does not require importing the `cmd` package:

```go
// libbeat/otelreceiver/settings.go  (new)
func ReceiverSettings(name string) instance.Settings { ... }
```

**Estimated saving: 5–15 MB**

### 5. Consolidate `osquery-extension.ext` into osqreceiver

`osqreceiver` ships a separately-bundled `osquery-extension.ext` binary alongside
`osqueryd`. For endpoint builds these could potentially be merged or linked
statically, avoiding a separate file.
(More invasive; may conflict with osquery's update cycle.)

---

## Expected total savings

| Change | Estimated saving |
|--------|-----------------|
| Slim autodiscover — suggestion #1 | >25 MiB |
| Slim fbreceiver inputs — suggestion #2 | 30–40 MiB |
| Slim mbreceiver modules — suggestion #3 | 20–30 MiB |
| Remove cmd import from receivers — suggestion #4 | 5–15 MiB |
| **Total** | **~80–110 MiB** |

Starting from ~236 MiB (darwin-arm64 in this prototype), these changes could
bring `elastic-otel-collector` to roughly **~130–160 MiB**.

To validate, measure the binary after applying each change with:

```bash
go tool nm <binary> | awk '{print $3}' | grep -E 'elastic/beats|x-pack|k8s.io|aws-sdk|azure|google.golang.org/cloud' | \
  wc -l   # count symbols remaining from beats and cloud SDKs
```
