//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"log"
	"os"
	"time"

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

var genPackageCmd = &cobra.Command{
	Use:   "package",
	Short: "Generate an Ingest Manager package from a NetWitness device",
	Run: func(cmd *cobra.Command, args []string) {
		generate(cmd, "package")
	},
}

var genPipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Generate a pipeline from a NetWitness device",
	Run: func(cmd *cobra.Command, args []string) {
		generate(cmd, "pipeline")
	},
}

var genLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Generate sample logs from a device",
	Run: func(cmd *cobra.Command, args []string) {
		generate(cmd, "logs")
	},
}

func init() {
	// Common flags for all sub-options.
	for _, cmd := range []*cobra.Command{genModuleCmd, genPipelineCmd, genPackageCmd, genLogsCmd} {
		cmd.PersistentFlags().String("device", "", "Input device path")
		cmd.PersistentFlags().StringP("format", "f", defaultPipelineFormat, "Pipeline format (js or yml)")
		cmd.PersistentFlags().StringSliceP("optimize", "O", nil, "Optimizations")
		cmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
		cmd.MarkPersistentFlagRequired("device")
		generateCmd.AddCommand(cmd)
	}

	genModuleCmd.PersistentFlags().String("output", "", "Output directory where the module is written to")
	genModuleCmd.PersistentFlags().String("module", "", "Module name")
	genModuleCmd.PersistentFlags().String("fileset", "", "Fileset name")
	genModuleCmd.PersistentFlags().String("vendor", "", "Vendor name")
	genModuleCmd.PersistentFlags().String("product", "", "Product name")
	genModuleCmd.PersistentFlags().String("type", "", "Type of logs (observer.type)")
	genModuleCmd.PersistentFlags().Uint16("port", 9010, "Default port number")
	genModuleCmd.MarkPersistentFlagDirname("output")
	genModuleCmd.MarkPersistentFlagRequired("output")

	genPackageCmd.PersistentFlags().String("output", "", "Output directory where the package is written to")
	genPackageCmd.PersistentFlags().String("module", "", "Package name")
	genPackageCmd.PersistentFlags().String("fileset", "", "Dataset name")
	genPackageCmd.PersistentFlags().String("vendor", "", "Vendor name")
	genPackageCmd.PersistentFlags().String("product", "", "Product name")
	genPackageCmd.PersistentFlags().String("type", "", "Type of logs (observer.type)")
	genPackageCmd.PersistentFlags().String("version", "0.0.1", "Package version")
	genPackageCmd.PersistentFlags().Uint16("port", 9010, "Default port number")
	genPackageCmd.MarkPersistentFlagDirname("output")
	genPackageCmd.MarkPersistentFlagRequired("output")

	genPipelineCmd.PersistentFlags().String("output", "", "Output directory where pipeline is written to")
	genPipelineCmd.MarkPersistentFlagFilename("output")
	genPipelineCmd.MarkPersistentFlagRequired("output")

	genLogsCmd.PersistentFlags().UintP("lines", "n", 100, "Number of lines to output")
	genLogsCmd.PersistentFlags().Uint64("seed", 0, "Random seed")
	genLogsCmd.PersistentFlags().String("output", "", "Output file to write logs to")
	genLogsCmd.MarkPersistentFlagFilename("output")
	genLogsCmd.MarkPersistentFlagRequired("output")
	// `generate logs`: Hardcode --format logs
	genLogsCmd.PersistentFlags().MarkHidden("format")
	genLogsCmd.PersistentFlags().Set("format", "logs")
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

	if targetLayout == "pipeline" || targetLayout == "logs" {
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
	if cfg.Module.Product == "" {
		cfg.Module.Product = p.Description.DisplayName
	}
	if cfg.Module.Vendor == "" {
		cfg.Module.Vendor = cfg.Module.Name
	}
	if cfg.Module.Type == "" {
		cfg.Module.Type = p.Description.Group
	}
	outLayout, err := layout.New(targetLayout, layout.Vars{
		LogParser:     p,
		DisplayName:   p.Description.DisplayName,
		Module:        cfg.Module.Name,
		Fileset:       cfg.Module.Fileset,
		Product:       cfg.Module.Product,
		Vendor:        cfg.Module.Vendor,
		Group:         cfg.Module.Type,
		Version:       cfg.Module.Version,
		Port:          cfg.Module.Port,
		GeneratedTime: time.Now().UTC(),
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
