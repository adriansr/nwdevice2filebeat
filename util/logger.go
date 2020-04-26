//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import (
	"log"
)

type Logger interface {
	Debugf(format string, args ...interface{})
}

type DontLog struct{}

func (DontLog) Debugf(string, ...interface{}) {
}

type StdLog struct{}

func (StdLog) Debugf(fmt string, args ...interface{}) {
	log.Printf(fmt, args...)
}

type VerbosityLogger struct {
	Logger   Logger
	MaxLevel VerbosityLevel
}

type VerbosityLevel uint8

const (
	LogError VerbosityLevel = iota
	LogWarn
	LogInfo
	LogDebug
	LogTrace
)

func (vl VerbosityLogger) Log(level VerbosityLevel, fmt string, args ...interface{}) {
	if level <= vl.MaxLevel {
		log.Printf(fmt, args...)
	}
}
