// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package soaktest

import (
	"errors"
	"flag"
	"os"
	"strings"
)

var SoakConfig struct {
	Scenario      string
	ScenariosPath string
	ServerURL     string
	SecretToken   string
	ApiKeys       map[string]string
	BypassProxy   bool
}

func init() {
	flag.StringVar(&SoakConfig.Scenario, "scenario", "steady", "Specify which scenario to use")
	flag.StringVar(&SoakConfig.ScenariosPath, "f", "./scenarios.yml", "Path to scenarios file")
	flag.StringVar(&SoakConfig.ServerURL, "server", "", "Ingest service URL (default http://127.0.0.1:8200), if specify <project_id>, it will be replaced with the project_id provided by the config, (https://<project_id>.apm.elastic.cloud)")
	flag.StringVar(&SoakConfig.SecretToken, "secret-token", "", "secret token for APM Server. Managed intake service doesn't support secret token")
	flag.Func("api-keys", "API keys by projectID for apm managed service",
		func(s string) error {
			SoakConfig.ApiKeys = make(map[string]string)
			pairs := strings.Split(s, ",")
			for _, pair := range pairs {
				kv := strings.Split(pair, ":")
				if len(kv) != 2 {
					return errors.New("invalid api keys provided. example: project_id:my_api_key")
				}
				SoakConfig.ApiKeys[kv[0]] = kv[1]
			}
			return nil
		})
	flag.BoolVar(&SoakConfig.BypassProxy, "bypass-proxy", false, "Detach from proxy dependency and provide projectID via header. Useful when testing locally")

	// For configs that can be set via environment variables, set the required
	// flags from env if they are not explicitly provided via command line
	setFlagsFromEnv()
}

func setFlagsFromEnv() {
	// value[0] is environment key
	// value[1] is default value
	flagEnvMap := map[string][]string{
		"server":       {"ELASTIC_APM_SERVER_URL", "http://127.0.0.1:8200"},
		"secret-token": {"ELASTIC_APM_SECRET_TOKEN", ""},
		"api-keys":     {"ELASTIC_APM_API_KEYS", ""},
	}

	for k, v := range flagEnvMap {
		flag.Set(k, getEnvOrDefault(v[0], v[1]))
	}
}

func getEnvOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return defaultValue
}
