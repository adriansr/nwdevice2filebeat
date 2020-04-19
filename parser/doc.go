//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

//go:generate ragel -Z -G1 call.go.rl -o call_generated.go
//go:generate goimports -l -w call_generated.go
//go:generate ragel -Z -G1 pattern.go.rl -o pattern_generated.go
//go:generate goimports -l -w pattern_generated.go
//go:generate ragel -Z -G1 alt.go.rl -o alt_generated.go
//go:generate goimports -l -w alt_generated.go
//
// Run go vet and remove any unreachable code in the generated go files.
// The go generator outputs duplicated goto statements sometimes.
//
// An SVG rendering of the state machine can be viewed by opening cef.svg in
// Chrome / Firefox.
//go:generate ragel -V -p call.go.rl -o call.dot
//go:generate dot -T svg call.dot -o call.svg
//go:generate ragel -V -p pattern.go.rl -o pattern.dot
//go:generate dot -T svg pattern.dot -o pattern.svg
//go:generate ragel -V -p alt.go.rl -o alt.dot
//go:generate dot -T svg alt.dot -o alt.svg
