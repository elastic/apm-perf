// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/elastic/apm-perf/internal/version"
)

func NewCmdVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show current version info",
		Run: func(cmd *cobra.Command, args []string) {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%s %s", version.CommitSha(), version.BuildTime())
			fmt.Fprintf(cmd.OutOrStdout(),
				"apmsoak version %s (%s/%s) [%s]\n",
				version.Version, runtime.GOOS, runtime.GOARCH,
				buf.String(),
			)
		},
	}
	return cmd
}
