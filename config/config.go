//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package config

import "time"

type Config struct {
	DevicePath string
	OutputPath string
	Opt        Optimizations
	Fixes      Fixes
	// For datetime handling
	Timezone *time.Location

	// These are set depending on what the output supports
	Dissect      bool
	StripPayload bool
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
}

type Fixes struct {
	// StripLeadingSpace strips space at the start of MESSAGES, as it seems
	// to be a common error to add an extra space first.
	StripLeadingSpace bool
}
