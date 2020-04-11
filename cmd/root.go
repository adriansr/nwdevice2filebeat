//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

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
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	//rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	//rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")

	rootCmd.AddCommand(generateCmd)
}

func LogError(msg string, keysAndValues ...interface{}) {
	var sb strings.Builder
	sb.WriteString("Error: ")
	sb.WriteString(msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		sb.WriteString(fmt.Sprintf(" %s=%v", keysAndValues[i], keysAndValues[i+1]))
	}
	if len(keysAndValues)&1 != 0 {
		sb.WriteString(fmt.Sprintf(" %s=%v", "_unmatched_", keysAndValues[len(keysAndValues)-1]))
	}
	log.Println(sb.String())
}
