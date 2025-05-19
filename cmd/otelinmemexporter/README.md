# OTEL in-memory exporter

Aggregates collected metrics as per the configured aggregation type and exposes them via a HTTP endpoint.

Three kinds of metrics are supported, each with different aggregations:
1. `pmetric.Sum` (aggregation: `last`, `sum`, `rate`, `max`)
2. `pmetric.Gauge` (aggregation: `last`, `sum`, `rate`, `max`)
3. `pmetric.Histogram` (aggregation: `percentile`, `sum`)

## Usage

The exporter is built to aid collection of metrics for benchmarking using [apmbench](../apmbench). It can be used to build OpenTelemetry collector using [OpenTelemetry Collector Builder (ocb)](https://pkg.go.dev/go.opentelemetry.io/collector/cmd/builder#section-readme). Example configurartion file for the builder:

```yaml
dist:
  module: opentelemetry-collector
  name: otel
  description: Test otel collector with in-memory exporter.
  output_path: ./generated
  otelcol_version: 0.88.0

exporters:
  - gomod: github.com/elastic/apm-perf/cmd/otelinmemexporter v0.0.0-00010101000000-000000000000

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.88.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.88.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.88.0

```

The above configuration file can be used to generate code for OpenTelemetry Collector using `builder --skip-compilation --config=ocb-config.yaml`.

NOTE: The in-memory exporter should be used for a single benchmark at a time to avoid conflicts in collected metrics.
