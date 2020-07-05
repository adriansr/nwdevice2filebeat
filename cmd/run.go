//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/runtime"
	"github.com/adriansr/nwdevice2filebeat/util"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs a runtime parser against a log file (for testing)",
	Run: func(cmd *cobra.Command, args []string) {
		terminateOnError(doRun(cmd, args))
	},
}

func init() {
	runCmd.PersistentFlags().String("logs", "l", "Input logs file path")
	runCmd.PersistentFlags().String("device", "", "Input device path")
	runCmd.PersistentFlags().String("output", "", "TODO")
	runCmd.PersistentFlags().String("tz", "", "Timezone")
	runCmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
	runCmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
	runCmd.PersistentFlags().CountP("verbose", "v", "Verbosity level, can be repeated.")
	runCmd.MarkPersistentFlagRequired("device")
	runCmd.MarkPersistentFlagRequired("logs")
	rootCmd.AddCommand(runCmd)
}

func doRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.NewFromCommand(cmd)
	if err != nil {
		LogError("Failed to parse configuration", "reason", err)
		return err
	}
	logPath, err := cmd.PersistentFlags().GetString("logs")
	if err != nil {
		LogError("Failed to parse log configuration", "reason", err)
		return err
	}
	inputFile, err := os.Open(logPath)
	if err != nil {
		LogError("Failed to open logs file", "path", logPath, "reason", err)
		return err
	}
	defer inputFile.Close()

	warnings := util.NewWarnings(20)
	dev, err := model.NewDevice(cfg.DevicePath, &warnings)
	if err != nil {
		LogError("Failed to load device", "path", cfg.DevicePath, "reason", err)
		return err
	}

	if !warnings.Print("loading XML device") {
		log.Printf("Loaded XML %s", dev.String())
	}
	warnings.Clear()

	p, err := parser.New(dev, cfg, &warnings)
	if err != nil {
		LogError("Failed to parse device", "path", cfg.DevicePath, "reason", err)
		return err
	}
	warnings.Print("parsing device")
	warnings.Clear()

	var logger util.StdLog
	rt, err := runtime.New(&p, &warnings, logger)
	if err != nil {
		LogError("Failed to load runtime", "reason", err)
		return err
	}
	scanner := bufio.NewScanner(inputFile)
	start := time.Now()
	var count int
	for count = 0; scanner.Scan(); count++ {
		line := scanner.Bytes()
		fields, errs := rt.Process(line)
		log.Printf("Processed line #%d <<%s>>", count, line)
		log.Printf("Got %d fields:", len(fields))
		for k, v := range fields {
			log.Printf("  '%s': '%s'", k, v)
		}
		if len(errs) > 0 {
			log.Printf("Got %d errors:", len(errs))
			for idx, err := range errs {
				log.Printf("  err[%d] = %v", idx, err)
			}

		}
	}
	took := time.Now().Sub(start)
	log.Printf("Processed %d lines in %v (%.0f eps)",
		count, took, float64(count)/took.Seconds())
	return nil
}
