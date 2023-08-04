# apm-perf
`apm-perf` contains server performance testing tools.

## apmsoak
Soak testing involves testing apm-server against a continuous, sustained workload to identify performance and stability issues that occur over an extended period. apmsoak command can be used for both classic APM Server and managed APM service(in development) for the purpose of soak testing:

```sh
❯ make build
❯ ./dist/apmsoak run --help
Run apmsoak

Usage:
  apmsoak run [flags]

Flags:
      --api-keys project_id_1:my_api_key,project_id_2:my_key   API keys for managed service. Specify key value pairs as project_id_1:my_api_key,project_id_2:my_key
      --bypass-proxy                                           Detach from proxy dependency and provide projectID via header. Useful when testing locally
  -f, --file string                                            Path to scenarios file (default "./scenarios.yml")
  -h, --help                                                   help for run
      --scenario string                                        Specify which scenario to use. the value should match one of the scenario key defined in given scenarios YAML file (default "steady")
      --secret-token string                                    Secret token for APM Server. Managed intake service doesn't support secret token
      --server-url string                                      Server URL (default http://127.0.0.1:8200), if specified <project_id>, it will be replaced with the project_id provided by the config, (example: https://<project_id>.apm.elastic.cloud)
```

All the flags are optional, but it's expected to provide a scenario flag to start a specific scenario from [pre-defined scenarios file](https://github.com/elastic/apm-perf/blob/main/cmd/apmsoak/scenarios.yml#L2), it is `steady` by default.
```sh
# start soaktest that generates 2000 events per second
./apmsoak run --scenario=steady
```

The configs in `scenarios.yml` file inherits [loadgen](./internal/loadgen/config/config.go) configs, with extra fields such as project_id and api_key that can be used for the managed APM service.

Note that the managed service will use API key based auth and one soaktest can target multiple projects, so we provide key-value pairs(projectID and api key respectively). the api key can also be provided as an [ENV variable](https://github.com/elastic/apm-perf/blob/main/cmd/apmsoak/run.go#L20).

```sh
./apmsoak run --scenario=fairness --api-keys="project_1:key-to-project_1,project_2:key-to-project_2"
```
