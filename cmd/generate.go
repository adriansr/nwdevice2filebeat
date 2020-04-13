//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"log"

	"github.com/adriansr/nwdevice2filebeat/generator"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new Filebeat fileset from a NetWitness device",
	Run: func(cmd *cobra.Command, args []string) {
		devicePath, err := cmd.PersistentFlags().GetString("device")
		if err != nil {
			log.Panic(err)
		}
		dev, err := model.NewDevice(devicePath)
		if err != nil {
			LogError("Failed to load device", "path", devicePath, "reason", err)
			return
		}
		log.Printf("Loaded XML %s", dev.String())
		_, err = generator.New(dev)
		if err != nil {
			LogError("Failed to parse device", "path", devicePath, "reason", err)
		}
	},
}

func init() {
	generateCmd.PersistentFlags().String("device", "", "Input device path")
	generateCmd.MarkPersistentFlagRequired("device")
}
