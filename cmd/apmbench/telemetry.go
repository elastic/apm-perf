// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type telemetry struct {
	endpoint string
}

func (t telemetry) GetAll() (map[string]float64, error) {
	resp, err := http.Get(t.endpoint + "/")
	if err != nil {
		return nil, fmt.Errorf("failed to get telemetry data: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 2:
		m := make(map[string]float64)
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			return nil, fmt.Errorf("failed to decode response body for getting telemetry data: %w", err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unsuccessful response from benchmark telemetry server: %d", resp.StatusCode)
	}
}

func (t telemetry) Reset() error {
	resp, err := http.Post(t.endpoint+"/reset", "application/json", nil)
	if err != nil {
		return errors.New("failed to reset telemetry")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to reset telemetry")
	}
	return nil
}
