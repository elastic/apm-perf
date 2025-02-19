// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	_ "golang.org/x/tools/cmd/goimports"   // go.mod
	_ "honnef.co/go/tools/cmd/staticcheck" // go.mod

	_ "go.elastic.co/go-licence-detector" // go.mod

	_ "github.com/elastic/go-licenser" // go.mod
)
