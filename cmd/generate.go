//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/generator/javascript"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/util"
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
	generateCmd.PersistentFlags().StringSliceP("fix", "F", nil, "Fixes")
	generateCmd.MarkPersistentFlagRequired("device")
}

func generateRun(cmd *cobra.Command, args []string) {
	cfg, err := readConf(cmd)
	if err != nil {
		log.Panic(err)
	}
	// TODO: Depend on output
	cfg.Dissect = true
	cfg.StripPayload = true

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

var timezoneFormats = []string{"-07", "-0700", "-07:00"}

// Copied from beats/libbeat/processor/timestamp.go
func loadLocation(timezone string) (*time.Location, error) {
	for _, format := range timezoneFormats {
		t, err := time.Parse(format, timezone)
		if err == nil {
			name, offset := t.Zone()
			return time.FixedZone(name, offset), nil
		}
	}

	// Rest of location formats
	return time.LoadLocation(timezone)
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
	if opts, err := cmd.PersistentFlags().GetStringSlice("fix"); err == nil {
		log.Printf("fixes = %v\n", opts)
		for _, o := range opts {
			switch o {
			case "space", "whitespace", "w", "s":
				cfg.Fixes.StripLeadingSpace = true
			}
		}
	}
	if tzName, err := cmd.PersistentFlags().GetString("tz"); err == nil {
		if cfg.Timezone, err = loadLocation(tzName); err != nil {
			return cfg, errors.Wrapf(err, "unable to parse timezone: '%s'", tzName)
		}
	}
	if verbosity, err := cmd.PersistentFlags().GetCount("verbose"); err == nil {
		cfg.Verbosity = util.VerbosityLevel(verbosity)
	}
	return cfg, nil
}
