//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/util"
)

const defaultOutputFormat = "javascript"

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new Filebeat module from a NetWitness device",
	Run:   generateRun,
}

func init() {
	generateCmd.PersistentFlags().String("device", "", "Input device path")
	generateCmd.PersistentFlags().String("output", "", "Output")
	generateCmd.PersistentFlags().StringP("format", "f", defaultOutputFormat, "Output")
	generateCmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
	generateCmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
	generateCmd.MarkPersistentFlagRequired("device")
	rootCmd.AddCommand(generateCmd)
}

func generateRun(cmd *cobra.Command, args []string) {
	cfg, err := config.NewFromCommand(cmd)
	if err != nil {
		log.Panic(err)
	}

	out, err := output.Registry.Get(cfg.PipelineFormat)
	if err != nil {
		LogError("Unable to initialize output", "reason", err)
		return
	}

	cfg.PipelineSettings = out.Settings()

	warnings := util.NewWarnings(20)
	dev, err := model.NewDevice(cfg.DevicePath, &warnings)
	if err != nil {
		LogError("Failed to load device", "path", cfg.DevicePath, "reason", err)
		return
	}
	if !warnings.Print("loading device XML") {
		log.Printf("Loaded XML %s", dev.String())
	}

	warnings.Clear()
	p, err := parser.New(dev, cfg, &warnings)
	if err != nil {
		LogError("Failed to parse device", "path", cfg.DevicePath, "reason", err)
		return
	}

	warnings.Print("parsing device")
	warnings.Clear()

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
	countWriter := util.NewCountingWriter(writer)
	if err = out.Generate(p, countWriter); err != nil {
		LogError("Failed writting output", "format", cfg.PipelineFormat, "reason", err)
		return
	}
	var size int64
	if st, err := os.Stat(dev.XMLPath); err == nil {
		size = st.Size()
	}
	log.Printf("INFO %d bytes for pipeline %s (%s) from %d original (%.2f%%)\n",
		countWriter.Count(), dev.Description.DisplayName, dev.Description.Name,
		size, 100.0*float64(countWriter.Count())/float64(size))
}
