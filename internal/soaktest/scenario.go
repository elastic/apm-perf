// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package soaktest

type Scenarios struct {
	Scenarios map[string][]ScenarioConfig `yaml:"scenarios"`
}

// ScenarioConfig contains scenario specific configuration parameters.
// All the field in this struct come from user input through the scenario
// config file.
type ScenarioConfig struct {
	// ProjectID is a project alpha numeric ID, assigned to each generated data
	// point.
	ProjectID string `yaml:"project_id"`
	// ServerURL is the Elasticsearch server URL where data from the scenario
	// is pushed to.
	ServerURL string `yaml:"server"`
	// APIKey is a API key used to push data to the specified server.
	// When is an empty string its value is taken from RunnerConfig.APIKeys
	// (selecting the appropriate one by ProjectID).
	APIKey string `yaml:"api_key"`
	// AgentName is a string used to specify which events file to source.
	// For Elastic APM agents this value must be the agent name (es go,
	// ruby, nodejs).
	// For OTLP agents this value bust be <otlp-traces|metrics|logs>, as
	// the OTLP protocol uses different endpoints for different data types.
	AgentName string `yaml:"agent_name"`
	// AgentReplicas is the number of different agents that this scenario
	// will emulate when generating data points.
	AgentReplicas int `yaml:"agent_replicas"`
	// EventRate represent the rate of events generated.
	// Must be in the format <number of events>/<time>. <time> is parsed
	// as a Go time.Duration value.
	// FIXME: improve comment
	EventRate string `yaml:"event_rate"`
	// Headers contains additional HTTP headers to attach to every data
	// request sent as part of the scenario.
	Headers map[string]string `yaml:"headers"`
	// V7 specifies if the sample data to be used for this scenario should
	// be compatible with a 7.x APM Server.
	V7 bool `yaml:"v7"`

	RewriteIDs                bool `yaml:"rewrite_ids"`
	RewriteTimestamps         bool `yaml:"rewrite_timestamps"`
	RewriteServiceNames       bool `yaml:"rewrite_service_names"`
	RewriteServiceNodeNames   bool `yaml:"rewrite_service_node_names"`
	RewriteServiceTargetNames bool `yaml:"rewrite_service_target_names"`
	RewriteSpanNames          bool `yaml:"rewrite_span_names"`
	RewriteTransactionNames   bool `yaml:"rewrite_transaction_names"`
	RewriteTransactionTypes   bool `yaml:"rewrite_transaction_types"`
}
