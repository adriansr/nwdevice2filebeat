//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/runtime"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs a runtime parser against a log file (for testing)",
	Run:   doRun,
}

func init() {
	runCmd.PersistentFlags().String("logs", "l", "Input logs file path")
	runCmd.PersistentFlags().String("device", "", "Input device path")
	runCmd.PersistentFlags().String("output", "", "TODO")
	runCmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
	runCmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
	runCmd.MarkPersistentFlagRequired("device")
	runCmd.MarkPersistentFlagRequired("logs")
}

func doRun(cmd *cobra.Command, args []string) {
	cfg, err := readConf(cmd)
	if err != nil {
		LogError("Failed to parse configuration", "reason", err)
		return
	}
	logPath, err := cmd.PersistentFlags().GetString("logs")
	if err != nil {
		LogError("Failed to parse log configuration", "reason", err)
		return
	}
	inputFile, err := os.Open(logPath)
	if err != nil {
		LogError("Failed to open logs file", "path", logPath, "reason", err)
		return
	}
	defer inputFile.Close()

	dev, err := model.NewDevice(cfg.DevicePath)
	if err != nil {
		LogError("Failed to load device", "path", cfg.DevicePath, "reason", err)
		return
	}
	log.Printf("Loaded XML %s", dev.String())
	p, err := parser.New(dev, cfg)
	if err != nil {
		LogError("Failed to parse device", "path", cfg.DevicePath, "reason", err)
		return
	}
	rt, err := runtime.New(&p)
	if err != nil {
		LogError("Failed to load runtime", "reason", err)
		return
	}
	scanner := bufio.NewScanner(inputFile)
	start := time.Now()
	var count int
	for count = 1; scanner.Scan(); count++ {
		line := scanner.Bytes()
		fields, err := rt.Process(line)
		if err != nil {
			LogError("Error processing logs", "line", count, "reason", err, "message", string(line))
			break
		}
		log.Printf("Processed message <<%s>>", line)
		log.Printf("Got %d fields:", len(fields))
		for k, v := range fields {
			log.Printf("  '%s': '%s'", k, v)
		}
	}
	took := time.Now().Sub(start)
	log.Printf("Processed %d lines in %v (%.0f eps)",
		count, took, float64(count)/took.Seconds())
}
