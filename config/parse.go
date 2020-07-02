//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package config

import (
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/adriansr/nwdevice2filebeat/util"
)

func NewFromCommand(cmd *cobra.Command) (cfg Config, err error) {
	// Mandatory flags (all verbs)
	if cfg.DevicePath, err = cmd.PersistentFlags().GetString("device"); err != nil {
		return cfg, err
	}
	if cfg.OutputPath, err = cmd.PersistentFlags().GetString("output"); err != nil {
		return cfg, err
	}

	// Optional flags
	cfg.PipelineFormat, _ = cmd.PersistentFlags().GetString("format")
	cfg.Module.Name, _ = cmd.PersistentFlags().GetString("module")
	cfg.Module.Fileset, _ = cmd.PersistentFlags().GetString("fileset")
	cfg.Module.Version, _ = cmd.PersistentFlags().GetString("version")
	cfg.Module.Port, _ = cmd.PersistentFlags().GetUint16("port")
	cfg.Module.Vendor, _ = cmd.PersistentFlags().GetString("vendor")
	cfg.Module.Product, _ = cmd.PersistentFlags().GetString("product")
	cfg.Module.Type, _ = cmd.PersistentFlags().GetString("type")

	if opts, err := cmd.PersistentFlags().GetStringSlice("optimize"); err == nil {
		if cfg.Opt, err = parseOpts(opts); err != nil {
			return cfg, err
		}
	}
	if fixes, err := cmd.PersistentFlags().GetStringSlice("fix"); err == nil {
		if cfg.Fixes, err = parseFix(fixes); err != nil {
			return cfg, err
		}
	}
	if tzName, err := cmd.PersistentFlags().GetString("tz"); err == nil {
		if cfg.Runtime.Timezone, err = loadLocation(tzName); err != nil {
			return cfg, errors.Wrapf(err, "unable to parse timezone: '%s'", tzName)
		}
	}
	if verbosity, err := cmd.PersistentFlags().GetCount("verbose"); err == nil {
		cfg.Verbosity = util.VerbosityLevel(verbosity)
	}

	cfg.Seed, _ = cmd.PersistentFlags().GetUint64("seed")
	cfg.NumLines, _ = cmd.PersistentFlags().GetUint("lines")
	return cfg, nil
}

func parseOpts(flags []string) (opt Optimizations, err error) {
	for _, flag := range flags {
		switch flag {
		case "globals":
			opt.GlobalEntities = true
		case "deduplicate":
			opt.DetectDuplicates = true
		case "stripid1":
			opt.StripMessageID1 = true
		default:
			return opt, errors.Errorf("unknown optimization flag: %s", flag)
		}
	}
	return opt, nil
}

func parseFix(flags []string) (fix Fixes, err error) {
	for _, flag := range flags {
		switch flag {
		case "space", "whitespace", "w", "s":
			fix.StripLeadingSpace = true
		default:
			return fix, errors.Errorf("unknown fix flag: %s", flag)
		}
	}
	return fix, nil
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
