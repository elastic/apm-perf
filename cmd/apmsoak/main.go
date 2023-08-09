// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const envVarPrefix = "ELASTIC_APM_"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Register root command in cobra
	var rootCmd = &cobra.Command{
		Use:              "apmsoak",
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				optionName := strings.ToUpper(flag.Name)
				optionName = strings.ReplaceAll(optionName, "-", "_")
				envVar := envVarPrefix + optionName
				if val, ok := os.LookupEnv(envVar); !flag.Changed && ok {
					flagErr := flag.Value.Set(val)
					if flagErr != nil {
						err = fmt.Errorf("invalid environment variable %s: %w", envVar, flagErr)
					}
				}
			})
			return err
		},
	}
	rootCmd.AddCommand(NewCmdVersion())
	rootCmd.AddCommand(NewCmdRun())

	// Execute commands
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Println(err)
		}
	}
}
