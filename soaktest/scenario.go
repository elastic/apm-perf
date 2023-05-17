// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package soaktest

type Scenarios struct {
	Scenarios map[string][]ScenarioConfig `yaml:"scenarios"`
}

type ScenarioConfig struct {
	ProjectID                 string            `yaml:"project_id"`
	ServerURL                 string            `yaml:"server"`
	APIKey                    string            `yaml:"api_key"`
	AgentName                 string            `yaml:"agent_name"`
	AgentsReplicas            int               `yaml:"agent_replicas"`
	EventRate                 string            `yaml:"event_rate"`
	RewriteIDs                bool              `yaml:"rewrite_ids"`
	RewriteTimestamps         bool              `yaml:"rewrite_timestamps"`
	RewriteServiceNames       bool              `yaml:"rewrite_service_names"`
	RewriteServiceNodeNames   bool              `yaml:"rewrite_service_node_names"`
	RewriteServiceTargetNames bool              `yaml:"rewrite_service_target_names"`
	RewriteSpanNames          bool              `yaml:"rewrite_span_names"`
	RewriteTransactionNames   bool              `yaml:"rewrite_transaction_names"`
	RewriteTransactionTypes   bool              `yaml:"rewrite_transaction_types"`
	Headers                   map[string]string `yaml:"headers"`
}
