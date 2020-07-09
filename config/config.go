//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package config

import (
	"net"
	"time"

	"github.com/adriansr/nwdevice2filebeat/util"
)

// Config contains the configuration for the conversion.
type Config struct {
	// DevicePath is the path to the NW device (directory or XML file).
	DevicePath string
	// OutputPath is TODO.
	OutputPath string

	// PipelineFormat is the kind of pipeline to generate.
	// One of: javascript, yaml.
	PipelineFormat string

	// Module generation settings.
	Module struct {
		Name    string // Name of the module
		Fileset string // Fileset for this device
		Product string // Product name (observer.product)
		Type    string // Type of observer (observer.type)
		Vendor  string // Vendor name (observer.vendor)
		Version string // Module version
		Port    uint16 // Default port number
	}

	// Verbosity is the logging verbosity level for this invocation of the tool.
	Verbosity util.VerbosityLevel

	// PipelineSettings are set automatically depending on the selected pipeline
	// format.
	PipelineSettings PipelineSettings

	// Fixes contains flags to workaround common problems in parsers.
	Fixes Fixes

	// Opt contains optimizations to apply on the generated parsers.
	Opt Optimizations

	// Runtime contains configuration for the runtime parser.
	Runtime Runtime

	// Seed for RNG.
	Seed uint64

	// NumLines of logs to generate.
	NumLines uint
}

type Optimizations struct {
	// Generate composite operations (chains, selects, matches) as top-level
	// global variables. This makes the generated code more compact with impacts
	// readability.
	GlobalEntities bool

	// This detects duplicate operations and extracts them into a global
	// variable replacing all instances with a reference to the variable.
	// Makes the generated JS more compact and saves memory, but impacts
	// readability.
	DetectDuplicates bool

	// This removes the id1 from messages. This ID uniquely identifies every
	// MESSAGE parser and its presence, while it helps to understand which
	// particular message matched, at the same time makes every message parser
	// unique and increases the output size.
	StripMessageID1 bool
}

type Fixes struct {
	// TrimEdgeSpace strips space at the start and end of MESSAGES, as it seems
	// to be a common error to add this extra space.
	TrimEdgeSpace bool
}

type Runtime struct {
	// For datetime handling (EVNTTIME function).
	Timezone *time.Location

	// For network direction calculation (DIRCHK function).
	LocalNetworks []net.IPNet
}

// PipelineSettings contains the configuration that a given pipeline format
// generator needs.
type PipelineSettings struct {
	// Split patterns into multiple dissect patterns (for alternatives).
	Dissect bool
	// Strip payload information (for dissect).
	StripPayload bool
}
