// Licensed to The OpenTelemetry Authors under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The OpenTelemetry Authors licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package common

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	"google.golang.org/grpc/credentials"
)

// caPool loads CA certificate from a file and returns a CertPool.
// The certPool is used to set RootCAs in certificate verification.
func caPool(caFile string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if caFile != "" {
		data, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("failed to add CA certificate to root CA pool")
		}
	}
	return pool, nil
}

func GetTLSCredentialsForGRPCExporter(caFile string, cAuth ClientAuth) (credentials.TransportCredentials, error) {

	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tls.Config{
		RootCAs: pool,
	})

	// Configuration for mTLS
	if cAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cAuth.ClientCertFile, cAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(&tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{keypair},
		})
	}

	return creds, nil
}

func GetTLSCredentialsForHTTPExporter(caFile string, cAuth ClientAuth) (*tls.Config, error) {
	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}

	tlsCfg := tls.Config{
		RootCAs: pool,
	}

	// Configuration for mTLS
	if cAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cAuth.ClientCertFile, cAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.Certificates = []tls.Certificate{keypair}
	}
	return &tlsCfg, nil
}
