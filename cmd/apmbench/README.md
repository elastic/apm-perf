# APMBench

APMBench allows running a set of benchmarks against APM supported endpoints. It
also allows collection of OTEL metrics from the servers implementing APM endpoints
and report it as benchmarking results. This is possible by using otel collector
with a custom [in-memory exporter](../otelinmemexporter).
