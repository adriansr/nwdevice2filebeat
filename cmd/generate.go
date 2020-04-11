//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"log"

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
			LogError("Failed to load device", "path", devicePath, "error", err)
			return
		}
		log.Print("Loading device from XML=", dev.XMLPath)
	},
}

//var devicePath string

func init() {
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	//rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	//rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")

	//generateCmd.AddCommand()
	//generateCmd.PersistentFlags().StringVar(&devicePath, "moduledir", "module", "Path to destination module dir")
	generateCmd.PersistentFlags().String("device", "", "Input device path")
	generateCmd.MarkPersistentFlagRequired("device")
	//generateCmd.PersistentFlags().StringVar(&devicePath, "fileset", "", "Generated fileset name")
}
