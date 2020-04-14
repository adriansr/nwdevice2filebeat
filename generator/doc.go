//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

//go:generate ragel -Z -G1 call.rl -o call.go
//go:generate goimports -l -w call.go
//go:generate ragel -Z -G1 pattern.go.rl -o pattern.go
//go:generate goimports -l -w pattern.go
//
// Run go vet and remove any unreachable code in the generated go files.
// The go generator outputs duplicated goto statements sometimes.
//
// An SVG rendering of the state machine can be viewed by opening cef.svg in
// Chrome / Firefox.
//go:generate ragel -V -p call.rl -o call.dot
//go:generate dot -T svg call.dot -o call.svg
//go:generate ragel -V -p pattern.go.rl -o pattern.dot
//go:generate dot -T svg pattern.dot -o pattern.svg

