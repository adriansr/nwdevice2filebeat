//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/generator/javascript"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/parser"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new Filebeat fileset from a NetWitness device",
	Run:   generateRun,
}

func init() {
	generateCmd.PersistentFlags().String("device", "", "Input device path")
	generateCmd.PersistentFlags().String("output", "", "Output")
	generateCmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
	generateCmd.MarkPersistentFlagRequired("device")
}

func generateRun(cmd *cobra.Command, args []string) {
	cfg, err := readConf(cmd)
	if err != nil {
		log.Panic(err)
	}
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
	writer := ioutil.Discard
	if outfile := cfg.OutputPath; outfile != "" {
		outf, err := os.Create(outfile)
		if err != nil {
			LogError("Failed to create output file", "path", outfile, "reason", err)
			return
		}
		defer outf.Close()
		writer = outf
	}
	numBytes, err := javascript.Generate(p, writer)
	if err != nil {
		LogError("Failed to generate javascript pipeline", "reason", err)
		return
	}
	var size int64
	if st, err := os.Stat(dev.XMLPath); err == nil {
		size = st.Size()
	}
	log.Printf("INFO %d bytes for pipeline %s (%s) from %d original (%.2f%%)\n",
		numBytes, dev.Description.DisplayName, dev.Description.Name,
		size, 100.0*float64(numBytes)/float64(size))
}

func readConf(cmd *cobra.Command) (cfg config.Config, err error) {
	if cfg.DevicePath, err = cmd.PersistentFlags().GetString("device"); err != nil {
		return cfg, err
	}
	if cfg.OutputPath, err = cmd.PersistentFlags().GetString("output"); err != nil {
		return cfg, err
	}
	if opts, err := cmd.PersistentFlags().GetStringSlice("optimize"); err == nil {
		log.Printf("opts = %v\n", opts)
		for _, o := range opts {
			switch o {
			case "globals":
				cfg.Opt.GlobalEntities = true
			case "deduplicate":
				cfg.Opt.DetectDuplicates = true
			}
		}
	}
	log.Printf("no opts\n")
	return cfg, nil
}
