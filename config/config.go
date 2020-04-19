//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package config

type Config struct {
	DevicePath string
	OutputPath string
	Opt        Optimizations
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
