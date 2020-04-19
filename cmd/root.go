//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joeshaw/multierror"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nwdevice2filebeat",
	Short: "Converts RSA NetWitness device log parsers to Filebeat modules",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

func LogError(msg string, keysAndValues ...interface{}) {
	var sb strings.Builder
	sb.WriteString("Error: ")
	sb.WriteString(msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		sb.WriteString(fmt.Sprintf(" %s=%v", keysAndValues[i], limitError(keysAndValues[i+1])))
	}
	if len(keysAndValues)&1 != 0 {
		sb.WriteString(fmt.Sprintf(" %s=%v", "_unmatched_", limitError(keysAndValues[len(keysAndValues)-1])))
	}
	log.Println(sb.String())
}

const MaxPrintErrors = 10

func limitError(val interface{}) interface{} {
	// TODO: This doesn't work
	if err, ok := val.(error); ok {
		var merr *multierror.MultiError
		if errors.As(err, &merr) {
			if n := len(merr.Errors); n > MaxPrintErrors {
				merr.Errors = append(merr.Errors[:MaxPrintErrors],
					errors.New(fmt.Sprintf("... and %d more.", n-MaxPrintErrors)))
			}
		}
	}
	return val
}
