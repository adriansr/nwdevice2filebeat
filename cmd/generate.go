//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/layout"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/util"
)

const defaultPipelineFormat = "javascript"

var generateCmd = &cobra.Command{
	Use:     "generate",
	Short:   "Generate an output from a NetWitness device",
	PreRunE: usage,
}

var genModuleCmd = &cobra.Command{
	Use:   "module",
	Short: "Generate a Filebeat module from a NetWitness device",
	Run: func(cmd *cobra.Command, args []string) {
		generate(cmd, "module")
	},
}

var genPipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Generate a pipeline from a NetWitness device",
	Run: func(cmd *cobra.Command, args []string) {
		generate(cmd, "pipeline")
	},
}

func init() {
	for _, cmd := range []*cobra.Command{genModuleCmd, genPipelineCmd} {
		cmd.PersistentFlags().String("device", "", "Input device path")
		cmd.PersistentFlags().StringP("format", "f", defaultPipelineFormat, "Pipeline format (js or yml)")
		cmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
		cmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
		cmd.MarkPersistentFlagRequired("device")
		generateCmd.AddCommand(cmd)
	}

	genModuleCmd.PersistentFlags().String("output", "", "Output directory where module is written to")
	genModuleCmd.PersistentFlags().String("module", "", "Module name")
	genModuleCmd.PersistentFlags().String("fileset", "", "Fileset name")
	genModuleCmd.PersistentFlags().Uint16("port", 9010, "Default port number")
	genModuleCmd.MarkPersistentFlagDirname("output")
	genModuleCmd.MarkPersistentFlagRequired("output")
	genPipelineCmd.PersistentFlags().String("output", "", "Output directory where pipeline is written to")
	genPipelineCmd.MarkPersistentFlagFilename("output")
	genPipelineCmd.MarkPersistentFlagRequired("output")
	rootCmd.AddCommand(generateCmd)
}

func generate(cmd *cobra.Command, targetLayout string) error {
	cfg, err := config.NewFromCommand(cmd)
	if err != nil {
		LogError("Unable to initialize config", "reason", err)
		return err
	}
	out, err := output.Registry.Get(cfg.PipelineFormat)
	if err != nil {
		LogError("Unable to initialize output", "reason", err)
		return err
	}

	cfg.PipelineSettings = out.Settings()

	warnings := util.NewWarnings(20)
	dev, err := model.NewDevice(cfg.DevicePath, &warnings)
	if err != nil {
		LogError("Failed to load device", "path", cfg.DevicePath, "reason", err)
		return err
	}
	if !warnings.Print("loading device XML") {
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

	if err = out.Generate(p); err != nil {
		LogError("Failed writing output", "format", cfg.PipelineFormat, "reason", err)
		return err
	}
	/*var size int64
	if st, err := os.Stat(dev.XMLPath); err == nil {
		size = st.Size()
	}
	log.Printf("INFO %d bytes for pipeline %s (%s) from %d original (%.2f%%)\n",
		countWriter.Count(), dev.Description.DisplayName, dev.Description.Name,
		size, 100.0*float64(countWriter.Count())/float64(size))*/

	if targetLayout == "pipeline" {
		srcName := out.OutputFile()
		destF, err := os.Create(cfg.OutputPath)
		if err != nil {
			LogError("Failed creating output file", "path", cfg.OutputPath, "reason", err)
			return err
		}
		defer destF.Close()
		action := layout.Move{Path: srcName}
		if err = action.WriteFile(destF); err != nil {
			LogError("Failed creating output file", "path", cfg.OutputPath, "reason", err)
			return err
		}
		return nil
	}

	if cfg.Module.Name == "" {
		cfg.Module.Name = p.Description.Name
	}
	if cfg.Module.Fileset == "" {
		cfg.Module.Fileset = "log"
	}
	if cfg.Module.Port == 0 {
		cfg.Module.Port = 9010
	}
	outLayout, err := layout.New(targetLayout, layout.Vars{
		Device:      p.Description.Name,
		DisplayName: p.Description.DisplayName,
		Module:      cfg.Module.Name,
		Fileset:     cfg.Module.Fileset,
		Port:        cfg.Module.Port,
	})
	if err != nil {
		LogError("Failed loading output layout", "format", targetLayout, "reason", err)
		return err
	}
	if err = out.Populate(outLayout); err != nil {
		LogError("Failed populating output layout from pipeline", "format", targetLayout, "reason", err)
		return err
	}
	if err := outLayout.Build(cfg.OutputPath); err != nil {
		LogError("Failed generating output layout", "reason", err)
		return err
	}
	return nil
}
