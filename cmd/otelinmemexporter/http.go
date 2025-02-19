// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package otelinmemexporter contains code for creating an in-memory OTEL exporter.
package otelinmemexporter

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type telemetryStore interface {
	GetAll() map[string]map[string]float64
	Get(string) (map[string]float64, error)
	Reset()
}

type server struct {
	store    telemetryStore
	endpoint string
	logger   *zap.Logger
}

func newServer(store telemetryStore, endpoint string, logger *zap.Logger) *server {
	return &server{
		store:    store,
		endpoint: endpoint,
		logger:   logger,
	}
}

func (s *server) Start() {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(s.store.GetAll()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Warn("failed to encode response", zap.Error(err))
		}
	}).Methods(http.MethodGet)

	r.HandleFunc("/{key:.*}", func(w http.ResponseWriter, r *http.Request) {
		key := mux.Vars(r)["key"]
		enc := json.NewEncoder(w)
		val, err := s.store.Get(key)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if err := enc.Encode(map[string]string{"err": err.Error()}); err != nil {
				s.logger.Warn("failed to encode response", zap.Error(err))
			}
			return
		}
		if err := enc.Encode(map[string]map[string]float64{key: val}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Warn("failed to encode response", zap.Error(err))
		}
	}).Methods(http.MethodGet)

	r.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		s.store.Reset()
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodPost)

	go func() {
		s.logger.Info("Starting http server for serving in-memory exporter API", zap.String("endpoint", s.endpoint))
		if err := http.ListenAndServe(s.endpoint, r); err != nil {
			s.logger.Error("failed to start server for in memory exporter", zap.Error(err))
		}
	}()
}
